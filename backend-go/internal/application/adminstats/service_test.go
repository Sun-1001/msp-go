package adminstats

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func TestOverviewStatsCalculatesRatesAndTrends(t *testing.T) {
	repo := &fakeRepository{overview: OverviewSnapshot{
		TotalUsers:       20,
		StudentCount:     12,
		TeacherCount:     6,
		AdminCount:       2,
		ActiveUsersToday: 5,
		ThisWeekUsers:    4,
		LastWeekUsers:    2,
	}}
	service := newTestService(t, repo, nil)

	response, err := service.OverviewStats(context.Background())
	if err != nil {
		t.Fatalf("OverviewStats() error = %v", err)
	}
	if response.ActiveRate != 25 || response.Trends.UsersChange != 100 {
		t.Fatalf("response = %#v", response)
	}
	if !repo.lastToday.Equal(time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("today = %s", repo.lastToday)
	}
}

func TestUserGrowthBuildsCompleteDateSeries(t *testing.T) {
	repo := &fakeRepository{growth: GrowthSnapshot{
		Base: GrowthCounts{Total: 2, Students: 1, Teachers: 1},
		Daily: []DailyRoleCount{
			{Date: "2026-04-27", Role: "STUDENT", Count: 2},
			{Date: "2026-05-01", Role: "TEACHER", Count: 1},
		},
	}}
	service := newTestService(t, repo, nil)

	response, err := service.UserGrowth(context.Background(), "7d")
	if err != nil {
		t.Fatalf("UserGrowth() error = %v", err)
	}
	if response.Period != "7d" || len(response.Data) != 8 {
		t.Fatalf("response = %#v", response)
	}
	if response.Data[0].Date != "2026-04-26" || response.Data[1].Total != 4 || response.Data[5].Teachers != 2 {
		t.Fatalf("data = %#v", response.Data)
	}
	if response.Summary.TotalNewUsers != 3 || response.Summary.AvgDailyGrowth != 0.43 {
		t.Fatalf("summary = %#v", response.Summary)
	}

	_, err = service.UserGrowth(context.Background(), "365d")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UserGrowth(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestRecentActivitiesMapsRoleActions(t *testing.T) {
	displayName := "Teacher One"
	repo := &fakeRepository{recentUsers: []RecentUser{
		{ID: "admin-1", Username: "admin", Role: "ADMIN", CreatedAt: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)},
		{ID: "teacher-1", Username: "teacher", DisplayName: &displayName, Role: "TEACHER", CreatedAt: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)},
		{ID: "student-1", Username: "student", Role: "STUDENT", CreatedAt: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}}
	service := newTestService(t, repo, nil)

	response, err := service.RecentActivities(context.Background(), 3)
	if err != nil {
		t.Fatalf("RecentActivities() error = %v", err)
	}
	if response.Total != 3 || response.Items[0].Type != "warning" || response.Items[1].UserName != "Teacher One" || response.Items[2].ActionDisplay != "创建了新账户" {
		t.Fatalf("response = %#v", response)
	}
	if response.Items[0].ID != "user-admin-1" || response.Items[2].ID != "user-student-1" {
		t.Fatalf("items = %#v", response.Items)
	}

	_, err = service.RecentActivities(context.Background(), 51)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("RecentActivities(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestSystemStatusBuildsAlerts(t *testing.T) {
	provider := StatusProviderFunc(func(context.Context) ([]ServiceStatus, error) {
		return []ServiceStatus{
			{Name: "PostgreSQL", Status: "running"},
			{Name: "Redis", Status: "stopped"},
		}, nil
	})
	service := newTestService(t, &fakeRepository{}, provider)

	response, err := service.SystemStatus(context.Background())
	if err != nil {
		t.Fatalf("SystemStatus() error = %v", err)
	}
	if len(response.Alerts) != 1 || response.Alerts[0].Severity != "error" || response.Alerts[0].Title != "Redis已停止" {
		t.Fatalf("response = %#v", response)
	}
	if response.Alerts[0].ID != "service-redis-stopped" {
		t.Fatalf("alerts = %#v", response.Alerts)
	}
}

func newTestService(t *testing.T, repo *fakeRepository, provider StatusProvider) *Service {
	t.Helper()
	service, err := NewService(repo, provider)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 5, 3, 15, 4, 5, 0, time.UTC) }
	return service
}

type fakeRepository struct {
	overview    OverviewSnapshot
	growth      GrowthSnapshot
	recentUsers []RecentUser
	lastToday   time.Time
}

func (r *fakeRepository) OverviewSnapshot(_ context.Context, today time.Time, _, _ time.Time) (OverviewSnapshot, error) {
	r.lastToday = today
	return r.overview, nil
}

func (r *fakeRepository) UserGrowthSnapshot(context.Context, time.Time) (GrowthSnapshot, error) {
	return r.growth, nil
}

func (r *fakeRepository) RecentUsers(context.Context, int) ([]RecentUser, error) {
	return r.recentUsers, nil
}
