package adminstats

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

var (
	// ErrBadRequest is returned when input cannot be applied.
	ErrBadRequest = errors.New("bad admin stats request")
)

// Error wraps a domain error with a Python-compatible message.
type Error struct {
	Kind    error
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Kind
}

// Repository is the persistence surface required by admin dashboard stats.
type Repository interface {
	OverviewSnapshot(context.Context, time.Time, time.Time, time.Time) (OverviewSnapshot, error)
	UserGrowthSnapshot(context.Context, time.Time) (GrowthSnapshot, error)
	RecentUsers(context.Context, int) ([]RecentUser, error)
}

// StatusProvider supplies process dependency status for /admin/stats/system-status.
type StatusProvider interface {
	ServiceStatuses(context.Context) ([]ServiceStatus, error)
}

// StatusProviderFunc adapts a function into a StatusProvider.
type StatusProviderFunc func(context.Context) ([]ServiceStatus, error)

// ServiceStatuses calls f(ctx).
func (f StatusProviderFunc) ServiceStatuses(ctx context.Context) ([]ServiceStatus, error) {
	return f(ctx)
}

// OverviewSnapshot contains raw dashboard counters from storage.
type OverviewSnapshot struct {
	TotalUsers       int
	StudentCount     int
	TeacherCount     int
	AdminCount       int
	ActiveUsersToday int
	ThisWeekUsers    int
	LastWeekUsers    int
}

// TrendData mirrors the Python admin stats trend payload.
type TrendData struct {
	UsersChange      float64 `json:"users_change"`
	StudentsChange   float64 `json:"students_change"`
	TeachersChange   float64 `json:"teachers_change"`
	ActiveRateChange float64 `json:"active_rate_change"`
}

// OverviewStatsResponse mirrors /admin/stats/overview.
type OverviewStatsResponse struct {
	TotalUsers       int       `json:"total_users"`
	StudentCount     int       `json:"student_count"`
	TeacherCount     int       `json:"teacher_count"`
	AdminCount       int       `json:"admin_count"`
	ActiveUsersToday int       `json:"active_users_today"`
	ActiveRate       float64   `json:"active_rate"`
	Trends           TrendData `json:"trends"`
}

// GrowthCounts stores cumulative counts.
type GrowthCounts struct {
	Total    int
	Students int
	Teachers int
}

// DailyRoleCount stores daily created-user counts by role.
type DailyRoleCount struct {
	Date  string
	Role  string
	Count int
}

// GrowthSnapshot contains raw growth counters from storage.
type GrowthSnapshot struct {
	Base  GrowthCounts
	Daily []DailyRoleCount
}

// UserGrowthDataPoint mirrors one Python growth data point.
type UserGrowthDataPoint struct {
	Date     string `json:"date"`
	Total    int    `json:"total"`
	Students int    `json:"students"`
	Teachers int    `json:"teachers"`
}

// UserGrowthSummary mirrors the Python growth summary.
type UserGrowthSummary struct {
	TotalNewUsers  int     `json:"total_new_users"`
	AvgDailyGrowth float64 `json:"avg_daily_growth"`
}

// UserGrowthResponse mirrors /admin/stats/user-growth.
type UserGrowthResponse struct {
	Period  string                `json:"period"`
	Data    []UserGrowthDataPoint `json:"data"`
	Summary UserGrowthSummary     `json:"summary"`
}

// RecentUser stores minimal account data for activity rows.
type RecentUser struct {
	ID          string
	Username    string
	DisplayName *string
	Role        string
	CreatedAt   time.Time
}

// ActivityItem mirrors one recent activity item.
type ActivityItem struct {
	ID            string    `json:"id"`
	UserName      string    `json:"user_name"`
	ActionDisplay string    `json:"action_display"`
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"`
}

// RecentActivitiesResponse mirrors /admin/stats/recent-activities.
type RecentActivitiesResponse struct {
	Items []ActivityItem `json:"items"`
	Total int            `json:"total"`
}

// ServiceStatus mirrors one dependency status item.
type ServiceStatus struct {
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	LatencyMS *float64 `json:"latency_ms"`
}

// SystemAlert mirrors one system alert item.
type SystemAlert struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// SystemStatusResponse mirrors /admin/stats/system-status.
type SystemStatusResponse struct {
	Services []ServiceStatus `json:"services"`
	Alerts   []SystemAlert   `json:"alerts"`
}

// Service implements admin dashboard stats use cases.
type Service struct {
	repo           Repository
	statusProvider StatusProvider
	now            func() time.Time
}

// NewService creates an admin stats service.
func NewService(repo Repository, providers ...StatusProvider) (*Service, error) {
	if repo == nil {
		return nil, errors.New("admin stats repository is nil")
	}
	var provider StatusProvider
	if len(providers) > 0 {
		provider = providers[0]
	}
	return &Service{
		repo:           repo,
		statusProvider: provider,
		now:            func() time.Time { return time.Now().UTC() },
	}, nil
}

// OverviewStats returns active user counts and weekly trends.
func (s *Service) OverviewStats(ctx context.Context) (OverviewStatsResponse, error) {
	now := s.now()
	today := startOfDay(now)
	oneWeekAgo := now.AddDate(0, 0, -7)
	twoWeeksAgo := now.AddDate(0, 0, -14)
	snapshot, err := s.repo.OverviewSnapshot(ctx, today, oneWeekAgo, twoWeeksAgo)
	if err != nil {
		return OverviewStatsResponse{}, err
	}

	activeRate := 0.0
	if snapshot.TotalUsers > 0 {
		activeRate = float64(snapshot.ActiveUsersToday) / float64(snapshot.TotalUsers) * 100
	}
	usersChange := percentChange(snapshot.ThisWeekUsers, snapshot.LastWeekUsers)
	return OverviewStatsResponse{
		TotalUsers:       snapshot.TotalUsers,
		StudentCount:     snapshot.StudentCount,
		TeacherCount:     snapshot.TeacherCount,
		AdminCount:       snapshot.AdminCount,
		ActiveUsersToday: snapshot.ActiveUsersToday,
		ActiveRate:       round(activeRate, 1),
		Trends: TrendData{
			UsersChange:      usersChange,
			StudentsChange:   round(usersChange*0.9, 1),
			TeachersChange:   round(usersChange*0.5, 1),
			ActiveRateChange: round(usersChange*0.3, 1),
		},
	}, nil
}

// UserGrowth returns cumulative user growth for the requested period.
func (s *Service) UserGrowth(ctx context.Context, period string) (UserGrowthResponse, error) {
	days, normalized, err := normalizePeriod(period)
	if err != nil {
		return UserGrowthResponse{}, err
	}
	now := s.now()
	start := now.AddDate(0, 0, -days)
	snapshot, err := s.repo.UserGrowthSnapshot(ctx, start)
	if err != nil {
		return UserGrowthResponse{}, err
	}

	daily := map[string]GrowthCounts{}
	for _, row := range snapshot.Daily {
		counts := daily[row.Date]
		counts.Total += row.Count
		switch strings.ToUpper(row.Role) {
		case "STUDENT":
			counts.Students += row.Count
		case "TEACHER":
			counts.Teachers += row.Count
		}
		daily[row.Date] = counts
	}

	cumulative := snapshot.Base
	totalNewUsers := 0
	data := make([]UserGrowthDataPoint, 0, days+1)
	for current := startOfDay(start); !current.After(startOfDay(now)); current = current.AddDate(0, 0, 1) {
		date := current.Format("2006-01-02")
		counts := daily[date]
		cumulative.Total += counts.Total
		cumulative.Students += counts.Students
		cumulative.Teachers += counts.Teachers
		totalNewUsers += counts.Total
		data = append(data, UserGrowthDataPoint{
			Date:     date,
			Total:    cumulative.Total,
			Students: cumulative.Students,
			Teachers: cumulative.Teachers,
		})
	}

	return UserGrowthResponse{
		Period: normalized,
		Data:   data,
		Summary: UserGrowthSummary{
			TotalNewUsers:  totalNewUsers,
			AvgDailyGrowth: round(float64(totalNewUsers)/float64(days), 2),
		},
	}, nil
}

// RecentActivities returns recently created active users as dashboard activities.
func (s *Service) RecentActivities(ctx context.Context, limit int) (RecentActivitiesResponse, error) {
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 50 {
		return RecentActivitiesResponse{}, badRequest("limit 必须在 1 到 50 之间")
	}
	users, err := s.repo.RecentUsers(ctx, limit)
	if err != nil {
		return RecentActivitiesResponse{}, err
	}
	items := make([]ActivityItem, 0, len(users))
	for _, account := range users {
		activityType := "success"
		action := "创建了新账户"
		switch strings.ToUpper(account.Role) {
		case "ADMIN":
			activityType = "warning"
			action = "创建了管理员账户"
		case "TEACHER":
			activityType = "info"
			action = "注册为教师"
		}
		name := account.Username
		if account.DisplayName != nil && strings.TrimSpace(*account.DisplayName) != "" {
			name = *account.DisplayName
		}
		items = append(items, ActivityItem{
			ID:            recentActivityID(account),
			UserName:      name,
			ActionDisplay: action,
			Timestamp:     account.CreatedAt,
			Type:          activityType,
		})
	}
	return RecentActivitiesResponse{Items: items, Total: len(items)}, nil
}

// SystemStatus returns process dependency status and derived alerts.
func (s *Service) SystemStatus(ctx context.Context) (SystemStatusResponse, error) {
	services := []ServiceStatus{{Name: "应用服务", Status: "running"}}
	if s.statusProvider != nil {
		statuses, err := s.statusProvider.ServiceStatuses(ctx)
		if err != nil {
			return SystemStatusResponse{}, err
		}
		if len(statuses) > 0 {
			services = statuses
		}
	}

	alerts := make([]SystemAlert, 0)
	for _, service := range services {
		switch service.Status {
		case "stopped":
			alerts = append(alerts, SystemAlert{
				ID:          systemAlertID(service.Name, service.Status),
				Title:       service.Name + "已停止",
				Description: service.Name + "无法连接，请检查服务是否正常运行",
				Severity:    "error",
			})
		case "warning":
			alerts = append(alerts, SystemAlert{
				ID:          systemAlertID(service.Name, service.Status),
				Title:       service.Name + "状态异常",
				Description: service.Name + "可能存在配置问题或性能问题",
				Severity:    "warning",
			})
		}
	}
	if len(alerts) == 0 {
		alerts = append(alerts, SystemAlert{
			ID:          "system-ok",
			Title:       "系统运行正常",
			Description: "所有服务运行正常",
			Severity:    "info",
		})
	}
	return SystemStatusResponse{Services: services, Alerts: alerts}, nil
}

func normalizePeriod(period string) (int, string, error) {
	period = strings.TrimSpace(period)
	if period == "" {
		period = "30d"
	}
	switch period {
	case "7d":
		return 7, period, nil
	case "30d":
		return 30, period, nil
	case "90d":
		return 90, period, nil
	default:
		return 0, "", badRequest("period 必须是 7d、30d 或 90d")
	}
}

func percentChange(current int, previous int) float64 {
	if previous == 0 {
		previous = 1
	}
	return round((float64(current)-float64(previous))/float64(previous)*100, 1)
}

func startOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func recentActivityID(account RecentUser) string {
	if strings.TrimSpace(account.ID) != "" {
		return "user-" + stableIDPart(account.ID)
	}
	return "user-" + stableIDPart(account.Username) + "-" + account.CreatedAt.UTC().Format("20060102150405.000000000")
}

func systemAlertID(serviceName string, status string) string {
	return "service-" + stableIDPart(serviceName) + "-" + stableIDPart(status)
}

func stableIDPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		isAlphaNumeric := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNumeric {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	part := strings.Trim(builder.String(), "-")
	if part == "" {
		return "unknown"
	}
	if len(part) > 80 {
		return strings.TrimRight(part[:80], "-")
	}
	return part
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}
