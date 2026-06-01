package adminsettings

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	allowStudentRegistration = "allow_student_registration"
	allowTeacherRegistration = "allow_teacher_registration"
	systemNameKey            = "system_name"
	systemDescriptionKey     = "system_description"
)

var (
	// ErrBadRequest is returned when input cannot be applied.
	ErrBadRequest = errors.New("bad admin settings request")
)

var exportableTables = []ExportableTableItem{
	{Name: "users", DisplayName: "用户"},
	{Name: "student_profiles", DisplayName: "学生画像"},
	{Name: "knowledge_nodes", DisplayName: "知识节点"},
	{Name: "knowledge_relations", DisplayName: "知识关系"},
	{Name: "learning_sessions", DisplayName: "学习会话"},
	{Name: "session_messages", DisplayName: "会话消息"},
	{Name: "contents", DisplayName: "内容"},
	{Name: "system_settings", DisplayName: "系统设置"},
	{Name: "classes", DisplayName: "班级"},
	{Name: "class_enrollments", DisplayName: "班级学生"},
	{Name: "security_logs", DisplayName: "安全日志"},
}

var importOrder = []string{
	"users",
	"student_profiles",
	"knowledge_nodes",
	"knowledge_relations",
	"system_settings",
	"classes",
	"class_enrollments",
	"contents",
	"learning_sessions",
	"session_messages",
	"security_logs",
}

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

// Repository is the persistence surface required by admin settings.
type Repository interface {
	GetSettings(context.Context, []string) (map[string]string, error)
	UpsertSettings(context.Context, []SettingUpdate) error
	ExportTable(context.Context, string) ([]map[string]any, error)
	ImportRows(context.Context, string, []map[string]any) (TableImportResult, error)
	DatabaseOverview(context.Context) (DatabaseOverview, error)
	TableStats(context.Context) ([]TableStats, error)
}

// PoolStatsProvider supplies connection pool status for database monitor.
type PoolStatsProvider interface {
	ConnectionPoolStatus() ConnectionPoolStatus
}

// PoolStatsProviderFunc adapts a function into a PoolStatsProvider.
type PoolStatsProviderFunc func() ConnectionPoolStatus

// ConnectionPoolStatus calls f().
func (f PoolStatsProviderFunc) ConnectionPoolStatus() ConnectionPoolStatus {
	return f()
}

// SettingUpdate stores one system setting mutation.
type SettingUpdate struct {
	Key         string
	Value       string
	Description string
	UpdatedAt   time.Time
}

// RegistrationSettingsResponse mirrors /admin/settings/registration.
type RegistrationSettingsResponse struct {
	AllowStudent bool `json:"allow_student"`
	AllowTeacher bool `json:"allow_teacher"`
}

// GeneralSettingsResponse mirrors /admin/settings/general.
type GeneralSettingsResponse struct {
	SystemName        string `json:"system_name"`
	SystemDescription string `json:"system_description"`
	SystemVersion     string `json:"system_version"`
}

// ExportableTableItem stores one exportable table descriptor.
type ExportableTableItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// ExportableTablesResponse mirrors /admin/settings/database/exportable-tables.
type ExportableTablesResponse struct {
	Tables []ExportableTableItem `json:"tables"`
}

// DataExportResponse mirrors /admin/settings/database/export.
type DataExportResponse struct {
	Filename     string         `json:"filename"`
	Content      string         `json:"content"`
	ExportedAt   time.Time      `json:"exported_at"`
	TableCounts  map[string]int `json:"table_counts"`
	TotalRecords int            `json:"total_records"`
}

// TableImportResult stores one table import outcome.
type TableImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`
}

// DataImportResponse mirrors /admin/settings/database/import.
type DataImportResponse struct {
	Success       bool                         `json:"success"`
	ImportedAt    time.Time                    `json:"imported_at"`
	TableResults  map[string]TableImportResult `json:"table_results"`
	TotalImported int                          `json:"total_imported"`
	TotalSkipped  int                          `json:"total_skipped"`
	TotalFailed   int                          `json:"total_failed"`
	Errors        []string                     `json:"errors"`
}

// ConnectionPoolStatus mirrors database pool status.
type ConnectionPoolStatus struct {
	PoolSize     int     `json:"pool_size"`
	MaxOverflow  int     `json:"max_overflow"`
	CheckedOut   int     `json:"checked_out"`
	CheckedIn    int     `json:"checked_in"`
	Overflow     int     `json:"overflow"`
	PoolTimeout  int     `json:"pool_timeout"`
	PoolRecycle  int     `json:"pool_recycle"`
	UsagePercent float64 `json:"usage_percent"`
}

// TableStats stores one database table statistic.
type TableStats struct {
	TableName   string `json:"table_name"`
	DisplayName string `json:"display_name"`
	RowCount    int    `json:"row_count"`
	TableSize   string `json:"table_size"`
	IndexSize   string `json:"index_size"`
	TotalSize   string `json:"total_size"`
}

// DatabaseOverview stores database-level monitor data.
type DatabaseOverview struct {
	DatabaseName      string `json:"database_name"`
	DatabaseSize      string `json:"database_size"`
	PostgresVersion   string `json:"postgres_version"`
	Uptime            string `json:"uptime"`
	ActiveConnections int    `json:"active_connections"`
	MaxConnections    int    `json:"max_connections"`
}

// DatabaseMonitorResponse mirrors /admin/settings/database/monitor.
type DatabaseMonitorResponse struct {
	Overview       DatabaseOverview     `json:"overview"`
	ConnectionPool ConnectionPoolStatus `json:"connection_pool"`
	Tables         []TableStats         `json:"tables"`
	HealthStatus   string               `json:"health_status"`
	CheckedAt      time.Time            `json:"checked_at"`
}

// Service implements admin settings and database management use cases.
type Service struct {
	repo        Repository
	poolStats   PoolStatsProvider
	appName     string
	appVersion  string
	now         func() time.Time
	tableLookup map[string]string
}

// NewService creates an admin settings service.
func NewService(repo Repository, appName string, appVersion string, providers ...PoolStatsProvider) (*Service, error) {
	if repo == nil {
		return nil, errors.New("admin settings repository is nil")
	}
	if strings.TrimSpace(appName) == "" {
		appName = "高等数学智能学习平台"
	}
	if strings.TrimSpace(appVersion) == "" {
		appVersion = "0.1.0"
	}
	var poolStats PoolStatsProvider
	if len(providers) > 0 {
		poolStats = providers[0]
	}
	lookup := map[string]string{}
	for _, table := range exportableTables {
		lookup[table.Name] = table.DisplayName
	}
	return &Service{
		repo:        repo,
		poolStats:   poolStats,
		appName:     appName,
		appVersion:  appVersion,
		now:         func() time.Time { return time.Now().UTC() },
		tableLookup: lookup,
	}, nil
}

// RegistrationSettings reads registration toggles with Python-compatible defaults.
func (s *Service) RegistrationSettings(ctx context.Context) (RegistrationSettingsResponse, error) {
	values, err := s.repo.GetSettings(ctx, []string{allowStudentRegistration, allowTeacherRegistration})
	if err != nil {
		return RegistrationSettingsResponse{}, err
	}
	return RegistrationSettingsResponse{
		AllowStudent: settingBool(values, allowStudentRegistration, true),
		AllowTeacher: settingBool(values, allowTeacherRegistration, false),
	}, nil
}

// UpdateRegistrationSettings updates registration toggles.
func (s *Service) UpdateRegistrationSettings(ctx context.Context, allowStudent bool, allowTeacher bool) (RegistrationSettingsResponse, error) {
	now := s.now()
	if err := s.repo.UpsertSettings(ctx, []SettingUpdate{
		{Key: allowStudentRegistration, Value: boolString(allowStudent), Description: "是否允许学生注册", UpdatedAt: now},
		{Key: allowTeacherRegistration, Value: boolString(allowTeacher), Description: "是否允许教师注册", UpdatedAt: now},
	}); err != nil {
		return RegistrationSettingsResponse{}, err
	}
	return RegistrationSettingsResponse{AllowStudent: allowStudent, AllowTeacher: allowTeacher}, nil
}

// GeneralSettings reads system display metadata.
func (s *Service) GeneralSettings(ctx context.Context) (GeneralSettingsResponse, error) {
	values, err := s.repo.GetSettings(ctx, []string{systemNameKey, systemDescriptionKey})
	if err != nil {
		return GeneralSettingsResponse{}, err
	}
	response := GeneralSettingsResponse{
		SystemName:        s.appName,
		SystemDescription: "",
		SystemVersion:     s.appVersion,
	}
	if value := strings.TrimSpace(values[systemNameKey]); value != "" {
		response.SystemName = value
	}
	if value, ok := values[systemDescriptionKey]; ok {
		response.SystemDescription = value
	}
	return response, nil
}

// UpdateGeneralSettings updates system display metadata.
func (s *Service) UpdateGeneralSettings(ctx context.Context, systemName string, systemDescription string) (GeneralSettingsResponse, error) {
	systemName = strings.TrimSpace(systemName)
	if systemName == "" || len(systemName) > 100 {
		return GeneralSettingsResponse{}, badRequest("system_name 长度必须在 1 到 100 之间")
	}
	if len(systemDescription) > 500 {
		return GeneralSettingsResponse{}, badRequest("system_description 长度不能超过 500")
	}
	now := s.now()
	if err := s.repo.UpsertSettings(ctx, []SettingUpdate{
		{Key: systemNameKey, Value: systemName, Description: "系统名称", UpdatedAt: now},
		{Key: systemDescriptionKey, Value: systemDescription, Description: "系统描述", UpdatedAt: now},
	}); err != nil {
		return GeneralSettingsResponse{}, err
	}
	return GeneralSettingsResponse{
		SystemName:        systemName,
		SystemDescription: systemDescription,
		SystemVersion:     s.appVersion,
	}, nil
}

// ExportableTables returns supported database export tables.
func (s *Service) ExportableTables(context.Context) (ExportableTablesResponse, error) {
	tables := append([]ExportableTableItem(nil), exportableTables...)
	return ExportableTablesResponse{Tables: tables}, nil
}

// ExportData exports selected tables as Base64-encoded JSON.
func (s *Service) ExportData(ctx context.Context, tables []string, adminID string) (DataExportResponse, error) {
	if len(tables) == 0 {
		return DataExportResponse{}, badRequest("至少选择一张表")
	}
	for _, table := range tables {
		if !s.isExportableTable(table) {
			return DataExportResponse{}, badRequest("不支持导出的表: " + table)
		}
	}

	exportedAt := s.now()
	tablePayloads := map[string][]map[string]any{}
	tableCounts := map[string]int{}
	total := 0
	for _, table := range tables {
		rows, err := s.repo.ExportTable(ctx, table)
		if err != nil {
			return DataExportResponse{}, fmt.Errorf("export %s: %w", table, err)
		}
		tablePayloads[table] = rows
		tableCounts[table] = len(rows)
		total += len(rows)
	}

	payload := map[string]any{
		"version":     "1.0",
		"exported_at": exportedAt.Format(time.RFC3339),
		"exported_by": adminID,
		"tables":      tablePayloads,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return DataExportResponse{}, fmt.Errorf("marshal export: %w", err)
	}
	return DataExportResponse{
		Filename:     "backup_" + exportedAt.Format("20060102_150405") + ".json",
		Content:      base64.StdEncoding.EncodeToString(data),
		ExportedAt:   exportedAt,
		TableCounts:  tableCounts,
		TotalRecords: total,
	}, nil
}

// ImportData imports a JSON backup file using ON CONFLICT DO NOTHING semantics.
func (s *Service) ImportData(ctx context.Context, content []byte, adminID string) (DataImportResponse, error) {
	_ = adminID
	var payload struct {
		Tables map[string]json.RawMessage `json:"tables"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return DataImportResponse{}, badRequest("JSON 文件解析失败: " + err.Error())
	}
	if payload.Tables == nil {
		return DataImportResponse{}, badRequest("无效的备份文件格式")
	}

	response := DataImportResponse{
		Success:      true,
		ImportedAt:   s.now(),
		TableResults: map[string]TableImportResult{},
		Errors:       []string{},
	}
	for _, table := range s.orderedImportTables(payload.Tables) {
		if !s.isExportableTable(table) {
			response.Errors = append(response.Errors, "跳过未知表: "+table)
			continue
		}
		var rows []map[string]any
		if err := json.Unmarshal(payload.Tables[table], &rows); err != nil {
			response.Errors = append(response.Errors, table+": 数据格式无效")
			response.TotalFailed++
			continue
		}
		result, err := s.repo.ImportRows(ctx, table, rows)
		if err != nil {
			return DataImportResponse{}, fmt.Errorf("import %s: %w", table, err)
		}
		response.TableResults[table] = result
		response.TotalImported += result.Imported
		response.TotalSkipped += result.Skipped
		response.TotalFailed += result.Failed
	}
	response.Success = response.TotalFailed == 0
	return response, nil
}

// DatabaseMonitor returns database overview, pool usage, and table statistics.
func (s *Service) DatabaseMonitor(ctx context.Context) (DatabaseMonitorResponse, error) {
	overview, err := s.repo.DatabaseOverview(ctx)
	if err != nil {
		return DatabaseMonitorResponse{}, err
	}
	tables, err := s.repo.TableStats(ctx)
	if err != nil {
		return DatabaseMonitorResponse{}, err
	}
	pool := ConnectionPoolStatus{}
	if s.poolStats != nil {
		pool = s.poolStats.ConnectionPoolStatus()
	}
	health := "healthy"
	if pool.UsagePercent > 90 {
		health = "degraded"
	}
	if pool.UsagePercent > 95 {
		health = "unhealthy"
	}
	return DatabaseMonitorResponse{
		Overview:       overview,
		ConnectionPool: pool,
		Tables:         tables,
		HealthStatus:   health,
		CheckedAt:      s.now(),
	}, nil
}

func (s *Service) orderedImportTables(tables map[string]json.RawMessage) []string {
	seen := map[string]bool{}
	ordered := make([]string, 0, len(tables))
	for _, table := range importOrder {
		if _, ok := tables[table]; ok {
			ordered = append(ordered, table)
			seen[table] = true
		}
	}
	remaining := make([]string, 0)
	for table := range tables {
		if !seen[table] {
			remaining = append(remaining, table)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func (s *Service) isExportableTable(table string) bool {
	_, ok := s.tableLookup[table]
	return ok
}

func settingBool(values map[string]string, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	return strings.EqualFold(value, "true")
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func DisplayNameForTable(table string) string {
	for _, item := range exportableTables {
		if item.Name == table {
			return item.DisplayName
		}
	}
	return table
}

func badRequest(message string) error {
	return Error{Kind: ErrBadRequest, Message: message}
}
