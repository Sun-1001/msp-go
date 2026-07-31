package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	portraitapp "mathstudy/backend/internal/application/portrait"
)

const portraitColumns = `
	student_id,
	mastery_vector,
	error_tendency,
	preferred_difficulty,
	learning_pace,
	total_exercises,
	correct_count,
	total_study_time_minutes,
	recent_concepts,
	portrait_content,
	portrait_generated_at,
	portrait_range,
	portrait_snapshot_at,
	portrait_version,
	portrait_revision`

const portraitReadColumns = `
	sp.student_id,
	sp.mastery_vector,
	sp.error_tendency,
	sp.preferred_difficulty,
	sp.learning_pace,
	sp.total_exercises,
	sp.correct_count,
	coalesce((
		SELECT sum(ca.time_spent_seconds) / 60
		FROM public.content_attempts ca
		WHERE ca.student_id = sp.student_id
	), 0)::int AS total_study_time_minutes,
	coalesce((
		SELECT json_agg(recent.name)
		FROM (
			SELECT kn.name
			FROM public.student_concept_dkt_states state
			JOIN public.knowledge_nodes kn ON kn.id = state.concept_id
			WHERE state.student_id = sp.student_id
			ORDER BY state.last_attempt_at DESC NULLS LAST
			LIMIT 5
		) recent
	), '[]'::json) AS recent_concepts,
	sp.portrait_content,
	sp.portrait_generated_at,
	sp.portrait_range,
	sp.portrait_snapshot_at,
	sp.portrait_version,
	sp.portrait_revision`

// PortraitRepository persists student portrait profiles in PostgreSQL.
type PortraitRepository struct {
	Repository
}

// GetRangeStats derives report inputs from the exact activity window selected in learning statistics.
func (r PortraitRepository) GetRangeStats(ctx context.Context, userID string, start time.Time, end time.Time) (portraitapp.RangeStats, error) {
	var stats portraitapp.RangeStats
	var errorsRaw []byte
	var recentRaw []byte
	err := r.DB().QueryRow(ctx, `
		WITH activity AS (
			SELECT
				count(ca.id)::int AS total_exercises,
				(count(ca.id) FILTER (WHERE ca.is_correct))::int AS correct_count,
				(coalesce(sum(ca.time_spent_seconds), 0) / 60)::int AS study_minutes
			FROM public.content_attempts ca
			WHERE ca.student_id = $1
				AND ca.submitted_at IS NOT NULL
				AND ca.submitted_at >= $2
				AND ca.submitted_at <= $3
		), error_counts AS (
			SELECT dr.error_type::text AS error_type, count(dr.id)::int AS error_count
			FROM public.diagnosis_reports dr
			JOIN public.content_attempts ca ON ca.id = dr.attempt_id
			WHERE ca.student_id = $1
				AND ca.submitted_at IS NOT NULL
				AND ca.submitted_at >= $2
				AND ca.submitted_at <= $3
				AND dr.error_type IS NOT NULL
			GROUP BY dr.error_type
		), recent_concepts AS (
			SELECT state.concept_id AS value, state.last_attempt_at AS latest_at
			FROM public.student_concept_dkt_states state
			WHERE state.student_id = $1
				AND state.last_attempt_at IS NOT NULL
				AND state.last_attempt_at >= $2
				AND state.last_attempt_at <= $3
			ORDER BY state.last_attempt_at DESC, state.concept_id
			LIMIT 5
		)
		SELECT
			activity.total_exercises,
			activity.correct_count,
			activity.study_minutes,
			coalesce((SELECT json_object_agg(error_type, error_count) FROM error_counts), '{}'::json),
			coalesce((
				SELECT json_agg(coalesce(kn.name, recent.value) ORDER BY recent.latest_at DESC)
				FROM recent_concepts recent
				LEFT JOIN public.knowledge_nodes kn ON kn.id = recent.value
			), '[]'::json)
		FROM activity`,
		userID,
		start,
		end,
	).Scan(
		&stats.TotalExercises,
		&stats.CorrectCount,
		&stats.TotalStudyTimeMinutes,
		&errorsRaw,
		&recentRaw,
	)
	if err != nil {
		return portraitapp.RangeStats{}, err
	}
	stats.ErrorTendency, err = decodeFloatMap(errorsRaw)
	if err != nil {
		return portraitapp.RangeStats{}, fmt.Errorf("decode range error tendency: %w", err)
	}
	stats.RecentConcepts, err = decodeStringSlice(recentRaw)
	if err != nil {
		return portraitapp.RangeStats{}, fmt.Errorf("decode range recent concepts: %w", err)
	}
	return stats, nil
}

// ListMasteryStates returns the persisted DKT inputs used for the shared read-side projection.
func (r PortraitRepository) ListMasteryStates(ctx context.Context, userID string) ([]portraitapp.MasteryState, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT concept_id, mastery_prob, last_attempt_at
		FROM public.student_concept_dkt_states
		WHERE student_id = $1
		ORDER BY concept_id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := []portraitapp.MasteryState{}
	for rows.Next() {
		var state portraitapp.MasteryState
		var lastAttemptAt pgtype.Timestamp
		if err := rows.Scan(&state.ConceptID, &state.Mastery, &lastAttemptAt); err != nil {
			return nil, err
		}
		state.LastAttemptAt = timestampPtr(lastAttemptAt)
		states = append(states, state)
	}
	return states, rows.Err()
}

// NewPortraitRepository creates a PostgreSQL-backed portrait repository.
func NewPortraitRepository(db Querier) (PortraitRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return PortraitRepository{}, err
	}
	return PortraitRepository{Repository: base}, nil
}

// WithTx runs fn in one database transaction when the repository is pool-backed.
func (r PortraitRepository) WithTx(ctx context.Context, fn func(context.Context, portraitapp.Repository) error) error {
	if fn == nil {
		return errors.New("portrait transaction function is nil")
	}
	return withRepositoryTx(ctx, "portrait", r.Repository, func(base Repository) PortraitRepository {
		return PortraitRepository{Repository: base}
	}, func(txRepo PortraitRepository) error {
		return fn(ctx, txRepo)
	})
}

// LockStudentTracking serializes portrait writes with mastery updates.
func (r PortraitRepository) LockStudentTracking(ctx context.Context, userID string) error {
	return lockStudentTracking(ctx, r.DB(), userID)
}

// GetProfile returns a student profile when it exists.
func (r PortraitRepository) GetProfile(ctx context.Context, userID string) (portraitapp.Profile, bool, error) {
	row := r.DB().QueryRow(ctx, `SELECT `+portraitReadColumns+` FROM public.student_profiles sp WHERE sp.student_id = $1`, userID)
	return scanOptionalPortrait(row)
}

// CreateProfile inserts an empty profile or returns the concurrent existing profile.
func (r PortraitRepository) CreateProfile(ctx context.Context, userID string, now time.Time) (portraitapp.Profile, error) {
	id, err := newUUID()
	if err != nil {
		return portraitapp.Profile{}, err
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.student_profiles (
			id,
			student_id,
			mastery_vector,
			error_tendency,
			preferred_difficulty,
			learning_pace,
			total_exercises,
			correct_count,
			total_study_time_minutes,
			recent_concepts,
			updated_at,
			portrait_version
		)
		VALUES ($1, $2, '{}'::json, '{}'::json, 0.5, 1.0, 0, 0, 0, '[]'::json, $3, 0)
		ON CONFLICT (student_id) DO UPDATE SET student_id = EXCLUDED.student_id
		RETURNING `+portraitColumns,
		id,
		userID,
		now,
	)
	profile, ok, err := scanOptionalPortrait(row)
	if err != nil {
		return portraitapp.Profile{}, err
	}
	if !ok {
		return portraitapp.Profile{}, pgx.ErrNoRows
	}
	return profile, nil
}

// SavePortrait stores generated portrait content and increments its version.
func (r PortraitRepository) SavePortrait(ctx context.Context, userID string, content string, rangeType string, generatedAt time.Time, snapshotAt time.Time, expectedRevision int64) (portraitapp.Profile, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.student_profiles
		SET
			portrait_content = $2,
			portrait_generated_at = $3,
			portrait_range = $4,
			portrait_snapshot_at = $5,
			portrait_version = portrait_version + 1,
			portrait_revision = portrait_revision + 1,
			updated_at = $3
		WHERE student_id = $1 AND portrait_revision = $6
		RETURNING `+portraitColumns,
		userID,
		content,
		generatedAt,
		rangeType,
		snapshotAt,
		expectedRevision,
	)
	return scanOptionalPortrait(row)
}

// ClearPortrait removes generated portrait content and resets its version.
func (r PortraitRepository) ClearPortrait(ctx context.Context, userID string, updatedAt time.Time, expectedRevision int64) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.student_profiles
		SET
			portrait_content = NULL,
			portrait_generated_at = NULL,
			portrait_range = NULL,
			portrait_snapshot_at = NULL,
			portrait_version = 0,
			portrait_revision = portrait_revision + 1,
			updated_at = $2
		WHERE student_id = $1 AND portrait_revision = $3`,
		userID,
		updatedAt,
		expectedRevision,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func scanOptionalPortrait(row pgx.Row) (portraitapp.Profile, bool, error) {
	var profile portraitapp.Profile
	var masteryRaw []byte
	var errorRaw []byte
	var recentRaw []byte
	var content pgtype.Text
	var generatedAt pgtype.Timestamp
	var portraitRange pgtype.Text
	var snapshotAt pgtype.Timestamp
	err := row.Scan(
		&profile.StudentID,
		&masteryRaw,
		&errorRaw,
		&profile.PreferredDifficulty,
		&profile.LearningPace,
		&profile.TotalExercises,
		&profile.CorrectCount,
		&profile.TotalStudyTimeMinutes,
		&recentRaw,
		&content,
		&generatedAt,
		&portraitRange,
		&snapshotAt,
		&profile.PortraitVersion,
		&profile.PortraitRevision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return portraitapp.Profile{}, false, nil
		}
		return portraitapp.Profile{}, false, err
	}
	mastery, err := decodeFloatMap(masteryRaw)
	if err != nil {
		return portraitapp.Profile{}, false, fmt.Errorf("decode mastery vector: %w", err)
	}
	errorTendency, err := decodeFloatMap(errorRaw)
	if err != nil {
		return portraitapp.Profile{}, false, fmt.Errorf("decode error tendency: %w", err)
	}
	recentConcepts, err := decodeStringSlice(recentRaw)
	if err != nil {
		return portraitapp.Profile{}, false, fmt.Errorf("decode recent concepts: %w", err)
	}
	profile.MasteryVector = mastery
	profile.ErrorTendency = errorTendency
	profile.RecentConcepts = recentConcepts
	profile.PortraitContent = textPtr(content)
	profile.PortraitGeneratedAt = timestampPtr(generatedAt)
	profile.PortraitRange = textPtr(portraitRange)
	profile.PortraitSnapshotAt = timestampPtr(snapshotAt)
	return profile, true, nil
}
