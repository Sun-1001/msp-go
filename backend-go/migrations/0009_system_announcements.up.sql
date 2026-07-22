CREATE TABLE IF NOT EXISTS public.system_announcements (
    id character varying(36) PRIMARY KEY,
    title character varying(120) NOT NULL,
    content text NOT NULL,
    content_format character varying(16) NOT NULL,
    audience character varying(16) NOT NULL,
    is_append boolean NOT NULL DEFAULT false,
    is_persistent boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    revision integer NOT NULL DEFAULT 1,
    published_at timestamp without time zone NOT NULL,
    created_by character varying(36),
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    CONSTRAINT ck_system_announcements_title
        CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    CONSTRAINT ck_system_announcements_content
        CHECK (char_length(content) BETWEEN 1 AND 50000 AND char_length(btrim(content)) > 0),
    CONSTRAINT ck_system_announcements_content_format
        CHECK (content_format IN ('markdown', 'html')),
    CONSTRAINT ck_system_announcements_audience
        CHECK (audience IN ('student', 'teacher', 'all')),
    CONSTRAINT ck_system_announcements_revision
        CHECK (revision >= 1),
    CONSTRAINT fk_system_announcements_created_by
        FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS public.announcement_dismissals (
    announcement_id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    dismissed_revision integer NOT NULL,
    dismissed_at timestamp without time zone NOT NULL,
    PRIMARY KEY (announcement_id, user_id),
    CONSTRAINT ck_announcement_dismissals_revision
        CHECK (dismissed_revision >= 1),
    CONSTRAINT fk_announcement_dismissals_announcement
        FOREIGN KEY (announcement_id) REFERENCES public.system_announcements(id) ON DELETE CASCADE,
    CONSTRAINT fk_announcement_dismissals_user
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_system_announcements_active_audience_published
    ON public.system_announcements (audience, published_at DESC, id DESC)
    WHERE is_active;

CREATE INDEX IF NOT EXISTS ix_system_announcements_admin_list
    ON public.system_announcements (is_active DESC, published_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS ix_announcement_dismissals_user
    ON public.announcement_dismissals (user_id, announcement_id);
