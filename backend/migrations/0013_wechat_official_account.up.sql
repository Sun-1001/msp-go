-- 0013: WeChat Official Account bindings, message reminders, and message-center time semantics.

-- Subscription rows may exist before a user completes the one-time binding flow.
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
    ON public.wechat_user_bindings USING btree (app_id, user_id)
    WHERE user_id IS NOT NULL;

-- Reminder jobs store semantic event references, never business content or provider credentials.
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
    ON public.wechat_message_reminder_jobs
        (app_id, next_attempt_at, created_at, id)
    WHERE status = 'pending';

CREATE INDEX ix_wechat_message_reminder_expired_lease
    ON public.wechat_message_reminder_jobs
        (app_id, lease_expires_at, id)
    WHERE status = 'processing';

CREATE INDEX ix_wechat_message_reminder_finished_retention
    ON public.wechat_message_reminder_jobs
        (app_id, finished_at, id)
    WHERE status IN ('sent', 'skipped', 'dead');

CREATE INDEX ix_wechat_message_reminder_recipient
    ON public.wechat_message_reminder_jobs (recipient_user_id);

-- Message-center business timestamps are Beijing wall time. Reminder scheduling remains UTC.
ALTER TABLE public.conversations
    ALTER COLUMN last_message_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'),
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'),
    ALTER COLUMN updated_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.conversation_messages
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.notices
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.notice_confirmations
    ALTER COLUMN confirmed_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.notice_recipients
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.question_threads
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai'),
    ALTER COLUMN updated_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');

ALTER TABLE public.question_thread_messages
    ALTER COLUMN created_at SET DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai');
