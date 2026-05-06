package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	adminsettingsapp "mathstudy/backend-go/internal/application/adminsettings"
)

var sensitiveExportFields = map[string]bool{
	"hashed_password":    true,
	"encrypted_password": true,
	"session_cookies":    true,
}

// AdminSettingsRepository persists system settings and database management operations.
type AdminSettingsRepository struct {
	Repository
}

// NewAdminSettingsRepository creates a PostgreSQL-backed admin settings repository.
func NewAdminSettingsRepository(db Querier) (AdminSettingsRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return AdminSettingsRepository{}, err
	}
	return AdminSettingsRepository{Repository: base}, nil
}

// GetSettings returns key/value pairs for requested system settings.
func (r AdminSettingsRepository) GetSettings(ctx context.Context, keys []string) (map[string]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT key, value
		FROM public.system_settings
		WHERE key = ANY($1)`,
		keys,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, rows.Err()
}

// UpsertSettings applies system setting changes.
func (r AdminSettingsRepository) UpsertSettings(ctx context.Context, updates []adminsettingsapp.SettingUpdate) error {
	for _, update := range updates {
		_, err := r.DB().Exec(ctx, `
			INSERT INTO public.system_settings (key, value, description, updated_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (key) DO UPDATE
			SET value = EXCLUDED.value,
				description = EXCLUDED.description,
				updated_at = EXCLUDED.updated_at`,
			update.Key,
			update.Value,
			update.Description,
			update.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExportTable exports one whitelisted table, excluding sensitive fields.
func (r AdminSettingsRepository) ExportTable(ctx context.Context, table string) ([]map[string]any, error) {
	if !safeTableName(table) {
		return nil, fmt.Errorf("unsafe table name %q", table)
	}
	rows, err := r.DB().Query(ctx, "SELECT * FROM "+pgx.Identifier{"public", table}.Sanitize())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	result := []map[string]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := map[string]any{}
		for index, field := range fields {
			name := field.Name
			if sensitiveExportFields[name] {
				continue
			}
			item[name] = normalizeExportValue(values[index])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ImportRows imports rows into one whitelisted table with ON CONFLICT DO NOTHING.
func (r AdminSettingsRepository) ImportRows(ctx context.Context, table string, rows []map[string]any) (adminsettingsapp.TableImportResult, error) {
	if !safeTableName(table) {
		return adminsettingsapp.TableImportResult{}, fmt.Errorf("unsafe table name %q", table)
	}
	var result adminsettingsapp.TableImportResult
	for _, row := range rows {
		filtered := filterImportRow(row)
		if len(filtered) == 0 {
			result.Skipped++
			continue
		}
		columns := make([]string, 0, len(filtered))
		for column := range filtered {
			columns = append(columns, column)
		}
		sort.Strings(columns)
		identifiers := make([]string, 0, len(columns))
		placeholders := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns))
		for index, column := range columns {
			identifiers = append(identifiers, pgx.Identifier{column}.Sanitize())
			placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
			args = append(args, normalizeImportValue(filtered[column]))
		}
		sql := "INSERT INTO " + pgx.Identifier{"public", table}.Sanitize() +
			" (" + strings.Join(identifiers, ", ") + ") VALUES (" +
			strings.Join(placeholders, ", ") + ") ON CONFLICT DO NOTHING"
		tag, err := r.DB().Exec(ctx, sql, args...)
		if err != nil {
			result.Failed++
			continue
		}
		if tag.RowsAffected() > 0 {
			result.Imported++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

// DatabaseOverview returns PostgreSQL overview data.
func (r AdminSettingsRepository) DatabaseOverview(ctx context.Context) (adminsettingsapp.DatabaseOverview, error) {
	var overview adminsettingsapp.DatabaseOverview
	if err := r.DB().QueryRow(ctx, `SELECT current_database()`).Scan(&overview.DatabaseName); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	if err := r.DB().QueryRow(ctx, `SELECT pg_size_pretty(pg_database_size(current_database()))`).Scan(&overview.DatabaseSize); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	if err := r.DB().QueryRow(ctx, `SELECT version()`).Scan(&overview.PostgresVersion); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	if comma := strings.Index(overview.PostgresVersion, ","); comma >= 0 {
		overview.PostgresVersion = overview.PostgresVersion[:comma]
	}
	if err := r.DB().QueryRow(ctx, `SELECT (now() - pg_postmaster_start_time())::text`).Scan(&overview.Uptime); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	if err := r.DB().QueryRow(ctx, `SELECT count(*)::int FROM pg_stat_activity WHERE state = 'active'`).Scan(&overview.ActiveConnections); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	var maxConnections string
	if err := r.DB().QueryRow(ctx, `SHOW max_connections`).Scan(&maxConnections); err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	parsed, err := strconv.Atoi(maxConnections)
	if err != nil {
		return adminsettingsapp.DatabaseOverview{}, err
	}
	overview.MaxConnections = parsed
	return overview, nil
}

// TableStats returns table row count and size statistics.
func (r AdminSettingsRepository) TableStats(ctx context.Context) ([]adminsettingsapp.TableStats, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT
			relname AS table_name,
			n_live_tup::int AS row_count,
			pg_size_pretty(pg_table_size(relid)) AS table_size,
			pg_size_pretty(pg_indexes_size(relid)) AS index_size,
			pg_size_pretty(pg_total_relation_size(relid)) AS total_size
		FROM pg_stat_user_tables
		ORDER BY pg_total_relation_size(relid) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []adminsettingsapp.TableStats{}
	for rows.Next() {
		var item adminsettingsapp.TableStats
		if err := rows.Scan(&item.TableName, &item.RowCount, &item.TableSize, &item.IndexSize, &item.TotalSize); err != nil {
			return nil, err
		}
		item.DisplayName = adminsettingsapp.DisplayNameForTable(item.TableName)
		stats = append(stats, item)
	}
	return stats, rows.Err()
}

func filterImportRow(row map[string]any) map[string]any {
	filtered := map[string]any{}
	for column, value := range row {
		if sensitiveExportFields[column] || !safeColumnName(column) {
			continue
		}
		filtered[column] = value
	}
	return filtered
}

func normalizeExportValue(value any) any {
	switch typed := value.(type) {
	case []byte:
		if json.Valid(typed) {
			var decoded any
			if err := json.Unmarshal(typed, &decoded); err == nil {
				return decoded
			}
		}
		return string(typed)
	default:
		return value
	}
}

func normalizeImportValue(value any) any {
	switch typed := value.(type) {
	case map[string]any, []any:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		return string(data)
	default:
		return typed
	}
}

func safeTableName(value string) bool {
	return adminsettingsapp.DisplayNameForTable(value) != value && safeColumnName(value)
}

func safeColumnName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '_' {
				return false
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
