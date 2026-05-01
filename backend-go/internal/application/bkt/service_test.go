package bkt

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListParamsValidatesPaginationAndReturnsRows(t *testing.T) {
	repo := &fakeBKTRepo{
		params: []Param{{ConceptID: "node-1", PL0: 0.25, PT: 0.12, PG: 0.2, PS: 0.1}},
		total:  1,
	}
	service := newBKTTestService(repo, time.Now())

	if _, err := service.ListParams(context.Background(), -1, 50); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ListParams(invalid offset) error = %v, want ErrBadRequest", err)
	}
	if _, err := service.ListParams(context.Background(), 0, 201); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ListParams(invalid limit) error = %v, want ErrBadRequest", err)
	}

	response, err := service.ListParams(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("ListParams() error = %v", err)
	}
	if response.Total != 1 || response.Offset != 10 || response.Limit != 20 || len(response.Items) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if repo.lastOffset != 10 || repo.lastLimit != 20 {
		t.Fatalf("offset=%d limit=%d", repo.lastOffset, repo.lastLimit)
	}
}

func TestUpdateParamValidatesAndMapsMissing(t *testing.T) {
	now := time.Date(2026, time.May, 1, 8, 0, 0, 0, time.UTC)
	repo := &fakeBKTRepo{}
	service := newBKTTestService(repo, now)

	badGuess := 0.6
	if _, err := service.UpdateParam(context.Background(), "node-1", Update{PG: &badGuess}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UpdateParam(invalid) error = %v, want ErrBadRequest", err)
	}

	value := 0.3
	if _, err := service.UpdateParam(context.Background(), "node-1", Update{PL0: &value}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateParam(missing) error = %v, want ErrNotFound", err)
	}

	repo.updatedOK = true
	repo.updatedParam = Param{ConceptID: "node-1", PL0: 0.3, PT: 0.12, PG: 0.2, PS: 0.1}
	param, err := service.UpdateParam(context.Background(), " node-1 ", Update{PL0: &value})
	if err != nil {
		t.Fatalf("UpdateParam() error = %v", err)
	}
	if param.PL0 != 0.3 || repo.lastConceptID != "node-1" || repo.lastUpdate.PL0 == nil || !repo.lastNow.Equal(now) {
		t.Fatalf("param=%#v concept=%q update=%#v now=%v", param, repo.lastConceptID, repo.lastUpdate, repo.lastNow)
	}
}

func TestResetParamUsesDefaults(t *testing.T) {
	now := time.Date(2026, time.May, 1, 9, 0, 0, 0, time.UTC)
	repo := &fakeBKTRepo{resetOK: true, resetParam: Param{ConceptID: "node-1", PL0: 0.25, PT: 0.12, PG: 0.2, PS: 0.1}}
	service := newBKTTestService(repo, now)

	param, err := service.ResetParam(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("ResetParam() error = %v", err)
	}
	if param != repo.resetParam || repo.lastDefault.PL0 != 0.25 || repo.lastDefault.PT != 0.12 || !repo.lastNow.Equal(now) {
		t.Fatalf("param=%#v default=%#v now=%v", param, repo.lastDefault, repo.lastNow)
	}
}

func TestSeedDefaultParamsBuildsPythonCompatibleMessage(t *testing.T) {
	service := newBKTTestService(&fakeBKTRepo{seededCount: 0}, time.Now())
	response, err := service.SeedDefaultParams(context.Background())
	if err != nil {
		t.Fatalf("SeedDefaultParams() error = %v", err)
	}
	if response.SeededCount != 0 || response.Message != "所有知识点已有 BKT 参数" {
		t.Fatalf("response = %#v", response)
	}

	service = newBKTTestService(&fakeBKTRepo{seededCount: 3}, time.Now())
	response, err = service.SeedDefaultParams(context.Background())
	if err != nil {
		t.Fatalf("SeedDefaultParams() error = %v", err)
	}
	if response.SeededCount != 3 || response.Message != "已为 3 个知识点创建默认 BKT 参数" {
		t.Fatalf("response = %#v", response)
	}
}

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func newBKTTestService(repo *fakeBKTRepo, now time.Time) *Service {
	service, err := NewService(repo)
	if err != nil {
		panic(err)
	}
	service.now = func() time.Time { return now }
	return service
}

type fakeBKTRepo struct {
	params        []Param
	total         int
	updatedParam  Param
	updatedOK     bool
	resetParam    Param
	resetOK       bool
	seededCount   int
	lastOffset    int
	lastLimit     int
	lastConceptID string
	lastUpdate    Update
	lastDefault   Param
	lastNow       time.Time
}

func (r *fakeBKTRepo) ListParams(_ context.Context, offset int, limit int) ([]Param, int, error) {
	r.lastOffset = offset
	r.lastLimit = limit
	return r.params, r.total, nil
}

func (r *fakeBKTRepo) UpdateParam(_ context.Context, conceptID string, update Update, now time.Time) (Param, bool, error) {
	r.lastConceptID = conceptID
	r.lastUpdate = update
	r.lastNow = now
	return r.updatedParam, r.updatedOK, nil
}

func (r *fakeBKTRepo) ResetParam(_ context.Context, conceptID string, defaults Param, now time.Time) (Param, bool, error) {
	r.lastConceptID = conceptID
	r.lastDefault = defaults
	r.lastNow = now
	return r.resetParam, r.resetOK, nil
}

func (r *fakeBKTRepo) SeedDefaultParams(_ context.Context, defaults Param, now time.Time) (int, error) {
	r.lastDefault = defaults
	r.lastNow = now
	return r.seededCount, nil
}
