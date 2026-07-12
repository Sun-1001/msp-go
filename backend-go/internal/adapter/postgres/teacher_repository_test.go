package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errTeacherQueryRecorded = errors.New("teacher query recorded")

type recordingTeacherQuerier struct {
	querySQL  string
	queryArgs []any
}

func (*recordingTeacherQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *recordingTeacherQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.record(sql, args)
	return nil, errTeacherQueryRecorded
}

func (q *recordingTeacherQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.record(sql, args)
	return teacherQueryErrorRow{}
}

func (q *recordingTeacherQuerier) record(sql string, args []any) {
	q.querySQL = sql
	q.queryArgs = append([]any(nil), args...)
}

type teacherQueryErrorRow struct{}

func (teacherQueryErrorRow) Scan(...any) error {
	return errTeacherQueryRecorded
}

func TestTeacherAttemptQueriesScopeResultsToOwnedContent(t *testing.T) {
	now := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
	studentIDs := []string{"student-1", "student-2"}
	tests := []struct {
		name string
		call func(TeacherRepository) error
	}{
		{
			name: "average attempt score",
			call: func(repo TeacherRepository) error {
				_, _, err := repo.AverageAttemptScore(context.Background(), "teacher-1", studentIDs, &now)
				return err
			},
		},
		{
			name: "sum attempt seconds",
			call: func(repo TeacherRepository) error {
				_, err := repo.SumAttemptSeconds(context.Background(), "teacher-1", studentIDs, &now)
				return err
			},
		},
		{
			name: "distinct attempt students",
			call: func(repo TeacherRepository) error {
				_, err := repo.CountDistinctAttemptStudentsSince(context.Background(), "teacher-1", studentIDs, now)
				return err
			},
		},
		{
			name: "top students",
			call: func(repo TeacherRepository) error {
				_, err := repo.TopStudentsByAverageScore(context.Background(), "teacher-1", studentIDs, 5)
				return err
			},
		},
		{
			name: "common errors",
			call: func(repo TeacherRepository) error {
				_, err := repo.CommonErrors(context.Background(), "teacher-1", studentIDs, 10)
				return err
			},
		},
		{
			name: "low score students",
			call: func(repo TeacherRepository) error {
				_, err := repo.LowScoreStudents(context.Background(), "teacher-1", studentIDs, 60)
				return err
			},
		},
		{
			name: "student profile counters",
			call: func(repo TeacherRepository) error {
				_, _, err := repo.GetProfile(context.Background(), "teacher-1", "student-1")
				return err
			},
		},
		{
			name: "student average score",
			call: func(repo TeacherRepository) error {
				_, _, err := repo.AverageStudentScore(context.Background(), "teacher-1", "student-1")
				return err
			},
		},
		{
			name: "student rank",
			call: func(repo TeacherRepository) error {
				_, err := repo.RankByAverageScore(context.Background(), "teacher-1", studentIDs)
				return err
			},
		},
		{
			name: "attempt concept counts",
			call: func(repo TeacherRepository) error {
				_, err := repo.AttemptConceptCounts(context.Background(), "teacher-1", "student-1")
				return err
			},
		},
		{
			name: "recent attempts",
			call: func(repo TeacherRepository) error {
				_, err := repo.RecentAttempts(context.Background(), "teacher-1", "student-1", 10)
				return err
			},
		},
		{
			name: "recent mistakes",
			call: func(repo TeacherRepository) error {
				_, err := repo.RecentMistakes(context.Background(), "teacher-1", "student-1", 5)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			querier := &recordingTeacherQuerier{}
			repo, err := NewTeacherRepository(querier)
			if err != nil {
				t.Fatalf("NewTeacherRepository() error = %v", err)
			}
			if err := tt.call(repo); !errors.Is(err, errTeacherQueryRecorded) {
				t.Fatalf("query error = %v, want %v", err, errTeacherQueryRecorded)
			}
			for _, fragment := range []string{
				"public.contents c",
				"c.owner_teacher_id = $1",
				"c.generated_by_student_id IS NULL",
			} {
				if !strings.Contains(querier.querySQL, fragment) {
					t.Fatalf("query missing %q: %s", fragment, querier.querySQL)
				}
			}
			if len(querier.queryArgs) == 0 || querier.queryArgs[0] != "teacher-1" {
				t.Fatalf("query args = %#v, want teacher ID first", querier.queryArgs)
			}
		})
	}
}
