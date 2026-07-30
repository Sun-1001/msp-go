-- 0005: Daily question schema.
--
-- This forward migration extends the current 0001-0004 clean baseline.

CREATE TABLE IF NOT EXISTS public.daily_question_candidates (
    content_id character varying(36) PRIMARY KEY
        REFERENCES public.contents(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    priority integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    valid_from date,
    valid_until date,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_daily_question_candidate_priority
        CHECK (priority BETWEEN -1000 AND 1000),
    CONSTRAINT ck_daily_question_candidate_window
        CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_from <= valid_until)
);

CREATE INDEX IF NOT EXISTS ix_daily_question_candidates_teacher_active
    ON public.daily_question_candidates (teacher_id, is_active, priority DESC);

CREATE TABLE IF NOT EXISTS public.daily_question_class_settings (
    class_id character varying(36) PRIMARY KEY
        REFERENCES public.classes(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    strategy character varying(20) DEFAULT 'personalized' NOT NULL,
    auto_reminder_enabled boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_daily_question_class_strategy
        CHECK (strategy IN ('personalized', 'uniform'))
);

ALTER TABLE public.daily_question_class_settings
    ADD COLUMN IF NOT EXISTS auto_reminder_enabled boolean DEFAULT false NOT NULL;

CREATE INDEX IF NOT EXISTS ix_daily_question_class_settings_teacher
    ON public.daily_question_class_settings (teacher_id, class_id);

CREATE INDEX IF NOT EXISTS ix_daily_question_class_settings_auto_reminder
    ON public.daily_question_class_settings (class_id)
    WHERE auto_reminder_enabled = true;

CREATE TABLE IF NOT EXISTS public.daily_question_class_selections (
    class_id character varying(36) NOT NULL
        REFERENCES public.classes(id) ON DELETE CASCADE,
    assignment_date date NOT NULL,
    content_id character varying(36)
        REFERENCES public.contents(id) ON DELETE SET NULL,
    target_concept_id character varying(36)
        REFERENCES public.knowledge_nodes(id) ON DELETE SET NULL,
    source character varying(32) NOT NULL,
    selection_reason character varying(64) NOT NULL,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT pk_daily_question_class_selections
        PRIMARY KEY (class_id, assignment_date),
    CONSTRAINT ck_daily_question_class_selection_source
        CHECK (source IN ('teacher_candidate', 'teacher_bank', 'ai_generated'))
);

CREATE INDEX IF NOT EXISTS ix_daily_question_class_selections_content
    ON public.daily_question_class_selections (content_id);

CREATE TABLE IF NOT EXISTS public.daily_question_assignments (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    class_id character varying(36)
        REFERENCES public.classes(id) ON DELETE SET NULL,
    assignment_date date NOT NULL,
    content_id character varying(36)
        REFERENCES public.contents(id) ON DELETE SET NULL,
    question_title character varying(500),
    question_body text,
    question_difficulty double precision,
    question_concept_ids json,
    question_meta json,
    question_generated_by_student_id character varying(36),
    target_concept_id character varying(36)
        REFERENCES public.knowledge_nodes(id) ON DELETE SET NULL,
    source character varying(32) DEFAULT 'ai_generated' NOT NULL,
    selection_reason character varying(64) DEFAULT 'default_concept' NOT NULL,
    status character varying(20) DEFAULT 'preparing' NOT NULL,
    first_attempt_id character varying(36)
        REFERENCES public.content_attempts(id) ON DELETE SET NULL,
    corrected_attempt_id character varying(36)
        REFERENCES public.content_attempts(id) ON DELETE SET NULL,
    first_result character varying(16),
    counts_toward_streak boolean DEFAULT false NOT NULL,
    generation_token character varying(36),
    retry_count integer DEFAULT 0 NOT NULL,
    failure_code character varying(64),
    assigned_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    opened_at timestamp without time zone,
    completed_at timestamp without time zone,
    updated_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT uq_daily_question_student_date
        UNIQUE (student_id, assignment_date),
    CONSTRAINT ck_daily_question_assignment_status
        CHECK (status IN ('preparing', 'ready', 'completed', 'unavailable')),
    CONSTRAINT ck_daily_question_assignment_source
        CHECK (source IN ('teacher_candidate', 'teacher_bank', 'ai_generated')),
    CONSTRAINT ck_daily_question_assignment_first_result
        CHECK (first_result IS NULL OR first_result IN ('correct', 'incorrect')),
    CONSTRAINT ck_daily_question_assignment_retry_count
        CHECK (retry_count >= 0),
    CONSTRAINT ck_daily_question_assignment_completion
        CHECK (
            status <> 'completed'
            OR (completed_at IS NOT NULL AND first_result IS NOT NULL)
        ),
    CONSTRAINT ck_daily_question_assignment_question_snapshot
        CHECK (
            (
                question_title IS NULL
                AND question_body IS NULL
                AND question_difficulty IS NULL
                AND question_concept_ids IS NULL
                AND question_meta IS NULL
                AND question_generated_by_student_id IS NULL
            )
            OR (
                question_title IS NOT NULL
                AND question_body IS NOT NULL
                AND question_difficulty IS NOT NULL
                AND question_concept_ids IS NOT NULL
                AND question_meta IS NOT NULL
            )
        )
);

ALTER TABLE public.daily_question_assignments
    ADD COLUMN IF NOT EXISTS question_title character varying(500),
    ADD COLUMN IF NOT EXISTS question_body text,
    ADD COLUMN IF NOT EXISTS question_difficulty double precision,
    ADD COLUMN IF NOT EXISTS question_concept_ids json,
    ADD COLUMN IF NOT EXISTS question_meta json,
    ADD COLUMN IF NOT EXISTS question_generated_by_student_id character varying(36);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_assignments'::regclass
          AND conname = 'ck_daily_question_assignment_question_snapshot'
    ) THEN
        ALTER TABLE public.daily_question_assignments
            ADD CONSTRAINT ck_daily_question_assignment_question_snapshot
            CHECK (
                (
                    question_title IS NULL
                    AND question_body IS NULL
                    AND question_difficulty IS NULL
                    AND question_concept_ids IS NULL
                    AND question_meta IS NULL
                    AND question_generated_by_student_id IS NULL
                )
                OR (
                    question_title IS NOT NULL
                    AND question_body IS NOT NULL
                    AND question_difficulty IS NOT NULL
                    AND question_concept_ids IS NOT NULL
                    AND question_meta IS NOT NULL
                )
            ) NOT VALID;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS ix_daily_question_assignments_student_history
    ON public.daily_question_assignments (student_id, assignment_date DESC);

CREATE INDEX IF NOT EXISTS ix_daily_question_assignments_class_date
    ON public.daily_question_assignments (class_id, assignment_date, status);

CREATE INDEX IF NOT EXISTS ix_daily_question_assignments_content
    ON public.daily_question_assignments (content_id);

CREATE INDEX IF NOT EXISTS ix_daily_question_assignments_preparing
    ON public.daily_question_assignments (status, updated_at)
    WHERE status = 'preparing';

CREATE TABLE IF NOT EXISTS public.daily_question_reminders (
    id character varying(36) PRIMARY KEY,
    teacher_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    class_id character varying(36) NOT NULL
        REFERENCES public.classes(id) ON DELETE CASCADE,
    assignment_date date NOT NULL,
    channel character varying(20) DEFAULT 'in_app' NOT NULL,
    notice_id character varying(36)
        REFERENCES public.notices(id) ON DELETE SET NULL,
    recipient_count integer DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT uq_daily_question_reminder
        UNIQUE (class_id, assignment_date, channel),
    CONSTRAINT ck_daily_question_reminder_channel
        CHECK (channel IN ('in_app', 'wechat')),
    CONSTRAINT ck_daily_question_reminder_status
        CHECK (status IN ('pending', 'sent', 'skipped', 'failed')),
    CONSTRAINT ck_daily_question_reminder_recipient_count
        CHECK (recipient_count >= 0)
);

CREATE INDEX IF NOT EXISTS ix_daily_question_reminders_teacher
    ON public.daily_question_reminders (teacher_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.daily_question_wechat_events (
    id character varying(36) PRIMARY KEY,
    kind character varying(32) NOT NULL,
    teacher_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    class_id character varying(36) NOT NULL
        REFERENCES public.classes(id) ON DELETE CASCADE,
    assignment_date date NOT NULL,
    remaining_content_id character varying(36)
        REFERENCES public.contents(id) ON DELETE CASCADE,
    recipient_count integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone
        DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_daily_question_wechat_event_kind
        CHECK (kind IN (
            'manual_student_reminder',
            'automatic_student_reminder',
            'uniform_low_stock'
        )),
    CONSTRAINT ck_daily_question_wechat_event_recipient_count
        CHECK (recipient_count >= 0),
    CONSTRAINT ck_daily_question_wechat_event_remaining_content
        CHECK (
            (kind = 'uniform_low_stock' AND remaining_content_id IS NOT NULL)
            OR (kind <> 'uniform_low_stock' AND remaining_content_id IS NULL)
        )
);

ALTER TABLE public.daily_question_wechat_events
    ADD COLUMN IF NOT EXISTS remaining_content_id character varying(36)
        REFERENCES public.contents(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS recipient_count integer DEFAULT 0 NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_wechat_events'::regclass
          AND conname = 'ck_daily_question_wechat_event_kind'
    ) THEN
        ALTER TABLE public.daily_question_wechat_events
            ADD CONSTRAINT ck_daily_question_wechat_event_kind
            CHECK (kind IN (
                'manual_student_reminder',
                'automatic_student_reminder',
                'uniform_low_stock'
            )) NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_wechat_events'::regclass
          AND conname = 'ck_daily_question_wechat_event_recipient_count'
    ) THEN
        ALTER TABLE public.daily_question_wechat_events
            ADD CONSTRAINT ck_daily_question_wechat_event_recipient_count
            CHECK (recipient_count >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_wechat_events'::regclass
          AND conname = 'ck_daily_question_wechat_event_remaining_content'
    ) THEN
        ALTER TABLE public.daily_question_wechat_events
            ADD CONSTRAINT ck_daily_question_wechat_event_remaining_content
            CHECK (
                (kind = 'uniform_low_stock' AND remaining_content_id IS NOT NULL)
                OR (kind <> 'uniform_low_stock' AND remaining_content_id IS NULL)
            ) NOT VALID;
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_daily_question_wechat_event_student_reminder
    ON public.daily_question_wechat_events (class_id, assignment_date)
    WHERE kind IN ('manual_student_reminder', 'automatic_student_reminder');

CREATE UNIQUE INDEX IF NOT EXISTS uq_daily_question_wechat_event_low_stock
    ON public.daily_question_wechat_events (class_id, remaining_content_id)
    WHERE kind = 'uniform_low_stock';

CREATE INDEX IF NOT EXISTS ix_daily_question_wechat_events_class_date
    ON public.daily_question_wechat_events (class_id, assignment_date DESC);

DO $$
BEGIN
    IF to_regclass('public.wechat_message_reminder_jobs') IS NULL THEN
        RAISE EXCEPTION 'daily question migration requires public.wechat_message_reminder_jobs';
    END IF;

    ALTER TABLE public.wechat_message_reminder_jobs
        DROP CONSTRAINT IF EXISTS ck_wechat_message_reminder_event_type;
    ALTER TABLE public.wechat_message_reminder_jobs
        ADD CONSTRAINT ck_wechat_message_reminder_event_type
            CHECK (event_type IN ('private_message', 'notice', 'qa_message', 'daily_question'));
END
$$;
