package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	admininboxapp "mathstudy/backend-go/internal/application/admininbox"
)

// ListPasswordResetRequests returns a filtered password reset request page and pending counter.
func (r UserRepository) ListPasswordResetRequests(ctx context.Context, filter admininboxapp.ListFilter) ([]admininboxapp.RequestItem, int, int, error) {
	where, args := passwordResetWhereClause(filter)
	var total int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.password_reset_requests
		WHERE `+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, 0, err
	}

	var pendingCount int
	var err error
	if pendingCount, err = r.CountPendingPasswordResetRequests(ctx); err != nil {
		return nil, 0, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))
	rows, err := r.DB().Query(ctx, `
		SELECT id, user_id, username, email, reason, status::text, created_at, reviewed_at
		FROM public.password_reset_requests
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder,
		args...,
	)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	items := []admininboxapp.RequestItem{}
	for rows.Next() {
		item, err := scanPasswordResetRequest(rows)
		if err != nil {
			return nil, 0, 0, err
		}
		items = append(items, item)
	}
	return items, total, pendingCount, rows.Err()
}

// CountPendingPasswordResetRequests returns the sidebar badge count.
func (r UserRepository) CountPendingPasswordResetRequests(ctx context.Context) (int, error) {
	var count int
	if err := r.DB().QueryRow(ctx, `
		SELECT count(id)::int
		FROM public.password_reset_requests
		WHERE status = 'pending'::public.passwordresetstatus`,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ReviewPasswordResetRequest applies an approve/reject decision in one transaction.
func (r UserRepository) ReviewPasswordResetRequest(ctx context.Context, update admininboxapp.ReviewUpdate) (admininboxapp.ReviewResult, error) {
	var result admininboxapp.ReviewResult
	err := r.withTx(ctx, func(tx UserRepository) error {
		var userID string
		var status string
		err := tx.DB().QueryRow(ctx, `
			SELECT user_id, username, status::text
			FROM public.password_reset_requests
			WHERE id = $1
			FOR UPDATE`,
			update.RequestID,
		).Scan(&userID, &result.Username, &status)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				result.Found = false
				return nil
			}
			return err
		}
		result.Found = true
		if status != string(admininboxapp.StatusPending) {
			result.AlreadyProcessed = true
			return nil
		}

		switch update.Action {
		case "approve":
			if update.PasswordHash == nil {
				return errors.New("approve password reset without password hash")
			}
			tag, err := tx.DB().Exec(ctx, `
				UPDATE public.users
				SET hashed_password = $2,
					updated_at = $3
				WHERE id = $1`,
				userID,
				*update.PasswordHash,
				update.ReviewedAt,
			)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				result.UserFound = false
				return nil
			}
			result.UserFound = true
			_, err = tx.DB().Exec(ctx, `
				UPDATE public.password_reset_requests
				SET status = 'approved'::public.passwordresetstatus,
					reviewed_by = $2,
					reviewed_at = $3,
					reject_reason = NULL
				WHERE id = $1`,
				update.RequestID,
				update.AdminID,
				update.ReviewedAt,
			)
			return err
		case "reject":
			result.UserFound = true
			_, err = tx.DB().Exec(ctx, `
				UPDATE public.password_reset_requests
				SET status = 'rejected'::public.passwordresetstatus,
					reviewed_by = $2,
					reviewed_at = $3,
					reject_reason = $4
				WHERE id = $1`,
				update.RequestID,
				update.AdminID,
				update.ReviewedAt,
				update.RejectReason,
			)
			return err
		default:
			return fmt.Errorf("unknown password reset review action %q", update.Action)
		}
	})
	if err != nil {
		return admininboxapp.ReviewResult{}, err
	}
	return result, nil
}

func passwordResetWhereClause(filter admininboxapp.ListFilter) (string, []any) {
	conditions := []string{"true"}
	args := []any{}
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d::public.passwordresetstatus", len(args)))
	}
	return strings.Join(conditions, " AND "), args
}

func scanPasswordResetRequest(row rowScanner) (admininboxapp.RequestItem, error) {
	var item admininboxapp.RequestItem
	var status string
	var reviewedAt pgtype.Timestamp
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Username,
		&item.Email,
		&item.Reason,
		&status,
		&item.CreatedAt,
		&reviewedAt,
	); err != nil {
		return admininboxapp.RequestItem{}, err
	}
	item.Status = admininboxapp.Status(status)
	if reviewedAt.Valid {
		value := reviewedAt.Time
		item.ReviewedAt = &value
	}
	return item, nil
}
