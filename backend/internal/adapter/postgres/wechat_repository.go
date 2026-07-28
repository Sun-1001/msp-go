package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	wechatapp "mathstudy/backend/internal/application/wechat"
)

const (
	wechatOpenIDConstraint = "uq_wechat_user_bindings_app_open"
	wechatUserIDConstraint = "uq_wechat_user_bindings_app_user"
)

// WechatRepository persists Official Account subscriptions and user bindings.
type WechatRepository struct {
	Repository
}

// NewWechatRepository creates a PostgreSQL-backed WeChat repository.
func NewWechatRepository(db Querier) (WechatRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return WechatRepository{}, err
	}
	return WechatRepository{Repository: base}, nil
}

// GetByUserID loads the Official Account identity bound to one platform user.
func (r WechatRepository) GetByUserID(ctx context.Context, appID, userID string) (wechatapp.Binding, bool, error) {
	row := r.DB().QueryRow(ctx, `
		SELECT user_id, open_id, subscribed, bound_at
		FROM public.wechat_user_bindings
		WHERE app_id = $1 AND user_id = $2`,
		appID,
		userID,
	)
	binding, err := scanWechatBinding(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return wechatapp.Binding{}, false, nil
		}
		return wechatapp.Binding{}, false, err
	}
	return binding, true, nil
}

// Bind associates an openid with one user without allowing either identity to be reassigned.
func (r WechatRepository) Bind(ctx context.Context, appID, openID, userID string, observedAt, processedAt time.Time) (wechatapp.Binding, error) {
	id, err := newUUID()
	if err != nil {
		return wechatapp.Binding{}, err
	}
	row := r.DB().QueryRow(ctx, `
		INSERT INTO public.wechat_user_bindings AS binding (
			id, app_id, open_id, user_id, subscribed, subscribed_at,
			unsubscribed_at, bound_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, true, $5, NULL, $6, $6, $6)
		ON CONFLICT (app_id, open_id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
			subscribed = CASE
				WHEN EXCLUDED.subscribed_at > GREATEST(
					COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
					COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
				) OR (
					EXCLUDED.subscribed_at = GREATEST(
						COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
						COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
					)
					AND binding.subscribed
				) THEN true
				ELSE binding.subscribed
			END,
			subscribed_at = CASE
				WHEN EXCLUDED.subscribed_at > GREATEST(
					COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
					COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
				) OR (
					EXCLUDED.subscribed_at = GREATEST(
						COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
						COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
					)
					AND binding.subscribed
				) THEN EXCLUDED.subscribed_at
				ELSE binding.subscribed_at
			END,
			unsubscribed_at = CASE
				WHEN EXCLUDED.subscribed_at > GREATEST(
					COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
					COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
				) OR (
					EXCLUDED.subscribed_at = GREATEST(
						COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
						COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
					)
					AND binding.subscribed
				) THEN NULL
				ELSE binding.unsubscribed_at
			END,
			bound_at = CASE
				WHEN binding.user_id = EXCLUDED.user_id THEN COALESCE(binding.bound_at, EXCLUDED.bound_at)
				ELSE EXCLUDED.bound_at
			END,
			updated_at = EXCLUDED.updated_at
		WHERE binding.user_id IS NULL OR binding.user_id = EXCLUDED.user_id
		RETURNING user_id, open_id, subscribed, bound_at`,
		id,
		appID,
		openID,
		userID,
		observedAt,
		processedAt,
	)
	binding, err := scanWechatBinding(row)
	if err != nil {
		return wechatapp.Binding{}, normalizeWechatBindError(err)
	}
	return binding, nil
}

// SetSubscription records subscribe and unsubscribe callbacks even before binding.
func (r WechatRepository) SetSubscription(ctx context.Context, appID, openID string, subscribed bool, observedAt, processedAt time.Time) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	_, err = r.DB().Exec(ctx, `
		INSERT INTO public.wechat_user_bindings AS binding (
			id, app_id, open_id, subscribed, subscribed_at, unsubscribed_at,
			created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4,
			CASE WHEN $4 THEN $5::timestamp without time zone ELSE NULL END,
			CASE WHEN $4 THEN NULL ELSE $5::timestamp without time zone END,
			$6, $6
		)
		ON CONFLICT (app_id, open_id) DO UPDATE
			SET subscribed = EXCLUDED.subscribed,
				subscribed_at = CASE
					WHEN EXCLUDED.subscribed THEN EXCLUDED.subscribed_at
					ELSE binding.subscribed_at
				END,
				unsubscribed_at = CASE
					WHEN EXCLUDED.subscribed THEN NULL
					ELSE EXCLUDED.unsubscribed_at
			END,
			updated_at = EXCLUDED.updated_at
		WHERE COALESCE(EXCLUDED.subscribed_at, EXCLUDED.unsubscribed_at) > GREATEST(
			COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
			COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
		) OR (
			COALESCE(EXCLUDED.subscribed_at, EXCLUDED.unsubscribed_at) = GREATEST(
				COALESCE(binding.subscribed_at, '-infinity'::timestamp without time zone),
				COALESCE(binding.unsubscribed_at, '-infinity'::timestamp without time zone)
			)
			AND (NOT EXCLUDED.subscribed OR binding.subscribed)
		)`,
		id,
		appID,
		openID,
		subscribed,
		observedAt,
		processedAt,
	)
	return err
}

// Unbind removes the platform association while retaining subscription history.
func (r WechatRepository) Unbind(ctx context.Context, appID, userID string, now time.Time) error {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.wechat_user_bindings
		SET user_id = NULL,
			bound_at = NULL,
			updated_at = $3
		WHERE app_id = $1 AND user_id = $2`,
		appID,
		userID,
		now,
	)
	return err
}

func scanWechatBinding(row pgx.Row) (wechatapp.Binding, error) {
	var binding wechatapp.Binding
	var userID pgtype.Text
	var boundAt pgtype.Timestamp
	if err := row.Scan(&userID, &binding.OpenID, &binding.Subscribed, &boundAt); err != nil {
		return wechatapp.Binding{}, err
	}
	if userID.Valid {
		binding.UserID = userID.String
	}
	binding.BoundAt = timestampPtr(boundAt)
	return binding, nil
}

func normalizeWechatBindError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return wechatapp.ErrOpenIDAlreadyBound
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	switch pgErr.ConstraintName {
	case wechatOpenIDConstraint:
		return wechatapp.ErrOpenIDAlreadyBound
	case wechatUserIDConstraint:
		return wechatapp.ErrUserAlreadyBound
	default:
		return err
	}
}
