package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	adminuserapp "mathstudy/backend-go/internal/application/adminuser"
	"mathstudy/backend-go/internal/domain/user"
)

// AccountStats returns account status counters.
func (r UserRepository) AccountStats(ctx context.Context) (adminuserapp.AccountStats, error) {
	var stats adminuserapp.AccountStats
	err := r.DB().QueryRow(ctx, `
		SELECT
			count(id)::int AS total,
			count(*) FILTER (WHERE status = 'ACTIVE'::public.userstatus)::int AS active,
			count(*) FILTER (WHERE status = 'SUSPENDED'::public.userstatus)::int AS suspended
		FROM public.users`).Scan(&stats.Total, &stats.Active, &stats.Suspended)
	if err != nil {
		return adminuserapp.AccountStats{}, err
	}
	return stats, nil
}

// ListUsers returns a filtered account page.
func (r UserRepository) ListUsers(ctx context.Context, filter adminuserapp.ListFilter) ([]adminuserapp.UserItem, int, error) {
	where, args := adminUserWhereClause(filter, false)
	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.users
		WHERE `+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))
	rows, err := r.DB().Query(ctx, `
		SELECT `+userColumns+`
		FROM public.users
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []adminuserapp.UserItem{}
	for rows.Next() {
		account, _, err := scanOptionalUser(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, adminUserItem(account))
	}
	return items, total, rows.Err()
}

// UpdateUser updates display name and optionally password.
func (r UserRepository) UpdateUser(ctx context.Context, userID string, update adminuserapp.Update, passwordHash *string, now time.Time) (user.User, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.users
		SET
			display_name = COALESCE($2, display_name),
			hashed_password = COALESCE($3, hashed_password),
			updated_at = $4
		WHERE id = $1
		RETURNING `+userColumns,
		userID,
		update.DisplayName,
		passwordHash,
		now,
	)
	return scanOptionalUser(row)
}

// UpdateUserStatus updates account status and the legacy is_active flag together.
func (r UserRepository) UpdateUserStatus(ctx context.Context, userID string, status user.Status, now time.Time) (user.User, bool, error) {
	row := r.DB().QueryRow(ctx, `
		UPDATE public.users
		SET status = $2::public.userstatus,
			is_active = $3,
			updated_at = $4
		WHERE id = $1
		RETURNING `+userColumns,
		userID,
		status.DBValue(),
		status == user.StatusActive,
		now,
	)
	return scanOptionalUser(row)
}

// DeleteUser physically deletes a user and dependent records that do not have FK cascade.
func (r UserRepository) DeleteUser(ctx context.Context, userID string) (bool, error) {
	deleted := false
	err := r.withTx(ctx, func(tx UserRepository) error {
		exists, err := tx.Exists(ctx, `SELECT EXISTS(SELECT 1 FROM public.users WHERE id = $1)`, userID)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		statements := []string{
			`DELETE FROM public.session_messages WHERE session_id IN (SELECT id FROM public.learning_sessions WHERE student_id = $1)`,
			`DELETE FROM public.learning_sessions WHERE student_id = $1`,
			`DELETE FROM public.student_profiles WHERE student_id = $1`,
			`DELETE FROM public.class_enrollments WHERE student_id = $1 OR class_id IN (SELECT id FROM public.classes WHERE teacher_id = $1)`,
			`DELETE FROM public.classes WHERE teacher_id = $1`,
			`DELETE FROM public.content_acl WHERE teacher_id = $1`,
			`DELETE FROM public.content_attempts WHERE student_id = $1 OR content_id IN (SELECT id FROM public.contents WHERE owner_teacher_id = $1 OR generated_by_student_id = $1)`,
			`DELETE FROM public.contents WHERE owner_teacher_id = $1 OR generated_by_student_id = $1`,
			`DELETE FROM public.import_jobs WHERE created_by = $1`,
			`DELETE FROM public.xidian_snapshots WHERE user_id = $1`,
			`DELETE FROM public.xidian_accounts WHERE user_id = $1`,
		}
		for _, statement := range statements {
			if _, err := tx.DB().Exec(ctx, statement, userID); err != nil {
				return err
			}
		}
		tag, err := tx.DB().Exec(ctx, `DELETE FROM public.users WHERE id = $1`, userID)
		if err != nil {
			return err
		}
		deleted = tag.RowsAffected() > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}

// ExportUsers returns non-admin accounts for CSV export.
func (r UserRepository) ExportUsers(ctx context.Context, filter adminuserapp.ListFilter) ([]adminuserapp.ExportUser, error) {
	where, args := adminUserWhereClause(filter, true)
	rows, err := r.DB().Query(ctx, `
		SELECT username, email, display_name, role::text, status::text, created_at
		FROM public.users
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []adminuserapp.ExportUser{}
	for rows.Next() {
		var row adminuserapp.ExportUser
		var displayName pgtype.Text
		var roleValue string
		var statusValue string
		if err := rows.Scan(&row.Username, &row.Email, &displayName, &roleValue, &statusValue, &row.CreatedAt); err != nil {
			return nil, err
		}
		if displayName.Valid {
			row.DisplayName = displayName.String
		}
		role, err := user.ParseRole(roleValue)
		if err != nil {
			return nil, err
		}
		status, err := user.ParseStatus(statusValue)
		if err != nil {
			return nil, err
		}
		row.Role = string(role)
		row.Status = string(status)
		users = append(users, row)
	}
	return users, rows.Err()
}

func (r UserRepository) withTx(ctx context.Context, fn func(UserRepository) error) error {
	if fn == nil {
		return errors.New("user transaction function is nil")
	}
	if r.beginner == nil {
		return fn(r)
	}
	tx, err := r.beginner.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin user transaction: %w", err)
	}
	base, err := NewRepository(tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	txRepo := UserRepository{Repository: base}
	if err := fn(txRepo); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback user transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(fmt.Errorf("commit user transaction: %w", err), fmt.Errorf("rollback user transaction: %w", rollbackErr))
		}
		return fmt.Errorf("commit user transaction: %w", err)
	}
	return nil
}

func adminUserWhereClause(filter adminuserapp.ListFilter, excludeAdmins bool) (string, []any) {
	conditions := []string{"true"}
	args := []any{}
	if excludeAdmins {
		conditions = append(conditions, "role <> 'ADMIN'::public.userrole")
	}
	if filter.Search != "" {
		args = append(args, filter.Search)
		placeholder := fmt.Sprintf("$%d", len(args))
		conditions = append(conditions, "(username ILIKE '%' || "+placeholder+" || '%' OR email ILIKE '%' || "+placeholder+" || '%' OR display_name ILIKE '%' || "+placeholder+" || '%')")
	}
	if filter.Role != "" {
		if role, err := user.ParseRole(filter.Role); err == nil {
			args = append(args, role.DBValue())
			conditions = append(conditions, fmt.Sprintf("role = $%d::public.userrole", len(args)))
		}
	}
	if filter.Status != "" {
		if status, err := user.ParseStatus(filter.Status); err == nil {
			args = append(args, status.DBValue())
			conditions = append(conditions, fmt.Sprintf("status = $%d::public.userstatus", len(args)))
		}
	}
	return strings.Join(conditions, " AND "), args
}

func adminUserItem(account user.User) adminuserapp.UserItem {
	return adminuserapp.UserItem{
		ID:          account.ID,
		Username:    account.Username,
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Role:        account.Role,
		Status:      account.Status,
		CreatedAt:   account.CreatedAt,
	}
}
