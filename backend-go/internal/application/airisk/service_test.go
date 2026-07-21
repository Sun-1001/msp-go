package airisk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewServiceValidatesDependencies(t *testing.T) {
	if _, err := NewService(nil, &fakeSlotStore{}); err == nil {
		t.Fatal("NewService(nil repo) error = nil")
	}
	if _, err := NewService(&fakeRepository{}, nil); err == nil {
		t.Fatal("NewService(nil slots) error = nil")
	}
}

func TestSettingsDefaultsUpdateAndValidation(t *testing.T) {
	repo := &fakeRepository{}
	service := newFakeService(t, repo, &fakeSlotStore{})

	settings, err := service.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings.DailyReplyLimit != 50 || settings.MaxConcurrentRequests != 2 || settings.ResetTimezone != ResetTimezone {
		t.Fatalf("GetSettings() = %#v", settings)
	}
	if settings.NextResetAt != "2026-07-22T00:00:00+08:00" {
		t.Fatalf("NextResetAt = %q", settings.NextResetAt)
	}

	updated, err := service.UpdateSettings(context.Background(), UpdateSettingsRequest{
		DailyReplyLimit:       80,
		MaxConcurrentRequests: 3,
		BlockedKeywords:       []string{"  代考 ", "代考", "HACK", "hack", ""},
	})
	if err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}
	if len(updated.BlockedKeywords) != 2 || updated.BlockedKeywords[0] != "代考" || updated.BlockedKeywords[1] != "HACK" {
		t.Fatalf("normalized keywords = %#v", updated.BlockedKeywords)
	}
	if len(repo.updates) != 3 || !strings.Contains(repo.updates[2].Value, "代考") {
		t.Fatalf("setting updates = %#v", repo.updates)
	}

	invalid := []UpdateSettingsRequest{
		{DailyReplyLimit: 0, MaxConcurrentRequests: 1},
		{DailyReplyLimit: 1, MaxConcurrentRequests: 21},
		{DailyReplyLimit: 1, MaxConcurrentRequests: 1, BlockedKeywords: []string{strings.Repeat("a", 65)}},
	}
	for _, request := range invalid {
		if _, err := service.UpdateSettings(context.Background(), request); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("UpdateSettings(%#v) error = %v", request, err)
		}
	}
}

func TestGetSettingsRejectsCorruptStoredValues(t *testing.T) {
	repo := &fakeRepository{settings: map[string]string{DailyReplyLimitKey: "not-a-number"}}
	service := newFakeService(t, repo, &fakeSlotStore{})
	if _, err := service.GetSettings(context.Background()); err == nil {
		t.Fatal("GetSettings() error = nil")
	}
}

func TestAdminOverviewStudentsEventsAndAccess(t *testing.T) {
	now := time.Date(2026, 7, 21, 4, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		settings:       map[string]string{DailyReplyLimitKey: "5", MaxConcurrencyKey: "2", BlockedKeywordsKey: `[]`},
		overview:       Overview{TotalStudents: 3, BlockedStudents: 1, RepliesToday: 7},
		students:       []StudentItem{{ID: "student-1", Username: "alice", RepliesUsed: 5}, {ID: "student-2", Username: "bob", RepliesUsed: 2}},
		studentTotal:   2,
		events:         []RiskEvent{{ID: "event-1", EventType: "content_blocked"}},
		eventTotal:     1,
		accessResponse: StudentAccessResponse{StudentID: "student-1", AIBlocked: true, BlockedReason: "违规"},
		accessFound:    true,
	}
	service := newFakeService(t, repo, &fakeSlotStore{})
	service.now = func() time.Time { return now }

	overview, err := service.GetOverview(context.Background())
	if err != nil || overview.DailyReplyLimit != 5 || overview.MaxConcurrentRequests != 2 || repo.overviewDate != "2026-07-21" {
		t.Fatalf("GetOverview() = %#v, %v date=%q", overview, err, repo.overviewDate)
	}
	students, err := service.ListStudents(context.Background(), StudentListFilter{Page: 1, PageSize: 20, Status: "all"})
	if err != nil {
		t.Fatalf("ListStudents() error = %v", err)
	}
	if !students.Items[0].QuotaExhausted || students.Items[0].RepliesRemaining != 0 || students.Items[1].RepliesRemaining != 3 {
		t.Fatalf("ListStudents() = %#v", students)
	}
	events, err := service.ListRiskEvents(context.Background(), EventListFilter{})
	if err != nil || events.Total != 1 || events.TotalPages != 1 {
		t.Fatalf("ListRiskEvents() = %#v, %v", events, err)
	}
	access, err := service.UpdateStudentAccess(context.Background(), "student-1", "admin-1", UpdateStudentAccessRequest{Blocked: true, Reason: "违规"})
	if err != nil || !access.AIBlocked || repo.mutation.EventDate != "2026-07-21" || repo.mutation.EventID != "id-1" {
		t.Fatalf("UpdateStudentAccess() = %#v, %v mutation=%#v", access, err, repo.mutation)
	}

	if _, err := service.ListStudents(context.Background(), StudentListFilter{Status: "unknown"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ListStudents(invalid) error = %v", err)
	}
	if _, err := service.ListRiskEvents(context.Background(), EventListFilter{EventType: "unknown"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ListRiskEvents(invalid) error = %v", err)
	}
	if _, err := service.UpdateStudentAccess(context.Background(), "student-1", "admin-1", UpdateStudentAccessRequest{Blocked: true}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UpdateStudentAccess(no reason) error = %v", err)
	}

	repo.accessFound = false
	if _, err := service.UpdateStudentAccess(context.Background(), "missing", "admin-1", UpdateStudentAccessRequest{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateStudentAccess(missing) error = %v", err)
	}
}

func TestAcquireAllowsNonStudentsAndEnforcesAccessContentQuotaAndConcurrency(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepository{
		settings:    map[string]string{DailyReplyLimitKey: "2", MaxConcurrencyKey: "1", BlockedKeywordsKey: `["代考"]`},
		access:      StudentAccess{StudentID: "teacher-1", Username: "teacher", IsStudent: false},
		accessFound: true,
	}
	slots := &fakeSlotStore{}
	service := newFakeService(t, repo, slots)

	lease, err := service.Acquire(ctx, "teacher-1", "session_chat", "代考", true)
	if err != nil {
		t.Fatalf("Acquire(teacher) error = %v", err)
	}
	if err := lease.Release(ctx); err != nil || slots.acquireCalls != 0 {
		t.Fatalf("teacher lease release=%v calls=%d", err, slots.acquireCalls)
	}

	repo.access = StudentAccess{StudentID: "student-1", Username: "alice", IsStudent: true, IsBlocked: true, BlockedReason: "人工封禁"}
	if _, err := service.Acquire(ctx, "student-1", "session_chat", "正常问题", true); !errors.Is(err, ErrAccessBlocked) || !strings.Contains(err.Error(), "人工封禁") {
		t.Fatalf("Acquire(blocked) error = %v", err)
	}

	repo.access.IsBlocked = false
	if _, err := service.Acquire(ctx, "student-1", "session_chat", "请帮我代考", true); !errors.Is(err, ErrContentBlocked) {
		t.Fatalf("Acquire(content) error = %v", err)
	}
	if len(repo.insertedEvents) != 1 || repo.insertedEvents[0].MatchedRule != "代考" || repo.insertedEvents[0].ContentHash == "" {
		t.Fatalf("risk events = %#v", repo.insertedEvents)
	}

	repo.replyCount = 2
	if _, err := service.Acquire(ctx, "student-1", "session_chat", "正常问题", true); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Acquire(quota) error = %v", err)
	}

	repo.replyCount = 1
	slots.decision = SlotDecision{Allowed: false, Reason: "concurrency"}
	if _, err := service.Acquire(ctx, "student-1", "session_chat", "正常问题", true); !errors.Is(err, ErrConcurrencyExceeded) {
		t.Fatalf("Acquire(concurrency) error = %v", err)
	}

	slots.decision = SlotDecision{Allowed: false, Reason: "quota"}
	if _, err := service.Acquire(ctx, "student-1", "session_chat", "正常问题", true); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Acquire(in-flight quota) error = %v", err)
	}

	slots.decision = SlotDecision{Allowed: true}
	lease, err = service.Acquire(ctx, "student-1", "exercise_generate", "", false)
	if err != nil {
		t.Fatalf("Acquire(operation) error = %v", err)
	}
	if slots.lastDailyLimit != 0 || slots.lastUsedToday != 0 {
		t.Fatalf("operation quota args = %d/%d", slots.lastDailyLimit, slots.lastUsedToday)
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if err := lease.Release(ctx); err != nil || slots.releaseCalls != 1 {
		t.Fatalf("second Release() = %v calls=%d", err, slots.releaseCalls)
	}
}

func TestAcquireFailsClosedWhenDependenciesFail(t *testing.T) {
	repo := &fakeRepository{accessErr: errors.New("db down")}
	service := newFakeService(t, repo, &fakeSlotStore{})
	if _, err := service.Acquire(context.Background(), "student-1", "chat", "hello", true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Acquire(db failure) error = %v", err)
	}

	repo.accessErr = nil
	repo.accessFound = true
	repo.access = StudentAccess{StudentID: "student-1", Username: "alice", IsStudent: true}
	repo.settings = map[string]string{DailyReplyLimitKey: "2", MaxConcurrencyKey: "1", BlockedKeywordsKey: `[]`}
	slots := &fakeSlotStore{err: errors.New("redis down")}
	service = newFakeService(t, repo, slots)
	if _, err := service.Acquire(context.Background(), "student-1", "chat", "hello", true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Acquire(redis failure) error = %v", err)
	}
}

func TestContentExcerptRemovesControlsAndBoundsRunes(t *testing.T) {
	got := contentExcerpt(" 你好\x00世界测试 ", 4)
	if got != "你好世界" {
		t.Fatalf("contentExcerpt() = %q", got)
	}
	if gotDate := UsageDate(time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC)); gotDate != "2026-07-21" {
		t.Fatalf("UsageDate() = %q", gotDate)
	}
}

func newFakeService(t *testing.T, repo Repository, slots SlotStore) *Service {
	t.Helper()
	service, err := NewService(repo, slots)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 7, 21, 4, 0, 0, 0, time.UTC) }
	ids := 0
	service.newID = func() (string, error) {
		ids++
		return "id-" + string(rune('0'+ids)), nil
	}
	return service
}

type fakeRepository struct {
	settings       map[string]string
	settingsErr    error
	updates        []SettingUpdate
	access         StudentAccess
	accessFound    bool
	accessErr      error
	replyCount     int
	replyErr       error
	overview       Overview
	overviewDate   string
	students       []StudentItem
	studentTotal   int
	studentFilter  StudentListFilter
	accessResponse StudentAccessResponse
	mutation       StudentAccessMutation
	events         []RiskEvent
	eventTotal     int
	eventFilter    EventListFilter
	insertedEvents []RiskEvent
}

func (r *fakeRepository) GetSettings(context.Context, []string) (map[string]string, error) {
	return r.settings, r.settingsErr
}

func (r *fakeRepository) UpsertSettings(_ context.Context, updates []SettingUpdate) error {
	r.updates = append([]SettingUpdate(nil), updates...)
	return nil
}

func (r *fakeRepository) GetStudentAccess(context.Context, string) (StudentAccess, bool, error) {
	return r.access, r.accessFound, r.accessErr
}

func (r *fakeRepository) CountReplies(context.Context, string, string) (int, error) {
	return r.replyCount, r.replyErr
}

func (r *fakeRepository) Overview(_ context.Context, date string, _ int) (Overview, error) {
	r.overviewDate = date
	return r.overview, nil
}

func (r *fakeRepository) ListStudents(_ context.Context, filter StudentListFilter) ([]StudentItem, int, error) {
	r.studentFilter = filter
	return append([]StudentItem(nil), r.students...), r.studentTotal, nil
}

func (r *fakeRepository) SetStudentAccess(_ context.Context, mutation StudentAccessMutation) (StudentAccessResponse, bool, error) {
	r.mutation = mutation
	return r.accessResponse, r.accessFound, nil
}

func (r *fakeRepository) InsertRiskEvent(_ context.Context, event RiskEvent) error {
	r.insertedEvents = append(r.insertedEvents, event)
	return nil
}

func (r *fakeRepository) ListRiskEvents(_ context.Context, filter EventListFilter) ([]RiskEvent, int, error) {
	r.eventFilter = filter
	return append([]RiskEvent(nil), r.events...), r.eventTotal, nil
}

type fakeSlotStore struct {
	decision       SlotDecision
	err            error
	acquireCalls   int
	releaseCalls   int
	lastDailyLimit int
	lastUsedToday  int
}

func (s *fakeSlotStore) Acquire(_ context.Context, _, _ string, _ int, dailyLimit int, usedToday int, _ time.Duration) (SlotDecision, error) {
	s.acquireCalls++
	s.lastDailyLimit = dailyLimit
	s.lastUsedToday = usedToday
	if s.err != nil {
		return SlotDecision{}, s.err
	}
	if !s.decision.Allowed && s.decision.Reason == "" {
		return SlotDecision{Allowed: true}, nil
	}
	return s.decision, nil
}

func (s *fakeSlotStore) Release(context.Context, string, string) error {
	s.releaseCalls++
	return s.err
}
