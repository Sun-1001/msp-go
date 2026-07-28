-- Provider-backed model moderation for student AI requests.

INSERT INTO public.system_settings (key, value, description, updated_at)
VALUES
    ('student_ai_model_review_enabled', 'false', '学生 AI 输入模型同步前置审查开关', now()),
    ('student_ai_model_review_thresholds', '{"harassment":0.98,"harassment/threatening":0.90,"hate":0.65,"hate/threatening":0.65,"illicit":0.95,"illicit/violent":0.95,"self-harm":0.65,"self-harm/intent":0.85,"self-harm/instructions":0.65,"sexual":0.65,"sexual/minors":0.65,"violence":0.95,"violence/graphic":0.95}', '学生 AI 模型审查分类阈值 JSON 对象', now())
ON CONFLICT (key) DO NOTHING;

ALTER TABLE public.student_ai_risk_events
    ADD COLUMN review_model character varying(200) DEFAULT ''::character varying NOT NULL,
    ADD COLUMN risk_score double precision,
    ADD COLUMN category_scores jsonb DEFAULT '{}'::jsonb NOT NULL,
    ADD COLUMN review_latency_ms integer,
    DROP CONSTRAINT ck_student_ai_risk_event_type,
    ADD CONSTRAINT ck_student_ai_risk_event_type
        CHECK (event_type IN ('content_blocked', 'model_blocked', 'model_review_error', 'admin_blocked', 'admin_unblocked')),
    ADD CONSTRAINT ck_student_ai_risk_score
        CHECK (risk_score IS NULL OR (risk_score >= 0 AND risk_score <= 1)),
    ADD CONSTRAINT ck_student_ai_risk_category_scores
        CHECK (jsonb_typeof(category_scores) = 'object'),
    ADD CONSTRAINT ck_student_ai_review_latency
        CHECK (review_latency_ms IS NULL OR review_latency_ms >= 0);
