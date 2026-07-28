-- 0010: Message Center — conversations, notices, and Q&A threads
-- Adds student-teacher private messaging, class notices, and question-answer threads.

-- ---------------------------------------------------------------------------
-- 1. Conversations (私信)
-- ---------------------------------------------------------------------------
CREATE TABLE public.conversations (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    subject character varying(200) NOT NULL DEFAULT ''::character varying,
    last_message_at timestamp without time zone DEFAULT now() NOT NULL,
    is_archived boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT uq_conversation_participants UNIQUE (student_id, teacher_id)
);

CREATE INDEX ix_conversations_student_id ON public.conversations USING btree (student_id);
CREATE INDEX ix_conversations_teacher_id ON public.conversations USING btree (teacher_id);
CREATE INDEX ix_conversations_student_archived ON public.conversations USING btree (student_id, is_archived, last_message_at DESC);

CREATE TABLE public.conversation_messages (
    id character varying(36) PRIMARY KEY,
    conversation_id character varying(36) NOT NULL REFERENCES public.conversations(id) ON DELETE CASCADE,
    sender_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    sender_role character varying(20) NOT NULL,
    text text NOT NULL,
    read_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_conversation_messages_sender_role CHECK (sender_role IN ('student', 'teacher'))
);

CREATE INDEX ix_conversation_messages_conversation_id ON public.conversation_messages USING btree (conversation_id, created_at);

-- ---------------------------------------------------------------------------
-- 2. Notices (通知)
-- ---------------------------------------------------------------------------
CREATE TABLE public.notices (
    id character varying(36) PRIMARY KEY,
    teacher_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    class_id character varying(36) NOT NULL REFERENCES public.classes(id) ON DELETE CASCADE,
    title character varying(500) NOT NULL,
    body text NOT NULL DEFAULT ''::text,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);

CREATE INDEX ix_notices_teacher_id ON public.notices USING btree (teacher_id, created_at DESC);
CREATE INDEX ix_notices_class_id ON public.notices USING btree (class_id);

CREATE TABLE public.notice_confirmations (
    id character varying(36) PRIMARY KEY,
    notice_id character varying(36) NOT NULL REFERENCES public.notices(id) ON DELETE CASCADE,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    confirmed_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT uq_notice_confirmation UNIQUE (notice_id, student_id)
);

CREATE INDEX ix_notice_confirmations_notice_id ON public.notice_confirmations USING btree (notice_id);

-- ---------------------------------------------------------------------------
-- 3. Question Threads (答疑)
-- ---------------------------------------------------------------------------
CREATE TABLE public.question_threads (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    teacher_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    class_name character varying(200),
    title character varying(500) NOT NULL,
    source character varying(50) NOT NULL DEFAULT '消息中心'::character varying,
    knowledge_point character varying(200),
    resource_name character varying(200),
    context text NOT NULL DEFAULT ''::text,
    status character varying(20) NOT NULL DEFAULT '待回复'::character varying,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_question_threads_status CHECK (status IN ('待回复', '已回复', '已解决', '需跟进'))
);

CREATE INDEX ix_question_threads_student_id ON public.question_threads USING btree (student_id, updated_at DESC);
CREATE INDEX ix_question_threads_teacher_id ON public.question_threads USING btree (teacher_id, updated_at DESC);
CREATE INDEX ix_question_threads_status ON public.question_threads USING btree (status);

CREATE TABLE public.question_thread_messages (
    id character varying(36) PRIMARY KEY,
    thread_id character varying(36) NOT NULL REFERENCES public.question_threads(id) ON DELETE CASCADE,
    sender_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    sender_role character varying(20) NOT NULL,
    text text NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_question_thread_messages_sender_role CHECK (sender_role IN ('student', 'teacher'))
);

CREATE INDEX ix_question_thread_messages_thread_id ON public.question_thread_messages USING btree (thread_id, created_at);
