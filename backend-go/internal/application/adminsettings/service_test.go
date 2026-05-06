package adminsettings

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNewServiceRejectsNilRepository(t *testing.T) {
	if _, err := NewService(nil, "app", "version"); err == nil {
		t.Fatal("NewService(nil) error = nil, want error")
	}
}

func TestRegistrationSettingsReadsDefaultsAndUpdates(t *testing.T) {
	repo := &fakeRepository{settings: map[string]string{allowStudentRegistration: "false"}}
	service := newTestService(t, repo, nil)

	settings, err := service.RegistrationSettings(context.Background())
	if err != nil {
		t.Fatalf("RegistrationSettings() error = %v", err)
	}
	if settings.AllowStudent || !settings.AllowTeacher {
		t.Fatalf("settings = %#v", settings)
	}

	updated, err := service.UpdateRegistrationSettings(context.Background(), true, false)
	if err != nil {
		t.Fatalf("UpdateRegistrationSettings() error = %v", err)
	}
	if !updated.AllowStudent || updated.AllowTeacher || len(repo.upserts) != 2 {
		t.Fatalf("updated=%#v upserts=%#v", updated, repo.upserts)
	}
}

func TestGeneralSettingsValidatesAndUpdates(t *testing.T) {
	repo := &fakeRepository{settings: map[string]string{systemDescriptionKey: "旧描述"}}
	service := newTestService(t, repo, nil)

	settings, err := service.GeneralSettings(context.Background())
	if err != nil {
		t.Fatalf("GeneralSettings() error = %v", err)
	}
	if settings.SystemName != "App" || settings.SystemDescription != "旧描述" || settings.SystemVersion != "v1" {
		t.Fatalf("settings = %#v", settings)
	}

	updated, err := service.UpdateGeneralSettings(context.Background(), " 新系统 ", "描述")
	if err != nil {
		t.Fatalf("UpdateGeneralSettings() error = %v", err)
	}
	if updated.SystemName != "新系统" || repo.upserts[0].Value != "新系统" {
		t.Fatalf("updated=%#v upserts=%#v", updated, repo.upserts)
	}

	_, err = service.UpdateGeneralSettings(context.Background(), "", "")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("UpdateGeneralSettings(empty) error = %v, want ErrBadRequest", err)
	}
}

func TestExportDataValidatesTablesAndEncodesPayload(t *testing.T) {
	repo := &fakeRepository{exportRows: map[string][]map[string]any{
		"users": {{"id": "user-1", "username": "alice"}},
	}}
	service := newTestService(t, repo, nil)

	response, err := service.ExportData(context.Background(), []string{"users"}, "admin-1")
	if err != nil {
		t.Fatalf("ExportData() error = %v", err)
	}
	if response.Filename != "backup_20260503_120000.json" || response.TableCounts["users"] != 1 || response.TotalRecords != 1 {
		t.Fatalf("response = %#v", response)
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Content)
	if err != nil {
		t.Fatalf("decode content: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["exported_by"] != "admin-1" {
		t.Fatalf("payload = %#v", payload)
	}

	_, err = service.ExportData(context.Background(), []string{"bad_table"}, "admin-1")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ExportData(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestImportDataOrdersKnownTablesAndSkipsUnknown(t *testing.T) {
	repo := &fakeRepository{importResults: map[string]TableImportResult{
		"users":           {Imported: 1},
		"system_settings": {Skipped: 1},
	}}
	service := newTestService(t, repo, nil)
	content := []byte(`{"tables":{"system_settings":[{"key":"a"}],"unknown":[{}],"users":[{"id":"u1"}]}}`)

	response, err := service.ImportData(context.Background(), content, "admin-1")
	if err != nil {
		t.Fatalf("ImportData() error = %v", err)
	}
	if response.TotalImported != 1 || response.TotalSkipped != 1 || len(response.Errors) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if len(repo.importOrder) != 2 || repo.importOrder[0] != "users" || repo.importOrder[1] != "system_settings" {
		t.Fatalf("import order = %#v", repo.importOrder)
	}

	_, err = service.ImportData(context.Background(), []byte(`{"bad":true}`), "admin-1")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("ImportData(invalid) error = %v, want ErrBadRequest", err)
	}
}

func TestDatabaseMonitorUsesPoolStatusForHealth(t *testing.T) {
	repo := &fakeRepository{
		overview: DatabaseOverview{DatabaseName: "math_platform"},
		tables:   []TableStats{{TableName: "users", RowCount: 2}},
	}
	provider := PoolStatsProviderFunc(func() ConnectionPoolStatus {
		return ConnectionPoolStatus{PoolSize: 10, CheckedOut: 10, UsagePercent: 96}
	})
	service := newTestService(t, repo, provider)

	response, err := service.DatabaseMonitor(context.Background())
	if err != nil {
		t.Fatalf("DatabaseMonitor() error = %v", err)
	}
	if response.HealthStatus != "unhealthy" || response.ConnectionPool.UsagePercent != 96 || response.CheckedAt.IsZero() {
		t.Fatalf("response = %#v", response)
	}
}

func newTestService(t *testing.T, repo *fakeRepository, provider PoolStatsProvider) *Service {
	t.Helper()
	if repo.settings == nil {
		repo.settings = map[string]string{}
	}
	service, err := NewService(repo, "App", "v1", provider)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC) }
	return service
}

type fakeRepository struct {
	settings      map[string]string
	upserts       []SettingUpdate
	exportRows    map[string][]map[string]any
	importResults map[string]TableImportResult
	importOrder   []string
	overview      DatabaseOverview
	tables        []TableStats
}

func (r *fakeRepository) GetSettings(_ context.Context, keys []string) (map[string]string, error) {
	result := map[string]string{}
	for _, key := range keys {
		if value, ok := r.settings[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *fakeRepository) UpsertSettings(_ context.Context, updates []SettingUpdate) error {
	r.upserts = updates
	for _, update := range updates {
		r.settings[update.Key] = update.Value
	}
	return nil
}

func (r *fakeRepository) ExportTable(_ context.Context, table string) ([]map[string]any, error) {
	return r.exportRows[table], nil
}

func (r *fakeRepository) ImportRows(_ context.Context, table string, rows []map[string]any) (TableImportResult, error) {
	r.importOrder = append(r.importOrder, table)
	if result, ok := r.importResults[table]; ok {
		return result, nil
	}
	return TableImportResult{Skipped: len(rows)}, nil
}

func (r *fakeRepository) DatabaseOverview(context.Context) (DatabaseOverview, error) {
	return r.overview, nil
}

func (r *fakeRepository) TableStats(context.Context) ([]TableStats, error) {
	return r.tables, nil
}
