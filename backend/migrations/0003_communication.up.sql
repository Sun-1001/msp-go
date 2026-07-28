-- Announcements, private conversations, class notices, and Q&A threads.

CREATE TABLE public.system_announcements (
    id character varying(36) PRIMARY KEY,
    title character varying(120) NOT NULL,
    content text NOT NULL,
    content_format character varying(16) NOT NULL,
    audience character varying(16) NOT NULL,
    is_append boolean DEFAULT false NOT NULL,
    is_persistent boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    revision integer DEFAULT 1 NOT NULL,
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

CREATE TABLE public.announcement_dismissals (
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

CREATE INDEX ix_system_announcements_active_audience_published
    ON public.system_announcements (audience, published_at DESC, id DESC)
    WHERE is_active;
CREATE INDEX ix_system_announcements_admin_list
    ON public.system_announcements (is_active DESC, published_at DESC, id DESC);
CREATE INDEX ix_system_announcements_created_by
    ON public.system_announcements (created_by);
CREATE INDEX ix_announcement_dismissals_user
    ON public.announcement_dismissals (user_id, announcement_id);

CREATE TABLE public.conversations (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    subject character varying(200) DEFAULT ''::character varying NOT NULL,
    last_message_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    student_archived boolean DEFAULT false NOT NULL,
    teacher_archived boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT uq_conversation_participants UNIQUE (student_id, teacher_id)
);

CREATE INDEX ix_conversations_student_participant_archived
    ON public.conversations (student_id, student_archived, last_message_at DESC);
CREATE INDEX ix_conversations_teacher_participant_archived
    ON public.conversations (teacher_id, teacher_archived, last_message_at DESC);

CREATE TABLE public.conversation_messages (
    id character varying(36) PRIMARY KEY,
    conversation_id character varying(36) NOT NULL REFERENCES public.conversations(id) ON DELETE CASCADE,
    sender_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    sender_role character varying(20) NOT NULL,
    text text NOT NULL,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    read_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_conversation_messages_sender_role
        CHECK (sender_role IN ('student', 'teacher')),
    CONSTRAINT ck_conversation_message_attachments_array
        CHECK (jsonb_typeof(attachments) = 'array')
);

CREATE INDEX ix_conversation_messages_conversation_id
    ON public.conversation_messages (conversation_id, created_at);
CREATE INDEX ix_conversation_messages_sender_id
    ON public.conversation_messages (sender_id);

CREATE TABLE public.notices (
    id character varying(36) PRIMARY KEY,
    teacher_id character varying(36),
    class_id character varying(36),
    class_name character varying(200) NOT NULL,
    title character varying(500) NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT notices_teacher_id_fkey
        FOREIGN KEY (teacher_id) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT notices_class_id_fkey
        FOREIGN KEY (class_id) REFERENCES public.classes(id) ON DELETE SET NULL
);

CREATE INDEX ix_notices_teacher_id
    ON public.notices (teacher_id, created_at DESC);
CREATE INDEX ix_notices_class_id
    ON public.notices (class_id);

CREATE TABLE public.notice_recipients (
    notice_id character varying(36) NOT NULL REFERENCES public.notices(id) ON DELETE CASCADE,
    student_id character varying(36) NOT NULL,
    recipient_name character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT pk_notice_recipients PRIMARY KEY (notice_id, student_id)
);

CREATE INDEX ix_notice_recipients_student_notice
    ON public.notice_recipients (student_id, notice_id);

CREATE TABLE public.notice_confirmations (
    id character varying(36) PRIMARY KEY,
    notice_id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    confirmed_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT uq_notice_confirmation UNIQUE (notice_id, student_id),
    CONSTRAINT notice_confirmations_recipient_fkey
        FOREIGN KEY (notice_id, student_id)
        REFERENCES public.notice_recipients (notice_id, student_id)
        ON DELETE CASCADE
);

CREATE TABLE public.question_threads (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    class_id character varying(36) REFERENCES public.classes(id) ON DELETE SET NULL,
    class_name character varying(200),
    title character varying(500) NOT NULL,
    source character varying(50) DEFAULT '消息中心'::character varying NOT NULL,
    knowledge_point character varying(200),
    resource_name character varying(200),
    context text DEFAULT ''::text NOT NULL,
    status character varying(20) DEFAULT '待回复'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_question_threads_status
        CHECK (status IN ('待回复', '已回复', '已解决', '需跟进'))
);

CREATE INDEX ix_question_threads_student_id
    ON public.question_threads (student_id, updated_at DESC);
CREATE INDEX ix_question_threads_teacher_id
    ON public.question_threads (teacher_id, updated_at DESC);
CREATE INDEX ix_question_threads_status
    ON public.question_threads (status);
CREATE INDEX ix_question_threads_teacher_class_status_updated
    ON public.question_threads (teacher_id, class_id, status, updated_at DESC);
CREATE INDEX ix_question_threads_student_teacher_updated
    ON public.question_threads (student_id, teacher_id, updated_at DESC);

CREATE TABLE public.question_thread_messages (
    id character varying(36) PRIMARY KEY,
    thread_id character varying(36) NOT NULL REFERENCES public.question_threads(id) ON DELETE CASCADE,
    sender_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    sender_role character varying(20) NOT NULL,
    text text NOT NULL,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    read_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_question_thread_messages_sender_role
        CHECK (sender_role IN ('student', 'teacher')),
    CONSTRAINT ck_question_thread_message_attachments_array
        CHECK (jsonb_typeof(attachments) = 'array')
);

CREATE INDEX ix_question_thread_messages_thread_id
    ON public.question_thread_messages (thread_id, created_at);
CREATE INDEX ix_question_thread_messages_teacher_unread
    ON public.question_thread_messages (thread_id)
    WHERE sender_role = 'teacher' AND read_at IS NULL;
CREATE INDEX ix_question_thread_messages_sender_id
    ON public.question_thread_messages (sender_id);

CREATE FUNCTION public.snapshot_notice_class_name()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.class_id IS NOT NULL THEN
        SELECT c.name
        INTO NEW.class_name
        FROM public.classes c
        WHERE c.id = NEW.class_id;
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER trg_snapshot_notice_class_name
BEFORE INSERT OR UPDATE OF class_id ON public.notices
FOR EACH ROW
EXECUTE FUNCTION public.snapshot_notice_class_name();

CREATE FUNCTION public.snapshot_notice_recipients()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO public.notice_recipients (
        notice_id, student_id, recipient_name, created_at
    )
    SELECT NEW.id, e.student_id, COALESCE(u.display_name, u.username), NEW.created_at
    FROM public.class_enrollments e
    JOIN public.users u ON u.id = e.student_id
    WHERE e.class_id = NEW.class_id
      AND u.is_active = true
      AND u.role = 'STUDENT'::public.userrole
    ON CONFLICT (notice_id, student_id) DO NOTHING;

    RETURN NEW;
END
$$;

CREATE TRIGGER trg_snapshot_notice_recipients
AFTER INSERT ON public.notices
FOR EACH ROW
EXECUTE FUNCTION public.snapshot_notice_recipients();
