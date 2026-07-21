package adminstats

import (
	"context"
	"errors"
	"strings"
	"time"

	"mathstudy/backend-go/internal/platform/numutil"
	"mathstudy/backend-go/internal/platform/timefmt"
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

// LearningActivityRepository optionally supplies live learning-session counts.
type LearningActivityRepository interface {
	LearningActivitySnapshot(context.Context) (LearningActivitySnapshot, error)
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

// OperationsProvider supplies in-process metrics without coupling this package
// to the platform metrics implementation.
type OperationsProvider interface {
	OperationsSnapshot() OperationsSnapshot
}

// OperationsProviderFunc adapts a function into an OperationsProvider.
type OperationsProviderFunc func() OperationsSnapshot

// OperationsSnapshot calls f().
func (f OperationsProviderFunc) OperationsSnapshot() OperationsSnapshot {
	return f()
}

// OperationsResetter starts a new admin traffic-analysis window.
type OperationsResetter interface {
	ResetTrafficMetrics() time.Time
}

// OperationsResetterFunc adapts a function into an OperationsResetter.
type OperationsResetterFunc func() time.Time

// ResetTrafficMetrics calls f().
func (f OperationsResetterFunc) ResetTrafficMetrics() time.Time {
	return f()
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

// LearningActivitySnapshot contains current unfinished learning sessions.
type LearningActivitySnapshot struct {
	OnlineUsers    int
	ActiveSessions int
}

// DatabasePoolSnapshot is the storage-neutral PostgreSQL pool view.
type DatabasePoolSnapshot struct {
	MaxConnections       int64
	TotalConnections     int64
	AcquiredConnections  int64
	IdleConnections      int64
	EmptyAcquireCount    int64
	CanceledAcquireCount int64
}

// RedisPoolSnapshot is the storage-neutral Redis pool view.
type RedisPoolSnapshot struct {
	MaxConnections   int64
	TotalConnections int64
	IdleConnections  int64
	StaleConnections int64
	Hits             uint64
	Misses           uint64
	Timeouts         uint64
	WaitCount        uint64
	Unusable         uint64
}

// OperationsSnapshot contains raw process metrics supplied by the composition root.
type OperationsSnapshot struct {
	Version           string
	Environment       string
	StartedAt         time.Time
	Uptime            time.Duration
	CPUUsagePercent   float64
	HeapUsedBytes     uint64
	HeapReservedBytes uint64
	Goroutines        int
	LogicalCPUs       int
	GOMAXPROCS        int
	GoVersion         string
	OS                string
	Arch              string
	RequestsTotal     uint64
	ClientErrorsTotal uint64
	ServerErrorsTotal uint64
	AverageLatency    time.Duration
	P95Latency        time.Duration
	P95Clamped        bool
	TrafficStartedAt  time.Time
	TrafficWindow     time.Duration
	PostgreSQL        DatabasePoolSnapshot
	Redis             RedisPoolSnapshot
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

// RuntimeStatus describes the current Go API process.
type RuntimeStatus struct {
	Version           string    `json:"version"`
	Environment       string    `json:"environment"`
	StartedAt         time.Time `json:"started_at"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	HeapUsedBytes     uint64    `json:"heap_used_bytes"`
	HeapReservedBytes uint64    `json:"heap_reserved_bytes"`
	HeapUsagePercent  float64   `json:"heap_usage_percent"`
	Goroutines        int       `json:"goroutines"`
	LogicalCPUs       int       `json:"logical_cpus"`
	GOMAXPROCS        int       `json:"gomaxprocs"`
	GoVersion         string    `json:"go_version"`
	OS                string    `json:"os"`
	Arch              string    `json:"arch"`
}

// TrafficStatus summarizes request behavior in the current admin window.
type TrafficStatus struct {
	WindowStartedAt        time.Time `json:"window_started_at"`
	WindowSeconds          int64     `json:"window_seconds"`
	RequestsTotal          uint64    `json:"requests_total"`
	AverageQPS             float64   `json:"average_qps"`
	ClientErrorsTotal      uint64    `json:"client_errors_total"`
	ClientErrorRatePercent float64   `json:"client_error_rate_percent"`
	ServerErrorsTotal      uint64    `json:"server_errors_total"`
	ServerErrorRatePercent float64   `json:"server_error_rate_percent"`
	AverageLatencyMS       float64   `json:"average_latency_ms"`
	P95LatencyMS           float64   `json:"p95_latency_ms"`
	P95Clamped             bool      `json:"p95_clamped"`
}

// ResetTrafficMetricsResponse confirms a new operations-analysis window.
type ResetTrafficMetricsResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	ResetAt time.Time `json:"reset_at"`
}

// LearningStatus describes current unfinished learning sessions.
type LearningStatus struct {
	OnlineUsers    *int `json:"online_users"`
	ActiveSessions *int `json:"active_sessions"`
}

// DatabaseStatus describes PostgreSQL connection-pool pressure.
type DatabaseStatus struct {
	MaxConnections       int64   `json:"max_connections"`
	TotalConnections     int64   `json:"total_connections"`
	AcquiredConnections  int64   `json:"acquired_connections"`
	IdleConnections      int64   `json:"idle_connections"`
	UsagePercent         float64 `json:"usage_percent"`
	EmptyAcquireCount    int64   `json:"empty_acquire_count"`
	CanceledAcquireCount int64   `json:"canceled_acquire_count"`
}

// RedisStatus describes Redis connection-pool pressure and reuse.
type RedisStatus struct {
	MaxConnections   int64   `json:"max_connections"`
	TotalConnections int64   `json:"total_connections"`
	IdleConnections  int64   `json:"idle_connections"`
	StaleConnections int64   `json:"stale_connections"`
	UsagePercent     float64 `json:"usage_percent"`
	ReusePercent     float64 `json:"reuse_percent"`
	Timeouts         uint64  `json:"timeouts"`
	WaitCount        uint64  `json:"wait_count"`
	Unusable         uint64  `json:"unusable"`
}

// SystemStatusResponse mirrors /admin/stats/system-status.
type SystemStatusResponse struct {
	Status     string          `json:"status"`
	CheckedAt  time.Time       `json:"checked_at"`
	Services   []ServiceStatus `json:"services"`
	Alerts     []SystemAlert   `json:"alerts"`
	Runtime    RuntimeStatus   `json:"runtime"`
	Traffic    TrafficStatus   `json:"traffic"`
	Learning   LearningStatus  `json:"learning"`
	PostgreSQL DatabaseStatus  `json:"postgresql"`
	Redis      RedisStatus     `json:"redis"`
}

// Service implements admin dashboard stats use cases.
type Service struct {
	repo               Repository
	statusProvider     StatusProvider
	operationsProvider OperationsProvider
	operationsResetter OperationsResetter
	now                func() time.Time
}

// SetOperationsProvider configures process and dependency metrics at startup.
func (s *Service) SetOperationsProvider(provider OperationsProvider) {
	s.operationsProvider = provider
}

// SetOperationsResetter configures the traffic-window reset action at startup.
func (s *Service) SetOperationsResetter(resetter OperationsResetter) {
	s.operationsResetter = resetter
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
	today := timefmt.StartOfDay(now)
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
		ActiveRate:       numutil.RoundPlaces(activeRate, 1),
		Trends: TrendData{
			UsersChange:      usersChange,
			StudentsChange:   numutil.RoundPlaces(usersChange*0.9, 1),
			TeachersChange:   numutil.RoundPlaces(usersChange*0.5, 1),
			ActiveRateChange: numutil.RoundPlaces(usersChange*0.3, 1),
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
	for current := timefmt.StartOfDay(start); !current.After(timefmt.StartOfDay(now)); current = current.AddDate(0, 0, 1) {
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
			AvgDailyGrowth: numutil.RoundPlaces(float64(totalNewUsers)/float64(days), 2),
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
		services = append(services, statuses...)
	}

	alerts := make([]SystemAlert, 0)
	status := "healthy"
	for _, service := range services {
		switch service.Status {
		case "stopped":
			status = "unhealthy"
			alerts = append(alerts, SystemAlert{
				ID:          systemAlertID(service.Name, service.Status),
				Title:       service.Name + "已停止",
				Description: service.Name + "无法连接，请检查服务是否正常运行",
				Severity:    "error",
			})
		case "warning":
			if status == "healthy" {
				status = "degraded"
			}
			alerts = append(alerts, SystemAlert{
				ID:          systemAlertID(service.Name, service.Status),
				Title:       service.Name + "状态异常",
				Description: service.Name + "可能存在配置问题或性能问题",
				Severity:    "warning",
			})
		}
	}

	learning := LearningStatus{}
	if activityRepo, ok := s.repo.(LearningActivityRepository); ok {
		activity, err := activityRepo.LearningActivitySnapshot(ctx)
		if err != nil {
			if status == "healthy" {
				status = "degraded"
			}
			alerts = append(alerts, SystemAlert{
				ID:          "learning-activity-unavailable",
				Title:       "在线学习统计暂不可用",
				Description: "服务仍可运行，但当前无法读取未结束的学习会话",
				Severity:    "warning",
			})
		} else {
			onlineUsers := activity.OnlineUsers
			activeSessions := activity.ActiveSessions
			learning.OnlineUsers = &onlineUsers
			learning.ActiveSessions = &activeSessions
		}
	}

	runtimeStatus := RuntimeStatus{}
	trafficStatus := TrafficStatus{}
	databaseStatus := DatabaseStatus{}
	redisStatus := RedisStatus{}
	if s.operationsProvider != nil {
		snapshot := s.operationsProvider.OperationsSnapshot()
		runtimeStatus, trafficStatus, databaseStatus, redisStatus = buildOperationsStatus(snapshot)
	}
	if len(alerts) == 0 {
		alerts = append(alerts, SystemAlert{
			ID:          "system-ok",
			Title:       "系统运行正常",
			Description: "所有服务运行正常",
			Severity:    "info",
		})
	}
	return SystemStatusResponse{
		Status:     status,
		CheckedAt:  s.now(),
		Services:   services,
		Alerts:     alerts,
		Runtime:    runtimeStatus,
		Traffic:    trafficStatus,
		Learning:   learning,
		PostgreSQL: databaseStatus,
		Redis:      redisStatus,
	}, nil
}

// ResetTrafficMetrics starts a fresh request-analysis window. It does not
// mutate users, learning sessions, logs, or any other persisted business data.
func (s *Service) ResetTrafficMetrics(context.Context) (ResetTrafficMetricsResponse, error) {
	if s.operationsResetter == nil {
		return ResetTrafficMetricsResponse{}, errors.New("operations metrics resetter is not configured")
	}
	resetAt := s.operationsResetter.ResetTrafficMetrics()
	return ResetTrafficMetricsResponse{
		Success: true,
		Message: "运维流量指标已重置，将从当前时间重新统计",
		ResetAt: resetAt,
	}, nil
}

func buildOperationsStatus(snapshot OperationsSnapshot) (RuntimeStatus, TrafficStatus, DatabaseStatus, RedisStatus) {
	windowSeconds := snapshot.TrafficWindow.Seconds()
	averageQPS := 0.0
	if windowSeconds > 0 {
		averageQPS = float64(snapshot.RequestsTotal) / windowSeconds
	}

	runtimeStatus := RuntimeStatus{
		Version:           snapshot.Version,
		Environment:       snapshot.Environment,
		StartedAt:         snapshot.StartedAt,
		UptimeSeconds:     max(snapshot.Uptime.Milliseconds()/1000, 0),
		CPUUsagePercent:   numutil.RoundPlaces(snapshot.CPUUsagePercent, 1),
		HeapUsedBytes:     snapshot.HeapUsedBytes,
		HeapReservedBytes: snapshot.HeapReservedBytes,
		HeapUsagePercent:  percentage(float64(snapshot.HeapUsedBytes), float64(snapshot.HeapReservedBytes)),
		Goroutines:        snapshot.Goroutines,
		LogicalCPUs:       snapshot.LogicalCPUs,
		GOMAXPROCS:        snapshot.GOMAXPROCS,
		GoVersion:         snapshot.GoVersion,
		OS:                snapshot.OS,
		Arch:              snapshot.Arch,
	}
	trafficStatus := TrafficStatus{
		WindowStartedAt:        snapshot.TrafficStartedAt,
		WindowSeconds:          max(snapshot.TrafficWindow.Milliseconds()/1000, 0),
		RequestsTotal:          snapshot.RequestsTotal,
		AverageQPS:             numutil.RoundPlaces(averageQPS, 2),
		ClientErrorsTotal:      snapshot.ClientErrorsTotal,
		ClientErrorRatePercent: percentage(float64(snapshot.ClientErrorsTotal), float64(snapshot.RequestsTotal)),
		ServerErrorsTotal:      snapshot.ServerErrorsTotal,
		ServerErrorRatePercent: percentage(float64(snapshot.ServerErrorsTotal), float64(snapshot.RequestsTotal)),
		AverageLatencyMS:       durationMilliseconds(snapshot.AverageLatency),
		P95LatencyMS:           durationMilliseconds(snapshot.P95Latency),
		P95Clamped:             snapshot.P95Clamped,
	}
	databaseStatus := DatabaseStatus{
		MaxConnections:       snapshot.PostgreSQL.MaxConnections,
		TotalConnections:     snapshot.PostgreSQL.TotalConnections,
		AcquiredConnections:  snapshot.PostgreSQL.AcquiredConnections,
		IdleConnections:      snapshot.PostgreSQL.IdleConnections,
		UsagePercent:         percentage(float64(snapshot.PostgreSQL.AcquiredConnections), float64(snapshot.PostgreSQL.MaxConnections)),
		EmptyAcquireCount:    snapshot.PostgreSQL.EmptyAcquireCount,
		CanceledAcquireCount: snapshot.PostgreSQL.CanceledAcquireCount,
	}
	redisRequests := snapshot.Redis.Hits + snapshot.Redis.Misses
	redisStatus := RedisStatus{
		MaxConnections:   snapshot.Redis.MaxConnections,
		TotalConnections: snapshot.Redis.TotalConnections,
		IdleConnections:  snapshot.Redis.IdleConnections,
		StaleConnections: snapshot.Redis.StaleConnections,
		UsagePercent:     percentage(float64(snapshot.Redis.TotalConnections), float64(snapshot.Redis.MaxConnections)),
		ReusePercent:     percentage(float64(snapshot.Redis.Hits), float64(redisRequests)),
		Timeouts:         snapshot.Redis.Timeouts,
		WaitCount:        snapshot.Redis.WaitCount,
		Unusable:         snapshot.Redis.Unusable,
	}
	return runtimeStatus, trafficStatus, databaseStatus, redisStatus
}

func percentage(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return numutil.RoundPlaces(value/total*100, 1)
}

func durationMilliseconds(duration time.Duration) float64 {
	return numutil.RoundPlaces(float64(duration.Microseconds())/1000, 1)
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
	return numutil.RoundPlaces((float64(current)-float64(previous))/float64(previous)*100, 1)
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
