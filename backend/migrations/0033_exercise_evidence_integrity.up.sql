-- Preserve final-answer scores and raw timing reports, while separating the
-- evidence admitted to mastery and study-time statistics.
ALTER TABLE public.content_attempts
    DROP CONSTRAINT ck_content_attempts_mastery_weight,
    ADD CONSTRAINT ck_content_attempts_mastery_weight CHECK (mastery_weight >= 0 AND mastery_weight <= 1),
    ADD COLUMN evidence_kind varchar(24) NOT NULL DEFAULT 'unknown'
        CHECK (evidence_kind IN ('unknown', 'first_attempt', 'repeat_practice', 'scheduled_review')),
    ADD COLUMN process_evaluation jsonb,
    ADD COLUMN reported_time_spent_seconds integer;

-- Quarantine only demonstrably invalid historical reports. Valid legacy values
-- are retained without claiming that a server clock verified them.
UPDATE public.content_attempts
SET reported_time_spent_seconds = time_spent_seconds, time_spent_seconds = 0
WHERE time_spent_seconds < 0 OR time_spent_seconds > 7200;

ALTER TABLE public.content_attempts
    ADD CONSTRAINT ck_content_attempts_study_time CHECK (time_spent_seconds BETWEEN 0 AND 7200),
    ADD CONSTRAINT ck_content_attempts_process_evaluation CHECK (
        process_evaluation IS NULL OR (jsonb_typeof(process_evaluation) = 'object'
        AND process_evaluation->>'status' IS NOT NULL
        AND process_evaluation->>'status' IN ('not_provided', 'correct', 'incorrect', 'indeterminate')));

-- Match the feedback layer's case/whitespace normalization, including Unicode
-- White_Space characters independent of the database locale. No semantic dedupe.
CREATE FUNCTION public.learning_question_body_v1(body text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT lower(translate(body, E' \t\n\r\f\013' || U&'\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000', ''))
$$;

CREATE INDEX ix_content_attempts_student_body_evidence
ON public.content_attempts(student_id, md5(public.learning_question_body_v1(review_question_body)))
WHERE submitted_at IS NOT NULL;

CREATE TABLE public.exercise_study_sessions (
    student_id varchar(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    exercise_id varchar(36) NOT NULL REFERENCES public.contents(id) ON DELETE CASCADE,
    started_at timestamp NOT NULL,
    PRIMARY KEY (student_id, exercise_id)
);
