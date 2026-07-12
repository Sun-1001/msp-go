package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errProgressQueryRecorded = errors.New("progress query recorded")

type recordingProgressQuerier struct {
	queryRowSQL  string
	queryRowArgs []any
	querySQL     string
	queryArgs    []any
	row          pgx.Row
}

func (*recordingProgressQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *recordingProgressQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.querySQL = sql
	q.queryArgs = append([]any(nil), args...)
	return nil, errProgressQueryRecorded
}

func (q *recordingProgressQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.queryRowSQL = sql
	q.queryRowArgs = append([]any(nil), args...)
	return q.row
}

type progressClassRow struct{}

func (progressClassRow) Scan(dest ...any) error {
	*dest[0].(*string) = "class-1"
	*dest[1].(*string) = "teacher-1"
	return nil
}

func TestProgressRepositoryListClassStudentIDsLoadsTeacherScope(t *testing.T) {
	querier := &recordingProgressQuerier{row: progressClassRow{}}
	repo, err := NewProgressRepository(querier)
	if err != nil {
		t.Fatalf("NewProgressRepository() error = %v", err)
	}

	_, _, _, err = repo.ListClassStudentIDs(context.Background(), "student-1")
	if !errors.Is(err, errProgressQueryRecorded) {
		t.Fatalf("ListClassStudentIDs() error = %v, want %v", err, errProgressQueryRecorded)
	}
	for _, fragment := range []string{
		"SELECT ce.class_id, c.teacher_id",
		"JOIN public.classes c ON c.id = ce.class_id",
		"WHERE ce.student_id = $1",
	} {
		if !strings.Contains(querier.queryRowSQL, fragment) {
			t.Fatalf("class scope query missing %q: %s", fragment, querier.queryRowSQL)
		}
	}
	if len(querier.queryRowArgs) != 1 || querier.queryRowArgs[0] != "student-1" {
		t.Fatalf("class scope args = %#v", querier.queryRowArgs)
	}
	if len(querier.queryArgs) != 1 || querier.queryArgs[0] != "class-1" {
		t.Fatalf("class student args = %#v", querier.queryArgs)
	}
}

func TestProgressRepositoryAttemptStatsScopesToCurrentTeacherContent(t *testing.T) {
	querier := &recordingProgressQuerier{}
	repo, err := NewProgressRepository(querier)
	if err != nil {
		t.Fatalf("NewProgressRepository() error = %v", err)
	}
	studentIDs := []string{"student-1", "student-2"}

	_, err = repo.AttemptStatsForStudents(context.Background(), "teacher-1", studentIDs)
	if !errors.Is(err, errProgressQueryRecorded) {
		t.Fatalf("AttemptStatsForStudents() error = %v, want %v", err, errProgressQueryRecorded)
	}
	for _, fragment := range []string{
		"FROM public.content_attempts ca",
		"JOIN public.contents c ON c.id = ca.content_id",
		"c.owner_teacher_id = $1",
		"c.generated_by_student_id IS NULL",
		"ca.student_id = ANY($2::varchar[])",
	} {
		if !strings.Contains(querier.querySQL, fragment) {
			t.Fatalf("attempt stats query missing %q: %s", fragment, querier.querySQL)
		}
	}
	if len(querier.queryArgs) != 2 || querier.queryArgs[0] != "teacher-1" {
		t.Fatalf("attempt stats args = %#v", querier.queryArgs)
	}
}
