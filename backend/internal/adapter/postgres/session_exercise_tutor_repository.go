package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	sessionapp "mathstudy/backend/internal/application/session"
)

func (r SessionRepository) PrepareExerciseTutor(ctx context.Context, session sessionapp.LearningSession, messages []sessionapp.Message, request sessionapp.FirstChatRequest, exerciseID string) (string, error) {
	var resolvedID string
	err := withRepositoryTx(ctx, "exercise tutor create", r.Repository, func(base Repository) SessionRepository {
		return SessionRepository{Repository: base}
	}, func(current SessionRepository) error {
		// Separate from learning tracking locks: hint chats never write mastery.
		if _, err := current.DB().Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('exercise-tutor:' || $1 || ':' || $2, 0))`, session.StudentID, exerciseID); err != nil {
			return err
		}
		err := current.DB().QueryRow(ctx, `
            SELECT t.session_id FROM public.session_exercise_tutors t
            JOIN public.learning_sessions s ON s.id = t.session_id AND s.student_id = t.student_id
            WHERE t.student_id = $1 AND t.exercise_id = $2 AND t.is_current AND s.is_active`,
			session.StudentID, exerciseID).Scan(&resolvedID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err := current.DB().Exec(ctx, `UPDATE public.session_exercise_tutors SET is_current = false WHERE student_id = $1 AND exercise_id = $2 AND is_current`, session.StudentID, exerciseID); err != nil {
			return err
		}
		inserted, err := current.insertSession(ctx, session)
		if err != nil {
			return err
		}
		if !inserted {
			return sessionapp.ErrSessionIDConflict
		}
		for _, message := range messages {
			if err := current.InsertMessage(ctx, message); err != nil {
				return err
			}
		}
		if err := current.insertFirstChatRequest(ctx, request); err != nil {
			return err
		}
		if _, err := current.DB().Exec(ctx, `INSERT INTO public.session_exercise_tutors (session_id, student_id, exercise_id) VALUES ($1, $2, $3)`, session.ID, session.StudentID, exerciseID); err != nil {
			return err
		}
		resolvedID = session.ID
		return nil
	})
	return resolvedID, err
}

func (r SessionRepository) GetTutorExerciseID(ctx context.Context, sessionID, studentID string) (string, error) {
	var exerciseID string
	err := r.DB().QueryRow(ctx, `SELECT t.exercise_id FROM public.session_exercise_tutors t
        JOIN public.learning_sessions s ON s.id = t.session_id AND s.student_id = t.student_id
        WHERE t.session_id = $1 AND t.student_id = $2`, sessionID, studentID).Scan(&exerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return exerciseID, err
}

// RecordExerciseHint runs before the first nonempty model chunk becomes available.
// It serializes with submission snapshots, but never writes mastery itself.
func (r SessionRepository) RecordExerciseHint(ctx context.Context, sessionID, studentID string) error {
	exerciseID, err := r.GetTutorExerciseID(ctx, sessionID, studentID)
	if err != nil || exerciseID == "" {
		return err
	}
	return withRepositoryTx(ctx, "exercise hint evidence", r.Repository, func(base Repository) SessionRepository {
		return SessionRepository{Repository: base}
	}, func(current SessionRepository) error {
		if err := lockStudentTracking(ctx, current.DB(), studentID); err != nil {
			return err
		}
		_, err := current.DB().Exec(ctx, `
			INSERT INTO public.exercise_hint_evidence (student_id, exercise_id, status, observed_at)
			VALUES ($1, $2, 'hinted', clock_timestamp() AT TIME ZONE 'UTC')
			ON CONFLICT (student_id, exercise_id) DO UPDATE
			SET status = 'hinted', observed_at = CASE
				WHEN exercise_hint_evidence.status = 'unknown' THEN EXCLUDED.observed_at
				ELSE exercise_hint_evidence.observed_at END`, studentID, exerciseID)
		return err
	})
}
