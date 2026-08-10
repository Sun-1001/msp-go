package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	adminuserapp "mathstudy/backend/internal/application/adminuser"
	"mathstudy/backend/internal/domain/user"
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
			auth_version = auth_version + CASE WHEN $3 IS NULL THEN 0 ELSE 1 END,
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
			auth_version = auth_version + 1,
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
		var lockedUserID string
		err := tx.DB().QueryRow(ctx, `
			SELECT id
			FROM public.users
			WHERE id = $1
			FOR UPDATE`, userID).Scan(&lockedUserID)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		statements := []string{
			// Forum content uses restrictive author/reporter foreign keys. Remove
			// dependent moderation records before deleting authored posts/replies.
			`DELETE FROM public.forum_notifications WHERE actor_id = $1`,
			`UPDATE public.forum_posts
			 SET is_featured = false, featured_by = NULL, featured_at = NULL
			 WHERE featured_by = $1`,
			`DELETE FROM public.forum_reports
			 WHERE reporter_id = $1
			    OR (target_type = 'post' AND target_id IN (SELECT id FROM public.forum_posts WHERE author_id = $1))
			    OR (target_type = 'reply' AND target_id IN (
					SELECT reply.id
					FROM public.forum_replies reply
					JOIN public.forum_posts post ON post.id = reply.post_id
					WHERE reply.author_id = $1 OR post.author_id = $1
				))`,
			`UPDATE public.forum_posts
			 SET accepted_reply_id = NULL,
			     status = CASE WHEN status = 'resolved' THEN 'open' ELSE status END,
			     updated_at = CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'
			 WHERE accepted_reply_id IN (SELECT id FROM public.forum_replies WHERE author_id = $1)`,
			`DELETE FROM public.forum_replies WHERE author_id = $1`,
			`DELETE FROM public.forum_posts WHERE author_id = $1`,
			`DELETE FROM public.session_messages WHERE session_id IN (SELECT id FROM public.learning_sessions WHERE student_id = $1)`,
			`DELETE FROM public.learning_sessions WHERE student_id = $1`,
			`DELETE FROM public.student_profiles WHERE student_id = $1`,
			`DELETE FROM public.class_enrollments WHERE student_id = $1 OR class_id IN (SELECT id FROM public.classes WHERE teacher_id = $1)`,
			`DELETE FROM public.classes WHERE teacher_id = $1`,
			`DELETE FROM public.content_acl WHERE teacher_id = $1`,
			`DELETE FROM public.content_attempts WHERE student_id = $1 OR content_id IN (SELECT id FROM public.contents WHERE owner_teacher_id = $1 OR generated_by_student_id = $1)`,
			`DELETE FROM public.contents WHERE owner_teacher_id = $1 OR generated_by_student_id = $1`,
			`DELETE FROM public.import_jobs WHERE created_by = $1`,
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
	return withRepositoryTx(ctx, "user", r.Repository, func(base Repository) UserRepository {
		return UserRepository{Repository: base}
	}, fn)
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
