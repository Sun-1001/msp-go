package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	portraitapp "mathstudy/backend-go/internal/application/portrait"
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
	portrait_version`

// PortraitRepository persists student portrait profiles in PostgreSQL.
type PortraitRepository struct {
	Repository
}

// NewPortraitRepository creates a PostgreSQL-backed portrait repository.
func NewPortraitRepository(db Querier) (PortraitRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return PortraitRepository{}, err
	}
	return PortraitRepository{Repository: base}, nil
}

// GetProfile returns a student profile when it exists.
func (r PortraitRepository) GetProfile(ctx context.Context, userID string) (portraitapp.Profile, bool, error) {
	row := r.DB().QueryRow(ctx, `SELECT `+portraitColumns+` FROM public.student_profiles WHERE student_id = $1`, userID)
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
func (r PortraitRepository) SavePortrait(ctx context.Context, userID string, content string, generatedAt time.Time) (portraitapp.Profile, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.student_profiles
		SET
			portrait_content = $2,
			portrait_generated_at = $3,
			portrait_version = portrait_version + 1,
			updated_at = $3
		WHERE student_id = $1
		RETURNING `+portraitColumns,
		userID,
		content,
		generatedAt,
	)
	return scanOptionalPortrait(row)
}

// ClearPortrait removes generated portrait content and resets its version.
func (r PortraitRepository) ClearPortrait(ctx context.Context, userID string, updatedAt time.Time) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.student_profiles
		SET
			portrait_content = NULL,
			portrait_generated_at = NULL,
			portrait_version = 0,
			updated_at = $2
		WHERE student_id = $1`,
		userID,
		updatedAt,
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
		&profile.PortraitVersion,
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
	if content.Valid {
		value := content.String
		profile.PortraitContent = &value
	}
	if generatedAt.Valid {
		value := generatedAt.Time
		profile.PortraitGeneratedAt = &value
	}
	return profile, true, nil
}
