package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	progressapp "mathstudy/backend/internal/application/progress"
)

// AttemptInsightsForStudents returns comparable activity over a shared window.
// A non-empty teacher ID restricts the cohort to common teacher-owned content.
func (r ProgressRepository) AttemptInsightsForStudents(ctx context.Context, teacherID string, studentIDs []string, start time.Time, end time.Time) (map[string]progressapp.StudentAttemptInsight, error) {
	stats := map[string]progressapp.StudentAttemptInsight{}
	if len(studentIDs) == 0 {
		return stats, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT
			ca.student_id,
			count(ca.id)::int AS attempt_count,
			(count(ca.id) FILTER (WHERE ca.is_correct))::int AS correct_count,
			coalesce(sum(ca.time_spent_seconds), 0)::int AS study_seconds,
			count(DISTINCT date_trunc('day', ca.submitted_at AT TIME ZONE 'UTC' AT TIME ZONE 'Asia/Shanghai'))::int AS active_days
		FROM public.content_attempts ca
		JOIN public.contents c ON c.id = ca.content_id
		WHERE ($1 = '' OR (c.owner_teacher_id = $1 AND c.generated_by_student_id IS NULL))
			AND ca.student_id = ANY($2::varchar[])
			AND ca.submitted_at IS NOT NULL
			AND ca.submitted_at >= $3
			AND ca.submitted_at <= $4
		GROUP BY ca.student_id`,
		teacherID,
		studentIDs,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var studentID string
		var stat progressapp.StudentAttemptInsight
		if err := rows.Scan(&studentID, &stat.AttemptCount, &stat.CorrectCount, &stat.StudySeconds, &stat.ActiveDays); err != nil {
			return nil, err
		}
		stats[studentID] = stat
	}
	return stats, rows.Err()
}

// MasteryStatesForStudents returns current DKT states without exposing student identities to HTTP clients.
func (r ProgressRepository) MasteryStatesForStudents(ctx context.Context, studentIDs []string) ([]progressapp.StudentMasteryInsight, error) {
	if len(studentIDs) == 0 {
		return []progressapp.StudentMasteryInsight{}, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT student_id, concept_id, mastery_prob, confidence, attempt_count, last_attempt_at
		FROM public.student_concept_dkt_states
		WHERE student_id = ANY($1::varchar[])
		ORDER BY concept_id, student_id`,
		studentIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := []progressapp.StudentMasteryInsight{}
	for rows.Next() {
		var state progressapp.StudentMasteryInsight
		var lastAttempt pgtype.Timestamp
		if err := rows.Scan(
			&state.StudentID,
			&state.ConceptID,
			&state.Mastery,
			&state.Confidence,
			&state.AttemptCount,
			&lastAttempt,
		); err != nil {
			return nil, err
		}
		state.LastAttemptAt = timestampPtr(lastAttempt)
		states = append(states, state)
	}
	return states, rows.Err()
}

// KnowledgeNodeNames resolves only the concepts present in the portrait cohort.
func (r ProgressRepository) KnowledgeNodeNames(ctx context.Context, conceptIDs []string) (map[string]string, error) {
	names := map[string]string{}
	if len(conceptIDs) == 0 {
		return names, nil
	}
	rows, err := r.DB().Query(ctx, `
		SELECT id, name
		FROM public.knowledge_nodes
		WHERE id = ANY($1::varchar[])`,
		conceptIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		names[id] = name
	}
	return names, rows.Err()
}

// ListPortraitActionProgresses derives every persisted action's progress from its stable DKT baseline.
func (r ProgressRepository) ListPortraitActionProgresses(ctx context.Context, userID string) (map[string]progressapp.PortraitActionProgress, error) {
	progresses := map[string]progressapp.PortraitActionProgress{}
	rows, err := r.DB().Query(ctx, `
		SELECT
			pa.concept_id,
			pa.target_count,
			pa.started_at,
			greatest(coalesce(state.attempt_count, 0) - pa.baseline_attempt_count, 0)::int AS completed_count
		FROM public.student_portrait_actions pa
		LEFT JOIN public.student_concept_dkt_states state
			ON state.student_id = pa.student_id
			AND state.concept_id = pa.concept_id
		WHERE pa.student_id = $1
			AND pa.action_type = 'practice'`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var progress progressapp.PortraitActionProgress
		if err := rows.Scan(&progress.ConceptID, &progress.TargetCount, &progress.StartedAt, &progress.CompletedCount); err != nil {
			return nil, err
		}
		progresses[progress.ConceptID] = progress
	}
	return progresses, rows.Err()
}

// StartPortraitAction creates one stable practice baseline for a valid knowledge concept.
func (r ProgressRepository) StartPortraitAction(ctx context.Context, userID string, conceptID string, startedAt time.Time) (bool, error) {
	id, err := newUUID()
	if err != nil {
		return false, err
	}
	var savedConceptID string
	err = r.DB().QueryRow(ctx, `
		INSERT INTO public.student_portrait_actions AS current_action (
			id, student_id, concept_id, action_type, target_count,
			baseline_attempt_count, started_at, created_at, updated_at
		)
		SELECT
			$1, $2, kn.id, 'practice', 10,
			coalesce(state.attempt_count, 0), $4, $4, $4
		FROM public.knowledge_nodes kn
		LEFT JOIN public.student_concept_dkt_states state
			ON state.student_id = $2
			AND state.concept_id = kn.id
		WHERE kn.id = $3
		ON CONFLICT (student_id, concept_id, action_type) DO UPDATE SET
			baseline_attempt_count = CASE
				WHEN EXCLUDED.baseline_attempt_count - current_action.baseline_attempt_count >= current_action.target_count
					THEN EXCLUDED.baseline_attempt_count
				ELSE current_action.baseline_attempt_count
			END,
			started_at = CASE
				WHEN EXCLUDED.baseline_attempt_count - current_action.baseline_attempt_count >= current_action.target_count
					THEN EXCLUDED.started_at
				ELSE current_action.started_at
			END,
			updated_at = EXCLUDED.updated_at
		RETURNING concept_id`,
		id,
		userID,
		conceptID,
		startedAt,
	).Scan(&savedConceptID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return savedConceptID != "", nil
}
