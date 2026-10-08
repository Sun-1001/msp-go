package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"mathstudy/backend/internal/application/learningfeedback"
)

// GetExerciseAssistance must be called under LockStudentTracking for submission.
// Absence means no platform exercise hint was recorded, not proof of no help.
func (r ExerciseRepository) GetExerciseAssistance(ctx context.Context, studentID, exerciseID string) (string, error) {
	if strings.TrimSpace(studentID) == "" || strings.TrimSpace(exerciseID) == "" {
		return "", errors.New("exercise assistance requires student and exercise IDs")
	}
	var status string
	err := r.DB().QueryRow(ctx, `SELECT status FROM public.exercise_hint_evidence
		WHERE student_id = $1 AND exercise_id = $2`, studentID, exerciseID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return learningfeedback.AssistanceNoneRecorded, nil
	}
	if err != nil {
		return "", fmt.Errorf("read exercise assistance: %w", err)
	}
	return status, nil
}
