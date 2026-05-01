package admininbox

import (
	"context"
	"errors"
	"testing"
	"time"

	authapp "mathstudy/backend-go/internal/application/auth"
)

func TestListRequestsNormalizesFilter(t *testing.T) {
	repo := &fakeRepository{items: []RequestItem{{ID: "reset-1", Status: StatusPending}}, total: 1, pendingCount: 3}
	service := newTestService(t, repo, nil)

	response, err := service.ListRequests(context.Background(), ListFilter{Status: " pending "})
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if repo.lastFilter.Status != "pending" || repo.lastFilter.Page != 1 || repo.lastFilter.PageSize != 20 {
		t.Fatalf("filter = %#v", repo.lastFilter)
	}
	if response.Total != 1 || response.PendingCount != 3 || len(response.Items) != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestListRequestsRejectsInvalidFilters(t *testing.T) {
	service := newTestService(t, &fakeRepository{}, nil)

	_, err := service.ListRequests(context.Background(), ListFilter{Status: "done", Page: 1, PageSize: 20})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("invalid status error = %v, want ErrBadRequest", err)
	}

	_, err = service.ListRequests(context.Background(), ListFilter{Page: 0, PageSize: 101})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("invalid page_size error = %v, want ErrBadRequest", err)
	}
}

func TestReviewRequestApprovesAndClearsLoginFailures(t *testing.T) {
	repo := &fakeRepository{reviewResult: ReviewResult{Found: true, UserFound: true, Username: "student"}}
	clearer := &fakeClearer{}
	service := newTestService(t, repo, clearer)
	service.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }

	response, err := service.ReviewRequest(context.Background(), "reset-1", "admin-1", "approve", nil)
	if err != nil {
		t.Fatalf("ReviewRequest(approve) error = %v", err)
	}
	if !response.Success || response.TempPassword == nil || len(*response.TempPassword) != tempPasswordLength {
		t.Fatalf("response = %#v", response)
	}
	if repo.lastReview.Action != "approve" || repo.lastReview.PasswordHash == nil {
		t.Fatalf("review update = %#v", repo.lastReview)
	}
	if !authapp.VerifyPassword(*response.TempPassword, *repo.lastReview.PasswordHash) {
		t.Fatal("temporary password does not match stored hash")
	}
	if clearer.username != "student" {
		t.Fatalf("cleared username = %q", clearer.username)
	}
}

func TestReviewRequestRejectsAndHandlesBusinessFailures(t *testing.T) {
	reason := "账号信息不匹配"
	repo := &fakeRepository{reviewResult: ReviewResult{Found: true, UserFound: true, Username: "student"}}
	service := newTestService(t, repo, nil)

	response, err := service.ReviewRequest(context.Background(), "reset-1", "admin-1", "reject", &reason)
	if err != nil {
		t.Fatalf("ReviewRequest(reject) error = %v", err)
	}
	if !response.Success || response.TempPassword != nil || repo.lastReview.RejectReason == nil || *repo.lastReview.RejectReason != reason {
		t.Fatalf("response=%#v update=%#v", response, repo.lastReview)
	}

	repo.reviewResult = ReviewResult{Found: false}
	response, err = service.ReviewRequest(context.Background(), "missing", "admin-1", "approve", nil)
	if err != nil {
		t.Fatalf("ReviewRequest(missing) error = %v", err)
	}
	if response.Success || response.Message != "申请不存在" {
		t.Fatalf("missing response = %#v", response)
	}

	response, err = service.ReviewRequest(context.Background(), "reset-1", "admin-1", "invalid", nil)
	if err != nil {
		t.Fatalf("ReviewRequest(invalid action) error = %v", err)
	}
	if response.Success || response.Message != "无效的操作" {
		t.Fatalf("invalid action response = %#v", response)
	}
}

func TestPendingCountUsesRepositoryCounter(t *testing.T) {
	repo := &fakeRepository{pendingCount: 7}
	service := newTestService(t, repo, nil)

	count, err := service.PendingCount(context.Background())
	if err != nil {
		t.Fatalf("PendingCount() error = %v", err)
	}
	if count != 7 || !repo.countPendingCalled {
		t.Fatalf("count=%d countPendingCalled=%t", count, repo.countPendingCalled)
	}
}

func newTestService(t *testing.T, repo Repository, clearer LoginFailureClearer) *Service {
	t.Helper()
	service, err := NewService(repo, clearer)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

type fakeRepository struct {
	items              []RequestItem
	total              int
	pendingCount       int
	reviewResult       ReviewResult
	err                error
	lastFilter         ListFilter
	lastReview         ReviewUpdate
	countPendingCalled bool
}

func (r *fakeRepository) ListPasswordResetRequests(_ context.Context, filter ListFilter) ([]RequestItem, int, int, error) {
	r.lastFilter = filter
	return r.items, r.total, r.pendingCount, r.err
}

func (r *fakeRepository) CountPendingPasswordResetRequests(context.Context) (int, error) {
	r.countPendingCalled = true
	return r.pendingCount, r.err
}

func (r *fakeRepository) ReviewPasswordResetRequest(_ context.Context, update ReviewUpdate) (ReviewResult, error) {
	r.lastReview = update
	return r.reviewResult, r.err
}

type fakeClearer struct {
	username string
}

func (c *fakeClearer) Clear(_ context.Context, username string) {
	c.username = username
}
