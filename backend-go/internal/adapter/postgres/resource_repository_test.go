package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	resourceapp "mathstudy/backend-go/internal/application/resource"
)

type recordingResourceQuerier struct {
	querySQL string
	execSQL  string
	row      pgx.Row
	execTag  pgconn.CommandTag
}

func (q *recordingResourceQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	q.execSQL = sql
	return q.execTag, nil
}

func (*recordingResourceQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *recordingResourceQuerier) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	q.querySQL = sql
	return q.row
}

type resourceErrorRow struct {
	err error
}

func (r resourceErrorRow) Scan(...any) error {
	return r.err
}

func TestNewResourceRepositoryRejectsNilQuerier(t *testing.T) {
	if _, err := NewResourceRepository(nil); err == nil {
		t.Fatal("NewResourceRepository(nil) error = nil, want error")
	}
}

func TestResourceRepositoryGetByIDExcludesProblemContents(t *testing.T) {
	querier := &recordingResourceQuerier{row: resourceErrorRow{err: pgx.ErrNoRows}}
	repo, err := NewResourceRepository(querier)
	if err != nil {
		t.Fatalf("NewResourceRepository() error = %v", err)
	}

	_, ok, err := repo.GetResourceByID(context.Background(), "generated-problem", "student-1")
	if err != nil {
		t.Fatalf("GetResourceByID() error = %v", err)
	}
	if ok {
		t.Fatal("GetResourceByID() ok = true, want false")
	}
	if !strings.Contains(querier.querySQL, resourceContentTypeCondition) {
		t.Fatalf("GetResourceByID() SQL missing resource type scope: %s", querier.querySQL)
	}
}

func TestResourceRepositoryToggleFavoriteExcludesProblemContents(t *testing.T) {
	querier := &recordingResourceQuerier{row: boolRow{value: false}}
	repo, err := NewResourceRepository(querier)
	if err != nil {
		t.Fatalf("NewResourceRepository() error = %v", err)
	}

	_, found, err := repo.ToggleFavorite(context.Background(), "student-1", "generated-problem")
	if err != nil {
		t.Fatalf("ToggleFavorite() error = %v", err)
	}
	if found {
		t.Fatal("ToggleFavorite() found = true, want false")
	}
	if !strings.Contains(querier.querySQL, resourceContentTypeCondition) {
		t.Fatalf("ToggleFavorite() SQL missing resource type scope: %s", querier.querySQL)
	}
}

func TestResourceRepositoryMutationsExcludeProblemContents(t *testing.T) {
	querier := &recordingResourceQuerier{
		row:     resourceErrorRow{err: pgx.ErrNoRows},
		execTag: pgconn.NewCommandTag("UPDATE 0"),
	}
	repo, err := NewResourceRepository(querier)
	if err != nil {
		t.Fatalf("NewResourceRepository() error = %v", err)
	}

	_, found, err := repo.UpdateResource(
		context.Background(),
		"class-problem",
		"teacher-1",
		resourceapp.ResourceUpdate{},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("UpdateResource() error = %v", err)
	}
	if found {
		t.Fatal("UpdateResource() found = true, want false")
	}
	if !strings.Contains(querier.querySQL, resourceContentTypeCondition) {
		t.Fatalf("UpdateResource() SQL missing resource type scope: %s", querier.querySQL)
	}

	_, err = repo.DeleteResource(context.Background(), "class-problem", "teacher-1", time.Now())
	if err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}
	if !strings.Contains(querier.execSQL, resourceContentTypeCondition) {
		t.Fatalf("DeleteResource() SQL missing resource type scope: %s", querier.execSQL)
	}
}
