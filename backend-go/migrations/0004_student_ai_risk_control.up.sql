-- Student AI risk control, uniform per-student limits, and durable reply usage.

INSERT INTO public.system_settings (key, value, description, updated_at)
VALUES
    ('student_ai_daily_reply_limit', '50', '每名学生每日可获得的 AI 成功回复数', now()),
    ('student_ai_max_concurrency', '2', '每名学生可同时执行的 AI 请求数', now()),
    ('student_ai_blocked_keywords', '["制作炸弹","获取他人密码","入侵学校系统","自杀方法","代考","build a bomb","steal a password","hack the school system"]', '学生 AI 请求拦截关键词 JSON 数组', now())
ON CONFLICT (key) DO NOTHING;

CREATE TABLE public.student_ai_access_controls (
    student_id character varying(36) PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    is_blocked boolean DEFAULT false NOT NULL,
    blocked_reason character varying(500),
    blocked_at timestamp without time zone,
    blocked_by character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.student_ai_reply_usage (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    session_id character varying(36) REFERENCES public.learning_sessions(id) ON DELETE SET NULL,
    message_id character varying(36) NOT NULL,
    usage_date date NOT NULL,
    created_at timestamp without time zone NOT NULL,
    CONSTRAINT uq_student_ai_reply_usage_message UNIQUE (message_id)
);

CREATE TABLE public.student_ai_risk_events (
    id character varying(36) PRIMARY KEY,
    student_id character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    student_username character varying(50) DEFAULT ''::character varying NOT NULL,
    event_type character varying(40) NOT NULL,
    severity character varying(16) NOT NULL,
    action character varying(32) NOT NULL,
    source character varying(64) DEFAULT ''::character varying NOT NULL,
    matched_rule character varying(100) DEFAULT ''::character varying NOT NULL,
    content_excerpt character varying(500) DEFAULT ''::character varying NOT NULL,
    content_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    actor_id character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    event_date date NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_student_ai_risk_event_type CHECK (event_type IN ('content_blocked', 'admin_blocked', 'admin_unblocked')),
    CONSTRAINT ck_student_ai_risk_severity CHECK (severity IN ('info', 'warning', 'critical'))
);

CREATE INDEX ix_student_ai_reply_usage_student_date
    ON public.student_ai_reply_usage (student_id, usage_date);
CREATE INDEX ix_student_ai_reply_usage_date
    ON public.student_ai_reply_usage (usage_date);
CREATE INDEX ix_student_ai_risk_events_created
    ON public.student_ai_risk_events (created_at DESC);
CREATE INDEX ix_student_ai_risk_events_student_created
    ON public.student_ai_risk_events (student_id, created_at DESC);
CREATE INDEX ix_student_ai_risk_events_date_type
    ON public.student_ai_risk_events (event_date, event_type);
