package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	mistakeapp "mathstudy/backend-go/internal/application/mistake"
)

// MistakeRepository persists mistake book read and write models in PostgreSQL.
type MistakeRepository struct {
	Repository
}

// NewMistakeRepository creates a PostgreSQL-backed mistake repository.
func NewMistakeRepository(db Querier) (MistakeRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return MistakeRepository{}, err
	}
	return MistakeRepository{Repository: base}, nil
}

// ListMistakes returns submitted incorrect attempts with their diagnosis and content.
func (r MistakeRepository) ListMistakes(ctx context.Context, userID string, filter mistakeapp.ListFilter) ([]mistakeapp.MistakeRow, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT `+mistakeSelectColumns+`
		FROM public.content_attempts ca
		JOIN public.diagnosis_reports dr ON ca.id = dr.attempt_id
		JOIN public.contents c ON ca.content_id = c.id
		WHERE
			ca.student_id = $1 AND
			ca.is_correct = false AND
			ca.submitted_at IS NOT NULL AND
			($2 = '' OR dr.error_type::text = $2) AND
			($3 = '' OR EXISTS (
				SELECT 1
				FROM json_array_elements_text(c.concept_ids) AS concept(value)
				WHERE concept.value = $3
			)) AND
			c.difficulty >= $4 AND
			c.difficulty <= $5 AND
			($6::timestamp IS NULL OR ca.submitted_at >= $6) AND
			($7::timestamp IS NULL OR ca.submitted_at <= $7)
		ORDER BY ca.submitted_at DESC, ca.id DESC`,
		userID,
		filter.ErrorType,
		filter.ConceptID,
		filter.DifficultyMin,
		filter.DifficultyMax,
		filter.DateFrom,
		filter.DateTo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mistakes := []mistakeapp.MistakeRow{}
	for rows.Next() {
		row, err := scanMistakeRow(rows)
		if err != nil {
			return nil, err
		}
		mistakes = append(mistakes, row)
	}
	return mistakes, rows.Err()
}

// GetMistakeByAttempt returns one attempt with diagnosis and content for detail views.
func (r MistakeRepository) GetMistakeByAttempt(ctx context.Context, userID string, attemptID string) (mistakeapp.MistakeRow, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT `+mistakeSelectColumns+`
		FROM public.content_attempts ca
		JOIN public.diagnosis_reports dr ON ca.id = dr.attempt_id
		JOIN public.contents c ON ca.content_id = c.id
		WHERE ca.id = $1 AND ca.student_id = $2`,
		attemptID,
		userID,
	)
	return scanOptionalMistakeRow(row)
}

// GetAttemptContent returns one attempt and content pair for write use cases.
func (r MistakeRepository) GetAttemptContent(ctx context.Context, userID string, attemptID string) (mistakeapp.AttemptContent, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT
			ca.id,
			ca.content_id,
			ca.student_answer,
			ca.student_steps,
			ca.is_correct,
			ca.score,
			ca.submitted_at,
			ca.time_spent_seconds,
			c.id,
			c.type::text,
			c.title,
			c.body,
			c.difficulty,
			c.concept_ids,
			c.meta
		FROM public.content_attempts ca
		JOIN public.contents c ON ca.content_id = c.id
		WHERE ca.id = $1 AND ca.student_id = $2`,
		attemptID,
		userID,
	)

	var attempt mistakeapp.Attempt
	var content mistakeapp.Content
	if err := scanAttemptAndContent(row, &attempt, &content); err != nil {
		if err == pgx.ErrNoRows {
			return mistakeapp.AttemptContent{}, false, nil
		}
		return mistakeapp.AttemptContent{}, false, err
	}
	return mistakeapp.AttemptContent{Attempt: attempt, Content: content}, true, nil
}

// ListAttemptHistory returns submitted attempts for the same content, excluding the current attempt.
func (r MistakeRepository) ListAttemptHistory(ctx context.Context, userID string, contentID string, excludeAttemptID string) ([]mistakeapp.MistakeHistory, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT id, submitted_at, is_correct, score
		FROM public.content_attempts
		WHERE
			student_id = $1 AND
			content_id = $2 AND
			id <> $3 AND
			submitted_at IS NOT NULL
		ORDER BY submitted_at DESC`,
		userID,
		contentID,
		excludeAttemptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := []mistakeapp.MistakeHistory{}
	for rows.Next() {
		var item mistakeapp.MistakeHistory
		var submittedAt pgtype.Timestamp
		if err := rows.Scan(&item.AttemptID, &submittedAt, &item.IsCorrect, &item.Score); err != nil {
			return nil, err
		}
		if submittedAt.Valid {
			value := submittedAt.Time.Format("2006-01-02T15:04:05.999999")
			item.SubmittedAt = &value
		}
		history = append(history, item)
	}
	return history, rows.Err()
}

// GetProfile returns the student's mastery vector.
func (r MistakeRepository) GetProfile(ctx context.Context, userID string) (mistakeapp.StudentProfile, bool, error) {
	var masteryRaw []byte
	err := r.DB().QueryRow(ctx, `
		SELECT mastery_vector
		FROM public.student_profiles
		WHERE student_id = $1`,
		userID,
	).Scan(&masteryRaw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return mistakeapp.StudentProfile{}, false, nil
		}
		return mistakeapp.StudentProfile{}, false, err
	}
	mastery, err := decodeFloatMap(masteryRaw)
	if err != nil {
		return mistakeapp.StudentProfile{}, false, fmt.Errorf("decode mastery vector: %w", err)
	}
	return mistakeapp.StudentProfile{MasteryVector: mastery}, true, nil
}

// ErrorCountsByContent returns incorrect attempt counts grouped by content.
func (r MistakeRepository) ErrorCountsByContent(ctx context.Context, userID string) (map[string]int, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT content_id, count(id)::int
		FROM public.content_attempts
		WHERE student_id = $1 AND is_correct = false
		GROUP BY content_id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var contentID string
		var count int
		if err := rows.Scan(&contentID, &count); err != nil {
			return nil, err
		}
		counts[contentID] = count
	}
	return counts, rows.Err()
}

// CountSubmittedAttempts counts submitted attempts in an optional time window.
func (r MistakeRepository) CountSubmittedAttempts(ctx context.Context, userID string, start *time.Time, end *time.Time) (int, error) {
	var count int
	err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.content_attempts
		WHERE
			student_id = $1 AND
			submitted_at IS NOT NULL AND
			($2::timestamp IS NULL OR submitted_at >= $2) AND
			($3::timestamp IS NULL OR submitted_at <= $3)`,
		userID,
		start,
		end,
	).Scan(&count)
	return count, err
}

// UpdateProfileMastery replaces a student's mastery vector.
func (r MistakeRepository) UpdateProfileMastery(ctx context.Context, userID string, mastery map[string]float64, updatedAt time.Time) (bool, error) {
	raw, err := json.Marshal(mastery)
	if err != nil {
		return false, err
	}
	tag, err := r.DB().Exec(ctx, `
		UPDATE public.student_profiles
		SET mastery_vector = $2::json, updated_at = $3
		WHERE student_id = $1`,
		userID,
		string(raw),
		updatedAt,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteAttempt deletes one attempt owned by the student.
func (r MistakeRepository) DeleteAttempt(ctx context.Context, userID string, attemptID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		DELETE FROM public.content_attempts
		WHERE id = $1 AND student_id = $2`,
		attemptID,
		userID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

const mistakeSelectColumns = `
	ca.id,
	ca.content_id,
	ca.student_answer,
	ca.student_steps,
	ca.is_correct,
	ca.score,
	ca.submitted_at,
	ca.time_spent_seconds,
	c.id,
	c.type::text,
	c.title,
	c.body,
	c.difficulty,
	c.concept_ids,
	c.meta,
	dr.error_type::text,
	dr.error_subtype,
	dr.severity,
	dr.explanation,
	dr.suggestion,
	dr.related_concept_ids,
	dr.error_step_index`

func scanOptionalMistakeRow(row pgx.Row) (mistakeapp.MistakeRow, bool, error) {
	mistake, err := scanMistake(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return mistakeapp.MistakeRow{}, false, nil
		}
		return mistakeapp.MistakeRow{}, false, err
	}
	return mistake, true, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanMistakeRow(rows pgx.Rows) (mistakeapp.MistakeRow, error) {
	return scanMistake(rows)
}

func scanMistake(scanner rowScanner) (mistakeapp.MistakeRow, error) {
	var attempt mistakeapp.Attempt
	var content mistakeapp.Content
	var diagnosis mistakeapp.Diagnosis
	var studentStepsRaw []byte
	var conceptIDsRaw []byte
	var metaRaw []byte
	var submittedAt pgtype.Timestamp
	var errorType pgtype.Text
	var errorSubtype pgtype.Text
	var relatedConceptIDsRaw []byte
	var errorStepIndex pgtype.Int4

	if err := scanner.Scan(
		&attempt.ID,
		&attempt.ContentID,
		&attempt.StudentAnswer,
		&studentStepsRaw,
		&attempt.IsCorrect,
		&attempt.Score,
		&submittedAt,
		&attempt.TimeSpentSeconds,
		&content.ID,
		&content.Type,
		&content.Title,
		&content.Body,
		&content.Difficulty,
		&conceptIDsRaw,
		&metaRaw,
		&errorType,
		&errorSubtype,
		&diagnosis.Severity,
		&diagnosis.Explanation,
		&diagnosis.Suggestion,
		&relatedConceptIDsRaw,
		&errorStepIndex,
	); err != nil {
		return mistakeapp.MistakeRow{}, err
	}
	if submittedAt.Valid {
		value := submittedAt.Time
		attempt.SubmittedAt = &value
	}
	studentSteps, err := decodeStringSlice(studentStepsRaw)
	if err != nil {
		return mistakeapp.MistakeRow{}, fmt.Errorf("decode student steps: %w", err)
	}
	conceptIDs, err := decodeStringSlice(conceptIDsRaw)
	if err != nil {
		return mistakeapp.MistakeRow{}, fmt.Errorf("decode concept ids: %w", err)
	}
	meta, err := decodeObjectMap(metaRaw)
	if err != nil {
		return mistakeapp.MistakeRow{}, fmt.Errorf("decode content meta: %w", err)
	}
	relatedConceptIDs, err := decodeStringSlice(relatedConceptIDsRaw)
	if err != nil {
		return mistakeapp.MistakeRow{}, fmt.Errorf("decode related concept ids: %w", err)
	}
	if errorType.Valid {
		value := errorType.String
		diagnosis.ErrorType = &value
	}
	if errorSubtype.Valid {
		diagnosis.ErrorSubtype = errorSubtype.String
	}
	if errorStepIndex.Valid {
		value := int(errorStepIndex.Int32)
		diagnosis.ErrorStepIndex = &value
	}
	attempt.StudentSteps = studentSteps
	content.ConceptIDs = conceptIDs
	content.Meta = meta
	diagnosis.RelatedConceptIDs = relatedConceptIDs
	return mistakeapp.MistakeRow{Attempt: attempt, Content: content, Diagnosis: diagnosis}, nil
}

func scanAttemptAndContent(scanner rowScanner, attempt *mistakeapp.Attempt, content *mistakeapp.Content) error {
	var studentStepsRaw []byte
	var conceptIDsRaw []byte
	var metaRaw []byte
	var submittedAt pgtype.Timestamp
	if err := scanner.Scan(
		&attempt.ID,
		&attempt.ContentID,
		&attempt.StudentAnswer,
		&studentStepsRaw,
		&attempt.IsCorrect,
		&attempt.Score,
		&submittedAt,
		&attempt.TimeSpentSeconds,
		&content.ID,
		&content.Type,
		&content.Title,
		&content.Body,
		&content.Difficulty,
		&conceptIDsRaw,
		&metaRaw,
	); err != nil {
		return err
	}
	if submittedAt.Valid {
		value := submittedAt.Time
		attempt.SubmittedAt = &value
	}
	studentSteps, err := decodeStringSlice(studentStepsRaw)
	if err != nil {
		return fmt.Errorf("decode student steps: %w", err)
	}
	conceptIDs, err := decodeStringSlice(conceptIDsRaw)
	if err != nil {
		return fmt.Errorf("decode concept ids: %w", err)
	}
	meta, err := decodeObjectMap(metaRaw)
	if err != nil {
		return fmt.Errorf("decode content meta: %w", err)
	}
	attempt.StudentSteps = studentSteps
	content.ConceptIDs = conceptIDs
	content.Meta = meta
	return nil
}

func decodeObjectMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	values := map[string]any{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}
