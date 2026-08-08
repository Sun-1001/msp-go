-- Persist AI chat mode independently from the user-facing session topic.

ALTER TABLE public.learning_sessions
    ADD COLUMN mode character varying(16) DEFAULT 'chat' NOT NULL;

-- Older mode changes stored their label in current_topic. Recover only exact
-- legacy labels; all other sessions retain the safe chat default.
UPDATE public.learning_sessions
SET mode = CASE current_topic
    WHEN '学习模式' THEN 'study'
    WHEN '练习模式' THEN 'practice'
    WHEN '讲解模式' THEN 'explain'
    WHEN '聊天模式' THEN 'chat'
    ELSE mode
END
WHERE current_topic IN ('学习模式', '练习模式', '讲解模式', '聊天模式');

ALTER TABLE public.learning_sessions
    ADD CONSTRAINT ck_learning_sessions_mode
    CHECK (mode IN ('study', 'chat', 'practice', 'explain'));
