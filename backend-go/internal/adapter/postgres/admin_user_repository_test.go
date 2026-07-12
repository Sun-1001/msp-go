package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type recordingUserDeleteQuerier struct {
	execSQL []string
}

func (q *recordingUserDeleteQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	return pgconn.NewCommandTag("DELETE 1"), nil
}

func (*recordingUserDeleteQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (*recordingUserDeleteQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return boolRow{value: true}
}

func TestDeleteUserRemovesStudentGeneratedExercisesAndAttempts(t *testing.T) {
	querier := &recordingUserDeleteQuerier{}
	repo, err := NewUserRepository(querier)
	if err != nil {
		t.Fatalf("NewUserRepository() error = %v", err)
	}

	deleted, err := repo.DeleteUser(context.Background(), "student-1")
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if !deleted {
		t.Fatal("DeleteUser() = false, want true")
	}

	var attemptsSQL string
	var contentsSQL string
	for _, sql := range querier.execSQL {
		if strings.Contains(sql, "DELETE FROM public.content_attempts") {
			attemptsSQL = sql
		}
		if strings.Contains(sql, "DELETE FROM public.contents") {
			contentsSQL = sql
		}
	}
	if !strings.Contains(attemptsSQL, "generated_by_student_id = $1") {
		t.Fatalf("content attempts cleanup SQL = %s", attemptsSQL)
	}
	if !strings.Contains(contentsSQL, "generated_by_student_id = $1") {
		t.Fatalf("contents cleanup SQL = %s", contentsSQL)
	}
}
