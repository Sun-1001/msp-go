-- WeChat delivery jobs and administrator-managed email templates.

CREATE TABLE public.wechat_user_bindings (
    id character varying(36) PRIMARY KEY,
    app_id character varying(64) NOT NULL,
    open_id character varying(128) NOT NULL,
    user_id character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    subscribed boolean DEFAULT false NOT NULL,
    subscribed_at timestamp without time zone,
    unsubscribed_at timestamp without time zone,
    bound_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT uq_wechat_user_bindings_app_open UNIQUE (app_id, open_id)
);

CREATE UNIQUE INDEX uq_wechat_user_bindings_app_user
    ON public.wechat_user_bindings (app_id, user_id)
    WHERE user_id IS NOT NULL;

CREATE TABLE public.wechat_message_reminder_jobs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    app_id character varying(64) NOT NULL,
    event_type character varying(32) NOT NULL,
    source_id character varying(36) NOT NULL,
    recipient_user_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    status character varying(16) DEFAULT 'pending' NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    lease_owner character varying(64),
    lease_expires_at timestamp without time zone,
    last_error_code character varying(64),
    provider_error_code integer,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC') NOT NULL,
    finished_at timestamp without time zone,
    CONSTRAINT ck_wechat_message_reminder_event_type
        CHECK (event_type IN ('private_message', 'notice', 'qa_message')),
    CONSTRAINT ck_wechat_message_reminder_status
        CHECK (status IN ('pending', 'processing', 'sent', 'skipped', 'dead')),
    CONSTRAINT ck_wechat_message_reminder_attempt_count
        CHECK (attempt_count >= 0),
    CONSTRAINT ck_wechat_message_reminder_lease
        CHECK (
            (status = 'processing' AND lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)
            OR (status <> 'processing' AND lease_owner IS NULL AND lease_expires_at IS NULL)
        ),
    CONSTRAINT ck_wechat_message_reminder_finished
        CHECK (
            (status IN ('pending', 'processing') AND finished_at IS NULL)
            OR (status IN ('sent', 'skipped', 'dead') AND finished_at IS NOT NULL)
        ),
    CONSTRAINT uq_wechat_message_reminder_semantic_event
        UNIQUE (app_id, event_type, source_id, recipient_user_id)
);

CREATE INDEX ix_wechat_message_reminder_pending_due
    ON public.wechat_message_reminder_jobs (app_id, next_attempt_at, created_at, id)
    WHERE status = 'pending';
CREATE INDEX ix_wechat_message_reminder_expired_lease
    ON public.wechat_message_reminder_jobs (app_id, lease_expires_at, id)
    WHERE status = 'processing';
CREATE INDEX ix_wechat_message_reminder_finished_retention
    ON public.wechat_message_reminder_jobs (app_id, finished_at, id)
    WHERE status IN ('sent', 'skipped', 'dead');
CREATE INDEX ix_wechat_message_reminder_recipient
    ON public.wechat_message_reminder_jobs (recipient_user_id);

CREATE TABLE public.email_templates (
    event character varying(64) NOT NULL,
    locale character varying(16) NOT NULL,
    subject character varying(200) NOT NULL,
    html_body text NOT NULL,
    updated_by character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT pk_email_templates PRIMARY KEY (event, locale),
    CONSTRAINT ck_email_templates_event_not_blank CHECK (btrim(event) <> ''),
    CONSTRAINT ck_email_templates_locale_not_blank CHECK (btrim(locale) <> ''),
    CONSTRAINT ck_email_templates_subject_not_blank CHECK (btrim(subject) <> ''),
    CONSTRAINT ck_email_templates_html_body_not_blank CHECK (btrim(html_body) <> '')
);

CREATE INDEX ix_email_templates_updated_by
    ON public.email_templates (updated_by)
    WHERE updated_by IS NOT NULL;
