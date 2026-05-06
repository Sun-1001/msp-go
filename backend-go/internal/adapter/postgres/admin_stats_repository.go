package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	adminstatsapp "mathstudy/backend-go/internal/application/adminstats"
)

// AdminStatsRepository persists admin dashboard read models.
type AdminStatsRepository struct {
	Repository
}

// NewAdminStatsRepository creates a PostgreSQL-backed admin stats repository.
func NewAdminStatsRepository(db Querier) (AdminStatsRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return AdminStatsRepository{}, err
	}
	return AdminStatsRepository{Repository: base}, nil
}

// OverviewSnapshot returns active account counters and weekly registration trend inputs.
func (r AdminStatsRepository) OverviewSnapshot(ctx context.Context, today time.Time, oneWeekAgo time.Time, twoWeeksAgo time.Time) (adminstatsapp.OverviewSnapshot, error) {
	var snapshot adminstatsapp.OverviewSnapshot
	if err := r.DB().QueryRow(ctx, `
		SELECT
			count(id)::int,
			coalesce(sum(CASE WHEN role = 'STUDENT'::public.userrole THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN role = 'TEACHER'::public.userrole THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN role = 'ADMIN'::public.userrole THEN 1 ELSE 0 END), 0)::int
		FROM public.users
		WHERE is_active = true`,
	).Scan(&snapshot.TotalUsers, &snapshot.StudentCount, &snapshot.TeacherCount, &snapshot.AdminCount); err != nil {
		return adminstatsapp.OverviewSnapshot{}, err
	}
	if err := r.DB().QueryRow(ctx, `
		SELECT count(DISTINCT student_id)::int
		FROM public.learning_sessions
		WHERE started_at >= $1`,
		today,
	).Scan(&snapshot.ActiveUsersToday); err != nil {
		return adminstatsapp.OverviewSnapshot{}, err
	}
	if err := r.DB().QueryRow(ctx, `
		SELECT
			coalesce(sum(CASE WHEN created_at >= $1 THEN 1 ELSE 0 END), 0)::int,
			coalesce(sum(CASE WHEN created_at >= $2 AND created_at < $1 THEN 1 ELSE 0 END), 0)::int
		FROM public.users
		WHERE created_at >= $2 AND is_active = true`,
		oneWeekAgo,
		twoWeeksAgo,
	).Scan(&snapshot.ThisWeekUsers, &snapshot.LastWeekUsers); err != nil {
		return adminstatsapp.OverviewSnapshot{}, err
	}
	return snapshot, nil
}

// UserGrowthSnapshot returns base cumulative counts and daily role counts since start.
func (r AdminStatsRepository) UserGrowthSnapshot(ctx context.Context, start time.Time) (adminstatsapp.GrowthSnapshot, error) {
	var snapshot adminstatsapp.GrowthSnapshot
	rows, err := r.DB().Query(ctx, `
		SELECT role::text, count(id)::int
		FROM public.users
		WHERE created_at < $1 AND is_active = true
		GROUP BY role`,
		start,
	)
	if err != nil {
		return adminstatsapp.GrowthSnapshot{}, err
	}
	for rows.Next() {
		var role string
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			rows.Close()
			return adminstatsapp.GrowthSnapshot{}, err
		}
		snapshot.Base.Total += count
		switch role {
		case "STUDENT":
			snapshot.Base.Students += count
		case "TEACHER":
			snapshot.Base.Teachers += count
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return adminstatsapp.GrowthSnapshot{}, err
	}
	rows.Close()

	rows, err = r.DB().Query(ctx, `
		SELECT to_char(created_at::date, 'YYYY-MM-DD'), role::text, count(id)::int
		FROM public.users
		WHERE created_at >= $1 AND is_active = true
		GROUP BY created_at::date, role
		ORDER BY created_at::date`,
		start,
	)
	if err != nil {
		return adminstatsapp.GrowthSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row adminstatsapp.DailyRoleCount
		if err := rows.Scan(&row.Date, &row.Role, &row.Count); err != nil {
			return adminstatsapp.GrowthSnapshot{}, err
		}
		snapshot.Daily = append(snapshot.Daily, row)
	}
	return snapshot, rows.Err()
}

// RecentUsers returns recently created active users.
func (r AdminStatsRepository) RecentUsers(ctx context.Context, limit int) ([]adminstatsapp.RecentUser, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT id, username, display_name, role::text, created_at
		FROM public.users
		WHERE is_active = true
		ORDER BY created_at DESC, id DESC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []adminstatsapp.RecentUser{}
	for rows.Next() {
		var account adminstatsapp.RecentUser
		var displayName pgtype.Text
		if err := rows.Scan(&account.ID, &account.Username, &displayName, &account.Role, &account.CreatedAt); err != nil {
			return nil, err
		}
		if displayName.Valid {
			value := displayName.String
			account.DisplayName = &value
		}
		users = append(users, account)
	}
	return users, rows.Err()
}
