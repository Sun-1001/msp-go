package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	emailapp "mathstudy/backend/internal/application/email"
)

// EmailRepository persists SMTP settings and template overrides.
type EmailRepository struct {
	Repository
}

// NewEmailRepository creates a PostgreSQL-backed email repository.
func NewEmailRepository(db Querier) (EmailRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return EmailRepository{}, err
	}
	return EmailRepository{Repository: base}, nil
}

// GetSettings returns requested system setting values.
func (r EmailRepository) GetSettings(ctx context.Context, keys []string) (map[string]string, error) {
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
	values := make(map[string]string, len(keys))
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, rows.Err()
}

// SaveSettings atomically upserts SMTP settings and optionally clears the password.
func (r EmailRepository) SaveSettings(ctx context.Context, updates []emailapp.SettingUpdate, clearPassword bool) error {
	return withRepositoryTx(ctx, "email settings", r.Repository, func(base Repository) EmailRepository {
		return EmailRepository{Repository: base}
	}, func(tx EmailRepository) error {
		if clearPassword {
			if _, err := tx.DB().Exec(ctx, `DELETE FROM public.system_settings WHERE key = 'smtp_password'`); err != nil {
				return err
			}
		}
		for _, update := range updates {
			if clearPassword && update.Key == "smtp_password" {
				continue
			}
			if _, err := tx.DB().Exec(ctx, `
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
			); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListTemplateOverrides returns every administrator template override.
func (r EmailRepository) ListTemplateOverrides(ctx context.Context) ([]emailapp.TemplateOverride, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT event, locale, subject, html_body, COALESCE(updated_by, ''), updated_at
		FROM public.email_templates
		ORDER BY event, locale`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []emailapp.TemplateOverride{}
	for rows.Next() {
		var item emailapp.TemplateOverride
		if err := rows.Scan(
			&item.Event,
			&item.Locale,
			&item.Subject,
			&item.HTMLBody,
			&item.UpdatedBy,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetTemplateOverride returns one administrator template override.
func (r EmailRepository) GetTemplateOverride(ctx context.Context, event string, locale string) (emailapp.TemplateOverride, bool, error) {
	var item emailapp.TemplateOverride
	err := r.DB().QueryRow(ctx, `
		SELECT event, locale, subject, html_body, COALESCE(updated_by, ''), updated_at
		FROM public.email_templates
		WHERE event = $1 AND locale = $2`,
		event,
		locale,
	).Scan(
		&item.Event,
		&item.Locale,
		&item.Subject,
		&item.HTMLBody,
		&item.UpdatedBy,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return emailapp.TemplateOverride{}, false, nil
		}
		return emailapp.TemplateOverride{}, false, err
	}
	return item, true, nil
}

// UpsertTemplate saves one administrator template override.
func (r EmailRepository) UpsertTemplate(ctx context.Context, item emailapp.TemplateOverride) error {
	_, err := r.DB().Exec(ctx, `
		INSERT INTO public.email_templates (
			event, locale, subject, html_body, updated_by, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event, locale) DO UPDATE
		SET subject = EXCLUDED.subject,
			html_body = EXCLUDED.html_body,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at`,
		item.Event,
		item.Locale,
		item.Subject,
		item.HTMLBody,
		item.UpdatedBy,
		item.UpdatedAt,
	)
	return err
}

// DeleteTemplate removes one administrator override and restores the official template.
func (r EmailRepository) DeleteTemplate(ctx context.Context, event string, locale string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `
		DELETE FROM public.email_templates
		WHERE event = $1 AND locale = $2`,
		event,
		locale,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
