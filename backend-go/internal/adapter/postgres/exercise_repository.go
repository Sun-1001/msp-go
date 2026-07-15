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

// ExerciseRepository persists adaptive exercise flow data in PostgreSQL.
type ExerciseRepository struct {
	Repository
}

// NewExerciseRepository creates a PostgreSQL-backed exercise repository.
func NewExerciseRepository(db Querier) (ExerciseRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return ExerciseRepository{}, err
	}
	return ExerciseRepository{Repository: base}, nil
}

// WithTx runs fn in one database transaction when the repository is pool-backed.
func (r ExerciseRepository) WithTx(ctx context.Context, fn func(context.Context, exerciseapp.Repository) error) error {
	if fn == nil {
		return errors.New("exercise transaction function is nil")
	}
	return withRepositoryTx(ctx, "exercise", r.Repository, func(base Repository) ExerciseRepository {
		return ExerciseRepository{Repository: base}
	}, func(txRepo ExerciseRepository) error {
		return fn(ctx, txRepo)
	})
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

// UpdateSessionAfterSubmit appends attempted content and clears it when it is the current class exercise.
func (r ExerciseRepository) UpdateSessionAfterSubmit(ctx context.Context, sessionID string, exerciseID string, attempted []string) error {
	raw, err := json.Marshal(attempted)
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		UPDATE public.learning_sessions
		SET
			contents_attempted = $3::json,
			current_content_id = CASE WHEN current_content_id = $2 THEN NULL ELSE current_content_id END
		WHERE id = $1`,
		sessionID,
		exerciseID,
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

// GetExerciseForUpdate holds a shared row lock until the surrounding submit transaction ends.
func (r ExerciseRepository) GetExerciseForUpdate(ctx context.Context, exerciseID string) (exerciseapp.Exercise, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT `+exerciseSelectColumns+`
		FROM public.contents
		WHERE id = $1 AND type = 'PROBLEM' AND deleted_at IS NULL
		FOR SHARE`,
		exerciseID,
	)
	return scanOptionalExercise(row)
}

// GetKnowledgeConcept returns trusted knowledge context for AI exercise generation.
func (r ExerciseRepository) GetKnowledgeConcept(ctx context.Context, conceptID string) (exerciseapp.KnowledgeConcept, bool, error) {
	var concept exerciseapp.KnowledgeConcept
	var chapter pgtype.Text
	err := r.DB().QueryRow(ctx, `
		SELECT id, name, description, chapter
		FROM public.knowledge_nodes
		WHERE id = $1`,
		conceptID,
	).Scan(&concept.ID, &concept.Name, &concept.Description, &chapter)
	if err != nil {
		if err == pgx.ErrNoRows {
			return exerciseapp.KnowledgeConcept{}, false, nil
		}
		return exerciseapp.KnowledgeConcept{}, false, err
	}
	if chapter.Valid {
		concept.Chapter = chapter.String
	}
	return concept, true, nil
}

// CreateGeneratedExercise persists one published student-owned AI exercise.
func (r ExerciseRepository) CreateGeneratedExercise(ctx context.Context, studentID string, generated exerciseapp.GeneratedQuestion, now time.Time) (exerciseapp.Exercise, error) {
	exerciseID, err := newUUID()
	if err != nil {
		return exerciseapp.Exercise{}, err
	}
	conceptIDsRaw, err := json.Marshal(generated.ConceptIDs)
	if err != nil {
		return exerciseapp.Exercise{}, fmt.Errorf("encode generated exercise concept ids: %w", err)
	}
	metaRaw, err := json.Marshal(generatedQuestionMeta(generated))
	if err != nil {
		return exerciseapp.Exercise{}, fmt.Errorf("encode generated exercise meta: %w", err)
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.contents (
			id,
			type,
			owner_teacher_id,
			generated_by_student_id,
			status,
			title,
			body,
			difficulty,
			concept_ids,
			tags,
			meta,
			created_at,
			updated_at,
			published_at,
			deleted_at
		)
		VALUES ($1, 'PROBLEM'::public.contenttype, NULL, $2, 'PUBLISHED'::public.contentstatus, $3, $4, $5, $6::json, '[]'::json, $7::json, $8, $8, $8, NULL)
		RETURNING `+exerciseSelectColumns,
		exerciseID,
		studentID,
		generated.Title,
		generated.Body,
		generated.Difficulty,
		string(conceptIDsRaw),
		string(metaRaw),
		now,
	)
	exercise, ok, err := scanOptionalExercise(row)
	if err != nil {
		return exerciseapp.Exercise{}, err
	}
	if !ok {
		return exerciseapp.Exercise{}, pgx.ErrNoRows
	}
	return exercise, nil
}

// ListRecentContentIDs returns recent class-exercise IDs without self-practice attempts.
func (r ExerciseRepository) ListRecentContentIDs(ctx context.Context, userID string, limit int) ([]string, error) {
	rows, err := r.DB().Query(ctx, recentClassContentIDsSQL,
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

const recentClassContentIDsSQL = `
	SELECT ca.content_id
	FROM public.content_attempts ca
	JOIN public.contents c ON c.id = ca.content_id
	WHERE ca.student_id = $1 AND c.generated_by_student_id IS NULL
	ORDER BY ca.started_at DESC
	LIMIT $2`

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
	row := r.DB().QueryRow(ctx, `
		SELECT `+exerciseProfileColumns+`
		FROM public.student_profiles
		WHERE student_id = $1`,
		userID,
	)
	return scanOptionalExerciseProfile(row)
}

// CreateProfile inserts default tracking state or returns a concurrently created profile.
func (r ExerciseRepository) CreateProfile(ctx context.Context, userID string, now time.Time) (exerciseapp.StudentProfile, error) {
	profileID, err := newUUID()
	if err != nil {
		return exerciseapp.StudentProfile{}, err
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
		RETURNING `+exerciseProfileColumns,
		profileID,
		userID,
		now,
	)
	profile, ok, err := scanOptionalExerciseProfile(row)
	if err != nil {
		return exerciseapp.StudentProfile{}, err
	}
	if !ok {
		return exerciseapp.StudentProfile{}, pgx.ErrNoRows
	}
	return profile, nil
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
	generated_by_student_id,
	status::text,
	title,
	body,
	difficulty,
	concept_ids,
	meta`

const exerciseProfileColumns = `
	mastery_vector,
	error_tendency,
	preferred_difficulty,
	learning_pace,
	total_exercises,
	correct_count`

func scanOptionalExerciseProfile(row pgx.Row) (exerciseapp.StudentProfile, bool, error) {
	var profile exerciseapp.StudentProfile
	var masteryRaw []byte
	var errorRaw []byte
	err := row.Scan(
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
	var ownerTeacherID pgtype.Text
	var generatedByStudentID pgtype.Text
	var conceptIDsRaw []byte
	var metaRaw []byte
	if err := scanner.Scan(
		&exercise.ID,
		&ownerTeacherID,
		&generatedByStudentID,
		&exercise.Status,
		&exercise.Title,
		&exercise.Body,
		&exercise.Difficulty,
		&conceptIDsRaw,
		&metaRaw,
	); err != nil {
		return exerciseapp.Exercise{}, err
	}
	if ownerTeacherID.Valid {
		exercise.OwnerTeacherID = ownerTeacherID.String
	}
	if generatedByStudentID.Valid {
		exercise.GeneratedByStudentID = generatedByStudentID.String
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

func generatedQuestionMeta(generated exerciseapp.GeneratedQuestion) map[string]any {
	return map[string]any{
		"answer":                 generated.Answer,
		"answer_type":            generated.AnswerType,
		"type":                   generated.Type,
		"options":                generated.Options,
		"hints":                  generated.Hints,
		"solution_steps":         generated.SolutionSteps,
		"estimated_time_seconds": generated.EstimatedTimeSeconds,
		"knowledge_point_names":  generated.KnowledgePointNames,
	}
}
