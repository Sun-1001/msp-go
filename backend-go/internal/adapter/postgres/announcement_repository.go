package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	announcementapp "mathstudy/backend-go/internal/application/announcement"
	"mathstudy/backend-go/internal/domain/user"
)

const announcementColumns = `
	id, title, content, content_format, audience, is_append, is_persistent,
	is_active, revision, published_at, created_by, created_at, updated_at`

// AnnouncementRepository persists system announcements and account dismissals.
type AnnouncementRepository struct {
	Repository
}

// NewAnnouncementRepository creates a PostgreSQL-backed announcement repository.
func NewAnnouncementRepository(db Querier) (AnnouncementRepository, error) {
	base, err := NewRepository(db)
	if err != nil {
		return AnnouncementRepository{}, err
	}
	return AnnouncementRepository{Repository: base}, nil
}

// ListAnnouncementsForAdmin returns all announcements for management.
func (r AnnouncementRepository) ListAnnouncementsForAdmin(ctx context.Context) ([]announcementapp.Announcement, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT `+announcementColumns+`
		FROM public.system_announcements
		ORDER BY is_active DESC, published_at DESC, id DESC
		LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

// ListAnnouncementsForUser returns current announcements for a role and omits current dismissals.
func (r AnnouncementRepository) ListAnnouncementsForUser(ctx context.Context, userID string, role user.Role) ([]announcementapp.Announcement, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT
			a.id, a.title, a.content, a.content_format, a.audience, a.is_append,
			a.is_persistent, a.is_active, a.revision, a.published_at, a.created_by,
			a.created_at, a.updated_at
		FROM public.system_announcements AS a
		LEFT JOIN public.announcement_dismissals AS dismissal
			ON dismissal.announcement_id = a.id
			AND dismissal.user_id = $1
		WHERE a.is_active = true
			AND (a.audience = $2 OR a.audience = 'all')
			AND (
				a.is_persistent = true
				OR dismissal.dismissed_revision IS NULL
				OR dismissal.dismissed_revision <> a.revision
			)
		ORDER BY a.published_at DESC, a.id DESC
		LIMIT 100`,
		userID,
		string(role),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAnnouncements(rows)
}

// CreateAnnouncement inserts an announcement and optionally replaces active rows in the same audience.
func (r AnnouncementRepository) CreateAnnouncement(ctx context.Context, params announcementapp.CreateParams) (announcementapp.Announcement, error) {
	var created announcementapp.Announcement
	err := withRepositoryTx(ctx, "announcement create", r.Repository, func(base Repository) AnnouncementRepository {
		return AnnouncementRepository{Repository: base}
	}, func(current AnnouncementRepository) error {
		if err := current.lockWrites(ctx); err != nil {
			return err
		}
		if params.IsActive && !params.Append {
			if err := current.deactivateAudience(ctx, params.Audience, "", params.Now); err != nil {
				return err
			}
		}
		row := current.DB().QueryRow(ctx, `
			INSERT INTO public.system_announcements (
				id, title, content, content_format, audience, is_append, is_persistent,
				is_active, revision, published_at, created_by, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, $10, $9, $9)
			RETURNING `+announcementColumns,
			params.ID,
			params.Title,
			params.Content,
			string(params.ContentFormat),
			string(params.Audience),
			params.Append,
			params.Persistent,
			params.IsActive,
			params.Now,
			params.CreatedBy,
		)
		item, err := scanAnnouncement(row)
		if err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return announcementapp.Announcement{}, err
	}
	return created, nil
}

// UpdateAnnouncement edits an announcement, increments its revision, and optionally replaces its audience queue.
func (r AnnouncementRepository) UpdateAnnouncement(ctx context.Context, params announcementapp.UpdateParams) (announcementapp.Announcement, bool, error) {
	var updated announcementapp.Announcement
	found := false
	err := withRepositoryTx(ctx, "announcement update", r.Repository, func(base Repository) AnnouncementRepository {
		return AnnouncementRepository{Repository: base}
	}, func(current AnnouncementRepository) error {
		if err := current.lockWrites(ctx); err != nil {
			return err
		}
		var currentActive bool
		if err := current.DB().QueryRow(ctx, `
			SELECT is_active
			FROM public.system_announcements
			WHERE id = $1
			FOR UPDATE`, params.ID).Scan(&currentActive); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = true
		effectiveActive := currentActive
		if params.IsActive != nil {
			effectiveActive = *params.IsActive
		}
		if effectiveActive && !params.Append {
			if err := current.deactivateAudience(ctx, params.Audience, params.ID, params.Now); err != nil {
				return err
			}
		}
		row := current.DB().QueryRow(ctx, `
			UPDATE public.system_announcements
			SET title = $2,
				content = $3,
				content_format = $4,
				audience = $5,
				is_append = $6,
				is_persistent = $7,
				is_active = $8,
				revision = revision + 1,
				published_at = $9,
				updated_at = $9
			WHERE id = $1
			RETURNING `+announcementColumns,
			params.ID,
			params.Title,
			params.Content,
			string(params.ContentFormat),
			string(params.Audience),
			params.Append,
			params.Persistent,
			effectiveActive,
			params.Now,
		)
		item, err := scanAnnouncement(row)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return announcementapp.Announcement{}, false, err
	}
	return updated, found, nil
}

// DeleteAnnouncement deletes an announcement and cascades its dismissals.
func (r AnnouncementRepository) DeleteAnnouncement(ctx context.Context, announcementID string) (bool, error) {
	tag, err := r.DB().Exec(ctx, `DELETE FROM public.system_announcements WHERE id = $1`, announcementID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DismissAnnouncement records the current revision for a visible, non-persistent announcement.
func (r AnnouncementRepository) DismissAnnouncement(ctx context.Context, announcementID string, userID string, role user.Role, dismissedAt time.Time) (announcementapp.DismissResult, error) {
	var result announcementapp.DismissResult
	err := withRepositoryTx(ctx, "announcement dismissal", r.Repository, func(base Repository) AnnouncementRepository {
		return AnnouncementRepository{Repository: base}
	}, func(current AnnouncementRepository) error {
		var revision int
		if err := current.DB().QueryRow(ctx, `
			SELECT is_persistent, revision
			FROM public.system_announcements
			WHERE id = $1
				AND is_active = true
				AND (audience = $2 OR audience = 'all')
			FOR SHARE`,
			announcementID,
			string(role),
		).Scan(&result.Persistent, &revision); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		result.Found = true
		if result.Persistent {
			return nil
		}
		_, err := current.DB().Exec(ctx, `
			INSERT INTO public.announcement_dismissals (
				announcement_id, user_id, dismissed_revision, dismissed_at
			)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (announcement_id, user_id) DO UPDATE
			SET dismissed_revision = EXCLUDED.dismissed_revision,
				dismissed_at = EXCLUDED.dismissed_at`,
			announcementID,
			userID,
			revision,
			dismissedAt,
		)
		return err
	})
	if err != nil {
		return announcementapp.DismissResult{}, err
	}
	return result, nil
}

func (r AnnouncementRepository) deactivateAudience(ctx context.Context, audience announcementapp.Audience, exceptID string, now time.Time) error {
	_, err := r.DB().Exec(ctx, `
		UPDATE public.system_announcements
		SET is_active = false,
			updated_at = $3
		WHERE is_active = true
			AND audience = $1
			AND ($2 = '' OR id <> $2)`,
		string(audience),
		exceptID,
		now,
	)
	return err
}

func (r AnnouncementRepository) lockWrites(ctx context.Context) error {
	_, err := r.DB().Exec(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended('system-announcements', 0))`,
	)
	return err
}

func scanAnnouncements(rows pgx.Rows) ([]announcementapp.Announcement, error) {
	items := []announcementapp.Announcement{}
	for rows.Next() {
		item, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanAnnouncement(row rowScanner) (announcementapp.Announcement, error) {
	var item announcementapp.Announcement
	var format string
	var audience string
	var createdBy pgtype.Text
	if err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Content,
		&format,
		&audience,
		&item.Append,
		&item.Persistent,
		&item.IsActive,
		&item.Revision,
		&item.PublishedAt,
		&createdBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return announcementapp.Announcement{}, err
	}
	item.ContentFormat = announcementapp.ContentFormat(format)
	item.Audience = announcementapp.Audience(audience)
	item.CreatedBy = textPtr(createdBy)
	return item, nil
}
