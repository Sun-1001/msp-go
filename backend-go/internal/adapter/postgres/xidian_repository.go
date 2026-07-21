package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	xidianapp "mathstudy/backend-go/internal/application/xidian"
)

// XidianRepository persists verified Xidian account bindings.
type XidianRepository struct {
	Repository
}

// NewXidianRepository creates a PostgreSQL-backed Xidian repository.
func NewXidianRepository(db Querier) (XidianRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return XidianRepository{}, err
	}
	return XidianRepository{Repository: base}, nil
}

// GetAccount loads the Xidian account bound to userID.
func (r XidianRepository) GetAccount(ctx context.Context, userID string) (xidianapp.Account, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT id, user_id, username, status, last_verified_at, created_at, updated_at
		FROM public.xidian_accounts
		WHERE user_id = $1`,
		userID,
	)
	account, err := scanXidianAccount(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return xidianapp.Account{}, false, nil
		}
		return xidianapp.Account{}, false, err
	}
	return account, true, nil
}

// UpsertAccount inserts or updates one verified Xidian account binding.
func (r XidianRepository) UpsertAccount(ctx context.Context, input xidianapp.AccountUpsert) (xidianapp.Account, error) {
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.xidian_accounts (
			id, user_id, username, status, last_verified_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'active', $4, $5, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET username = EXCLUDED.username,
			status = 'active',
			last_verified_at = EXCLUDED.last_verified_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, username, status, last_verified_at, created_at, updated_at`,
		input.ID,
		input.UserID,
		input.Username,
		input.LastVerifiedAt,
		input.Now,
	)
	return scanXidianAccount(row)
}

// DeleteAccount removes the binding for a user.
func (r XidianRepository) DeleteAccount(ctx context.Context, userID string) error {
	_, err := r.DB().Exec(ctx, `DELETE FROM public.xidian_accounts WHERE user_id = $1`, userID)
	return err
}

func scanXidianAccount(row pgx.Row) (xidianapp.Account, error) {
	var account xidianapp.Account
	if err := row.Scan(
		&account.ID,
		&account.UserID,
		&account.Username,
		&account.Status,
		&account.LastVerifiedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return xidianapp.Account{}, err
	}
	return account, nil
}
