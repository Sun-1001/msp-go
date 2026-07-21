package airisk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mathstudy/backend-go/internal/platform/identifier"
	"mathstudy/backend-go/internal/platform/numutil"
)

const (
	defaultDailyReplyLimit = 50
	defaultMaxConcurrency  = 2
	defaultLeaseTTL        = 10 * time.Minute
	maxBlockedKeywords     = 100
)

var shanghaiLocation = time.FixedZone(ResetTimezone, 8*60*60)

// Service implements administrator and runtime AI risk-control use cases.
type Service struct {
	repo     Repository
	slots    SlotStore
	now      func() time.Time
	newID    func() (string, error)
	leaseTTL time.Duration
}

// NewService creates an AI risk-control service.
func NewService(repo Repository, slots SlotStore) (*Service, error) {
	if repo == nil {
		return nil, errors.New("AI risk repository is nil")
	}
	if slots == nil {
		return nil, errors.New("AI risk slot store is nil")
	}
	return &Service{
		repo:     repo,
		slots:    slots,
		now:      func() time.Time { return time.Now().UTC() },
		newID:    identifier.NewUUID,
		leaseTTL: defaultLeaseTTL,
	}, nil
}

// GetSettings returns the uniform student AI limits.
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	values, err := s.repo.GetSettings(ctx, []string{DailyReplyLimitKey, MaxConcurrencyKey, BlockedKeywordsKey})
	if err != nil {
		return Settings{}, fmt.Errorf("get AI risk settings: %w", err)
	}
	settings, err := parseSettings(values)
	if err != nil {
		return Settings{}, err
	}
	return s.withResetMetadata(settings), nil
}

// UpdateSettings validates and stores the uniform student AI limits.
func (s *Service) UpdateSettings(ctx context.Context, input UpdateSettingsRequest) (Settings, error) {
	settings, err := normalizeSettings(input)
	if err != nil {
		return Settings{}, err
	}
	keywords, err := json.Marshal(settings.BlockedKeywords)
	if err != nil {
		return Settings{}, fmt.Errorf("marshal blocked keywords: %w", err)
	}
	now := s.now()
	if err := s.repo.UpsertSettings(ctx, []SettingUpdate{
		{Key: DailyReplyLimitKey, Value: strconv.Itoa(settings.DailyReplyLimit), Description: "每名学生每日可获得的 AI 成功回复数", UpdatedAt: now},
		{Key: MaxConcurrencyKey, Value: strconv.Itoa(settings.MaxConcurrentRequests), Description: "每名学生可同时执行的 AI 请求数", UpdatedAt: now},
		{Key: BlockedKeywordsKey, Value: string(keywords), Description: "学生 AI 请求拦截关键词 JSON 数组", UpdatedAt: now},
	}); err != nil {
		return Settings{}, fmt.Errorf("update AI risk settings: %w", err)
	}
	return s.withResetMetadata(settings), nil
}

// GetOverview returns risk-center summary counters.
func (s *Service) GetOverview(ctx context.Context) (Overview, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return Overview{}, err
	}
	overview, err := s.repo.Overview(ctx, s.usageDate(s.now()), settings.DailyReplyLimit)
	if err != nil {
		return Overview{}, fmt.Errorf("get AI risk overview: %w", err)
	}
	overview.DailyReplyLimit = settings.DailyReplyLimit
	overview.MaxConcurrentRequests = settings.MaxConcurrentRequests
	return overview, nil
}

// ListStudents returns current access and daily reply usage for students.
func (s *Service) ListStudents(ctx context.Context, filter StudentListFilter) (StudentListResponse, error) {
	filter, err := normalizeStudentFilter(filter)
	if err != nil {
		return StudentListResponse{}, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return StudentListResponse{}, err
	}
	filter.UsageDate = s.usageDate(s.now())
	filter.DailyLimit = settings.DailyReplyLimit
	items, total, err := s.repo.ListStudents(ctx, filter)
	if err != nil {
		return StudentListResponse{}, fmt.Errorf("list AI risk students: %w", err)
	}
	for index := range items {
		items[index].RepliesRemaining = max(settings.DailyReplyLimit-items[index].RepliesUsed, 0)
		items[index].QuotaExhausted = items[index].RepliesUsed >= settings.DailyReplyLimit
	}
	return StudentListResponse{
		Items:      items,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: numutil.TotalPages(total, filter.PageSize),
	}, nil
}

// UpdateStudentAccess blocks or unblocks one student's AI access only.
func (s *Service) UpdateStudentAccess(ctx context.Context, studentID, actorID string, input UpdateStudentAccessRequest) (StudentAccessResponse, error) {
	studentID = strings.TrimSpace(studentID)
	actorID = strings.TrimSpace(actorID)
	input.Reason = strings.TrimSpace(input.Reason)
	if studentID == "" || len(studentID) > 36 {
		return StudentAccessResponse{}, badRequest("学生 ID 无效")
	}
	if actorID == "" || len(actorID) > 36 {
		return StudentAccessResponse{}, badRequest("管理员 ID 无效")
	}
	if input.Blocked && input.Reason == "" {
		return StudentAccessResponse{}, badRequest("封禁原因不能为空")
	}
	if utf8.RuneCountInString(input.Reason) > 500 {
		return StudentAccessResponse{}, badRequest("封禁原因不能超过 500 个字符")
	}
	eventID, err := s.newID()
	if err != nil {
		return StudentAccessResponse{}, fmt.Errorf("create AI access event ID: %w", err)
	}
	now := s.now()
	response, ok, err := s.repo.SetStudentAccess(ctx, StudentAccessMutation{
		EventID:   eventID,
		StudentID: studentID,
		ActorID:   actorID,
		Blocked:   input.Blocked,
		Reason:    input.Reason,
		EventDate: s.usageDate(now),
		Now:       now,
	})
	if err != nil {
		return StudentAccessResponse{}, fmt.Errorf("update student AI access: %w", err)
	}
	if !ok {
		return StudentAccessResponse{}, Error{Kind: ErrNotFound, Message: "学生不存在"}
	}
	return response, nil
}

// ListRiskEvents returns content and administrator action events.
func (s *Service) ListRiskEvents(ctx context.Context, filter EventListFilter) (EventListResponse, error) {
	filter, err := normalizeEventFilter(filter)
	if err != nil {
		return EventListResponse{}, err
	}
	items, total, err := s.repo.ListRiskEvents(ctx, filter)
	if err != nil {
		return EventListResponse{}, fmt.Errorf("list AI risk events: %w", err)
	}
	return EventListResponse{
		Items:      items,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: numutil.TotalPages(total, filter.PageSize),
	}, nil
}

func (s *Service) withResetMetadata(settings Settings) Settings {
	now := s.now().In(shanghaiLocation)
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, shanghaiLocation)
	settings.ResetTimezone = ResetTimezone
	settings.NextResetAt = next.Format(time.RFC3339)
	return settings
}

func (s *Service) usageDate(now time.Time) string {
	return UsageDate(now)
}

// UsageDate returns the quota day in the platform's fixed Asia/Shanghai timezone.
func UsageDate(now time.Time) string {
	return now.In(shanghaiLocation).Format("2006-01-02")
}

func parseSettings(values map[string]string) (Settings, error) {
	settings := Settings{DailyReplyLimit: defaultDailyReplyLimit, MaxConcurrentRequests: defaultMaxConcurrency, BlockedKeywords: []string{}}
	if raw, ok := values[DailyReplyLimitKey]; ok {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return Settings{}, fmt.Errorf("parse %s: %w", DailyReplyLimitKey, err)
		}
		settings.DailyReplyLimit = value
	}
	if raw, ok := values[MaxConcurrencyKey]; ok {
		value, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return Settings{}, fmt.Errorf("parse %s: %w", MaxConcurrencyKey, err)
		}
		settings.MaxConcurrentRequests = value
	}
	if raw, ok := values[BlockedKeywordsKey]; ok && strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &settings.BlockedKeywords); err != nil {
			return Settings{}, fmt.Errorf("parse %s: %w", BlockedKeywordsKey, err)
		}
	}
	return normalizeSettings(UpdateSettingsRequest{
		DailyReplyLimit:       settings.DailyReplyLimit,
		MaxConcurrentRequests: settings.MaxConcurrentRequests,
		BlockedKeywords:       settings.BlockedKeywords,
	})
}

func normalizeSettings(input UpdateSettingsRequest) (Settings, error) {
	if input.DailyReplyLimit < 1 || input.DailyReplyLimit > 10_000 {
		return Settings{}, badRequest("每日回复额度必须在 1 到 10000 之间")
	}
	if input.MaxConcurrentRequests < 1 || input.MaxConcurrentRequests > 20 {
		return Settings{}, badRequest("每生并发上限必须在 1 到 20 之间")
	}
	if len(input.BlockedKeywords) > maxBlockedKeywords {
		return Settings{}, badRequest("风险关键词不能超过 100 个")
	}
	keywords := make([]string, 0, len(input.BlockedKeywords))
	seen := map[string]struct{}{}
	for _, keyword := range input.BlockedKeywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if utf8.RuneCountInString(keyword) > 64 {
			return Settings{}, badRequest("单个风险关键词不能超过 64 个字符")
		}
		key := strings.ToLower(keyword)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keywords = append(keywords, keyword)
	}
	return Settings{
		DailyReplyLimit:       input.DailyReplyLimit,
		MaxConcurrentRequests: input.MaxConcurrentRequests,
		BlockedKeywords:       keywords,
	}, nil
}

func normalizeStudentFilter(filter StudentListFilter) (StudentListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return StudentListFilter{}, badRequest("分页参数超出范围")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "all" {
		filter.Status = ""
	}
	switch filter.Status {
	case "", "active", "blocked", "quota_exhausted":
	default:
		return StudentListFilter{}, badRequest("status 参数无效")
	}
	return filter, nil
}

func normalizeEventFilter(filter EventListFilter) (EventListFilter, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return EventListFilter{}, badRequest("分页参数超出范围")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.EventType = strings.ToLower(strings.TrimSpace(filter.EventType))
	if filter.EventType == "all" {
		filter.EventType = ""
	}
	switch filter.EventType {
	case "", "content_blocked", "admin_blocked", "admin_unblocked":
	default:
		return EventListFilter{}, badRequest("event_type 参数无效")
	}
	return filter, nil
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}

type distributedLease struct {
	store     SlotStore
	studentID string
	leaseID   string
	once      sync.Once
	err       error
}

func (l *distributedLease) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.once.Do(func() { l.err = l.store.Release(ctx, l.studentID, l.leaseID) })
	return l.err
}

type noopLease struct{}

func (noopLease) Release(context.Context) error { return nil }
