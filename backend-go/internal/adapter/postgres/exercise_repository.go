package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	exerciseapp "mathstudy/backend-go/internal/application/exercise"
)

type pgxTxBeginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// ExerciseRepository persists adaptive exercise flow data in PostgreSQL.
type ExerciseRepository struct {
	Repository
	beginner pgxTxBeginner
}

// NewExerciseRepository creates a PostgreSQL-backed exercise repository.
func NewExerciseRepository(db Querier) (ExerciseRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return ExerciseRepository{}, err
	}
	repo := ExerciseRepository{Repository: base}
	if beginner, ok := db.(pgxTxBeginner); ok {
		repo.beginner = beginner
	}
	return repo, nil
}

// WithTx runs fn in one database transaction when the repository is pool-backed.
func (r ExerciseRepository) WithTx(ctx context.Context, fn func(context.Context, exerciseapp.Repository) error) error {
	if fn == nil {
		return errors.New("exercise transaction function is nil")
	}
	if r.beginner == nil {
		return fn(ctx, r)
	}
	tx, err := r.beginner.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin exercise transaction: %w", err)
	}
	base, err := NewRepository(tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	txRepo := ExerciseRepository{Repository: base}
	if err := fn(ctx, txRepo); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback exercise transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(fmt.Errorf("commit exercise transaction: %w", err), fmt.Errorf("rollback exercise transaction: %w", rollbackErr))
		}
		return fmt.Errorf("commit exercise transaction: %w", err)
	}
	return nil
}

// GetTeacherIDForStudent returns the teacher for the student's current class.
func (r ExerciseRepository) GetTeacherIDForStudent(ctx context.Context, userID string) (string, bool, error) {
	var teacherID string
	err := r.DB().QueryRow(ctx, `
		SELECT c.teacher_id
		FROM public.classes c
		JOIN public.class_enrollments ce ON ce.class_id = c.id
		WHERE ce.student_id = $1`,
		userID,
	).Scan(&teacherID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return teacherID, true, nil
}

// GetLatestSession returns the newest learning session for one student.
func (r ExerciseRepository) GetLatestSession(ctx context.Context, userID string) (exerciseapp.LearningSession, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT id, student_id, current_content_id, contents_attempted
		FROM public.learning_sessions
		WHERE student_id = $1
		ORDER BY started_at DESC
		LIMIT 1`,
		userID,
	)
	return scanOptionalExerciseSession(row)
}

// CreateSession inserts a blank active session.
func (r ExerciseRepository) CreateSession(ctx context.Context, userID string, now time.Time) (exerciseapp.LearningSession, error) {
	id, err := newUUID()
	if err != nil {
		return exerciseapp.LearningSession{}, err
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.learning_sessions (
			id,
			student_id,
			is_active,
			current_topic,
			current_content_id,
			contents_attempted,
			concepts_discussed,
			started_at,
			ended_at
		)
		VALUES ($1, $2, true, NULL, NULL, '[]'::json, '[]'::json, $3, NULL)
		RETURNING id, student_id, current_content_id, contents_attempted`,
		id,
		userID,
		now,
	)
	session, ok, err := scanOptionalExerciseSession(row)
	if err != nil {
		return exerciseapp.LearningSession{}, err
	}
	if !ok {
		return exerciseapp.LearningSession{}, pgx.ErrNoRows
	}
	return session, nil
}

// UpdateSessionCurrentContent stores or clears the current pending exercise.
func (r ExerciseRepository) UpdateSessionCurrentContent(ctx context.Context, sessionID string, contentID *string) error {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.learning_sessions
		SET current_content_id = $2
		WHERE id = $1`,
		sessionID,
		contentID,
	)
	return err
}

// UpdateSessionAfterSubmit appends attempted content and clears the current content.
func (r ExerciseRepository) UpdateSessionAfterSubmit(ctx context.Context, sessionID string, attempted []string) error {
	raw, err := json.Marshal(attempted)
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		UPDATE public.learning_sessions
		SET contents_attempted = $2::json, current_content_id = NULL
		WHERE id = $1`,
		sessionID,
		string(raw),
	)
	return err
}

// GetExercise returns a non-deleted problem content row.
func (r ExerciseRepository) GetExercise(ctx context.Context, exerciseID string) (exerciseapp.Exercise, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT `+exerciseSelectColumns+`
		FROM public.contents
		WHERE id = $1 AND type = 'PROBLEM' AND deleted_at IS NULL`,
		exerciseID,
	)
	return scanOptionalExercise(row)
}

// ListRecentContentIDs returns recently started content IDs.
func (r ExerciseRepository) ListRecentContentIDs(ctx context.Context, userID string, limit int) ([]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT content_id
		FROM public.content_attempts
		WHERE student_id = $1
		ORDER BY started_at DESC
		LIMIT $2`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListCandidateExercises returns published teacher-owned problems in a difficulty window.
func (r ExerciseRepository) ListCandidateExercises(ctx context.Context, filter exerciseapp.CandidateFilter) ([]exerciseapp.Exercise, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT `+exerciseSelectColumns+`
		FROM public.contents
		WHERE
			type = 'PROBLEM' AND
			status = 'PUBLISHED' AND
			deleted_at IS NULL AND
			owner_teacher_id = $1 AND
			difficulty >= $2 AND
			difficulty <= $3 AND
			(coalesce(cardinality($4::varchar[]), 0) = 0 OR NOT (id = ANY($4::varchar[])))
		ORDER BY difficulty ASC, id ASC
		LIMIT $5`,
		filter.TeacherID,
		filter.DifficultyMin,
		filter.DifficultyMax,
		filter.ExcludeContent,
		filter.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exercises := []exerciseapp.Exercise{}
	for rows.Next() {
		exercise, err := scanExercise(rows)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}
	return exercises, rows.Err()
}

// GetProfile returns the student's exercise tracking profile.
func (r ExerciseRepository) GetProfile(ctx context.Context, userID string) (exerciseapp.StudentProfile, bool, error) {
	var profile exerciseapp.StudentProfile
	var masteryRaw []byte
	var errorRaw []byte
	err := r.DB().QueryRow(ctx, `
		SELECT
			mastery_vector,
			error_tendency,
			preferred_difficulty,
			learning_pace,
			total_exercises,
			correct_count
		FROM public.student_profiles
		WHERE student_id = $1`,
		userID,
	).Scan(
		&masteryRaw,
		&errorRaw,
		&profile.PreferredDifficulty,
		&profile.LearningPace,
		&profile.TotalExercises,
		&profile.CorrectCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return exerciseapp.StudentProfile{}, false, nil
		}
		return exerciseapp.StudentProfile{}, false, err
	}
	mastery, err := decodeFloatMap(masteryRaw)
	if err != nil {
		return exerciseapp.StudentProfile{}, false, fmt.Errorf("decode mastery vector: %w", err)
	}
	errorTendency, err := decodeFloatMap(errorRaw)
	if err != nil {
		return exerciseapp.StudentProfile{}, false, fmt.Errorf("decode error tendency: %w", err)
	}
	profile.MasteryVector = mastery
	profile.ErrorTendency = errorTendency
	return profile, true, nil
}

// HasSubmittedAttempt reports whether the student has attempted the exercise.
func (r ExerciseRepository) HasSubmittedAttempt(ctx context.Context, userID string, exerciseID string) (bool, error) {
	return r.Exists(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM public.content_attempts
			WHERE student_id = $1 AND content_id = $2 AND submitted_at IS NOT NULL
		)`,
		userID,
		exerciseID,
	)
}

// ListDKTStates returns current DKT state rows for the requested concepts.
func (r ExerciseRepository) ListDKTStates(ctx context.Context, userID string, conceptIDs []string) (map[string]exerciseapp.DKTState, error) {
	states := map[string]exerciseapp.DKTState{}
	if len(conceptIDs) == 0 {
		return states, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			id,
			student_id,
			concept_id,
			mastery_prob,
			confidence,
			attempt_count,
			correct_count,
			incorrect_count,
			sequence_length,
			attention_weight,
			last_outcome,
			last_exercise_id,
			last_attempt_at,
			created_at,
			updated_at
		FROM public.student_concept_dkt_states
		WHERE student_id = $1 AND concept_id = ANY($2::varchar[])`,
		userID,
		conceptIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var state exerciseapp.DKTState
		var lastOutcome pgtype.Bool
		var lastExerciseID pgtype.Text
		var lastAttemptAt pgtype.Timestamp
		if err := rows.Scan(
			&state.ID,
			&state.StudentID,
			&state.ConceptID,
			&state.MasteryProb,
			&state.Confidence,
			&state.AttemptCount,
			&state.CorrectCount,
			&state.IncorrectCount,
			&state.SequenceLength,
			&state.AttentionWeight,
			&lastOutcome,
			&lastExerciseID,
			&lastAttemptAt,
			&state.CreatedAt,
			&state.UpdatedAt,
		); err != nil {
			return nil, err
		}
		state.LastOutcome = boolPtr(lastOutcome)
		state.LastExerciseID = textPtr(lastExerciseID)
		state.LastAttemptAt = timestampPtr(lastAttemptAt)
		states[state.ConceptID] = state
	}
	return states, rows.Err()
}

// ListRecentInteractions returns recent submitted exercise events for sequence-based DKT.
func (r ExerciseRepository) ListRecentInteractions(ctx context.Context, userID string, limit int) ([]exerciseapp.LearningInteraction, error) {
	if limit < 1 {
		return []exerciseapp.LearningInteraction{}, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			ca.content_id,
			c.concept_ids,
			ca.is_correct,
			c.difficulty,
			ca.submitted_at
		FROM public.content_attempts ca
		JOIN public.contents c ON c.id = ca.content_id
		WHERE ca.student_id = $1 AND ca.submitted_at IS NOT NULL
		ORDER BY ca.submitted_at DESC, ca.id DESC
		LIMIT $2`,
		userID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	interactions := []exerciseapp.LearningInteraction{}
	for rows.Next() {
		var interaction exerciseapp.LearningInteraction
		var conceptIDsRaw []byte
		if err := rows.Scan(
			&interaction.ExerciseID,
			&conceptIDsRaw,
			&interaction.IsCorrect,
			&interaction.Difficulty,
			&interaction.SubmittedAt,
		); err != nil {
			return nil, err
		}
		conceptIDs, err := decodeStringSlice(conceptIDsRaw)
		if err != nil {
			return nil, fmt.Errorf("decode interaction concept ids: %w", err)
		}
		interaction.ConceptIDs = conceptIDs
		interactions = append(interactions, interaction)
	}
	return interactions, rows.Err()
}

// InsertAttempt inserts a submitted answer attempt.
func (r ExerciseRepository) InsertAttempt(ctx context.Context, record exerciseapp.AttemptRecord) error {
	stepsRaw, err := json.Marshal(record.StudentSteps)
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.content_attempts (
			id,
			content_id,
			student_id,
			student_answer,
			student_steps,
			is_correct,
			score,
			started_at,
			submitted_at,
			time_spent_seconds
		)
		VALUES ($1, $2, $3, $4, $5::json, $6, $7, $8, $9, $10)`,
		record.ID,
		record.ContentID,
		record.StudentID,
		record.StudentAnswer,
		string(stepsRaw),
		record.IsCorrect,
		record.Score,
		record.StartedAt,
		record.SubmittedAt,
		record.TimeSpentSeconds,
	)
	return err
}

// InsertDiagnosis inserts a lightweight diagnosis report.
func (r ExerciseRepository) InsertDiagnosis(ctx context.Context, record exerciseapp.DiagnosisRecord) error {
	relatedRaw, err := json.Marshal(record.RelatedConcept)
	if err != nil {
		return err
	}
	var errorType any
	if record.ErrorType != nil {
		errorType = *record.ErrorType
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.diagnosis_reports (
			id,
			attempt_id,
			error_step_index,
			bifurcation_point,
			error_type,
			error_subtype,
			severity,
			related_concept_ids,
			related_misconception_ids,
			explanation,
			suggestion,
			recommended_resources,
			created_at
		)
		VALUES ($1, $2, NULL, NULL, $3::public.errortype, $4, $5, $6::json, '[]'::json, $7, $8, '[]'::json, $9)`,
		record.ID,
		record.AttemptID,
		errorType,
		record.ErrorSubtype,
		record.Severity,
		string(relatedRaw),
		record.Explanation,
		record.Suggestion,
		record.CreatedAt,
	)
	return err
}

// UpsertDKTStates writes student concept DKT states.
func (r ExerciseRepository) UpsertDKTStates(ctx context.Context, states []exerciseapp.DKTState) error {
	for _, state := range states {
		_, err := r.DB().Exec(ctx, `
			INSERT INTO public.student_concept_dkt_states (
				id,
				student_id,
				concept_id,
				mastery_prob,
				confidence,
				attempt_count,
				correct_count,
				incorrect_count,
				sequence_length,
				attention_weight,
				last_outcome,
				last_exercise_id,
				last_attempt_at,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT ON CONSTRAINT uq_student_concept_dkt_state DO UPDATE SET
				mastery_prob = EXCLUDED.mastery_prob,
				confidence = EXCLUDED.confidence,
				attempt_count = EXCLUDED.attempt_count,
				correct_count = EXCLUDED.correct_count,
				incorrect_count = EXCLUDED.incorrect_count,
				sequence_length = EXCLUDED.sequence_length,
				attention_weight = EXCLUDED.attention_weight,
				last_outcome = EXCLUDED.last_outcome,
				last_exercise_id = EXCLUDED.last_exercise_id,
				last_attempt_at = EXCLUDED.last_attempt_at,
				updated_at = EXCLUDED.updated_at`,
			state.ID,
			state.StudentID,
			state.ConceptID,
			state.MasteryProb,
			state.Confidence,
			state.AttemptCount,
			state.CorrectCount,
			state.IncorrectCount,
			state.SequenceLength,
			state.AttentionWeight,
			state.LastOutcome,
			state.LastExerciseID,
			state.LastAttemptAt,
			state.CreatedAt,
			state.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateProfileTracking stores updated mastery and counters.
func (r ExerciseRepository) UpdateProfileTracking(ctx context.Context, userID string, update exerciseapp.ProfileTrackingUpdate) error {
	masteryRaw, err := json.Marshal(update.MasteryVector)
	if err != nil {
		return err
	}
	errorRaw, err := json.Marshal(update.ErrorTendency)
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		UPDATE public.student_profiles
		SET
			mastery_vector = $2::json,
			error_tendency = $3::json,
			total_exercises = $4,
			correct_count = $5,
			updated_at = $6
		WHERE student_id = $1`,
		userID,
		string(masteryRaw),
		string(errorRaw),
		update.TotalExercises,
		update.CorrectCount,
		update.UpdatedAt,
	)
	return err
}

const exerciseSelectColumns = `
	id,
	owner_teacher_id,
	status::text,
	title,
	body,
	difficulty,
	concept_ids,
	meta`

func scanOptionalExerciseSession(row pgx.Row) (exerciseapp.LearningSession, bool, error) {
	var session exerciseapp.LearningSession
	var currentContent pgtype.Text
	var attemptedRaw []byte
	err := row.Scan(&session.ID, &session.StudentID, &currentContent, &attemptedRaw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return exerciseapp.LearningSession{}, false, nil
		}
		return exerciseapp.LearningSession{}, false, err
	}
	session.CurrentContentID = textPtr(currentContent)
	attempted, err := decodeStringSlice(attemptedRaw)
	if err != nil {
		return exerciseapp.LearningSession{}, false, fmt.Errorf("decode contents attempted: %w", err)
	}
	session.ContentsAttempted = attempted
	return session, true, nil
}

func scanOptionalExercise(row pgx.Row) (exerciseapp.Exercise, bool, error) {
	exercise, err := scanExercise(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return exerciseapp.Exercise{}, false, nil
		}
		return exerciseapp.Exercise{}, false, err
	}
	return exercise, true, nil
}

func scanExercise(scanner rowScanner) (exerciseapp.Exercise, error) {
	var exercise exerciseapp.Exercise
	var conceptIDsRaw []byte
	var metaRaw []byte
	if err := scanner.Scan(
		&exercise.ID,
		&exercise.OwnerTeacherID,
		&exercise.Status,
		&exercise.Title,
		&exercise.Body,
		&exercise.Difficulty,
		&conceptIDsRaw,
		&metaRaw,
	); err != nil {
		return exerciseapp.Exercise{}, err
	}
	conceptIDs, err := decodeStringSlice(conceptIDsRaw)
	if err != nil {
		return exerciseapp.Exercise{}, fmt.Errorf("decode concept ids: %w", err)
	}
	meta, err := decodeObjectMap(metaRaw)
	if err != nil {
		return exerciseapp.Exercise{}, fmt.Errorf("decode content meta: %w", err)
	}
	exercise.ConceptIDs = conceptIDs
	exercise.Meta = meta
	return exercise, nil
}
