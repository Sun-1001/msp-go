-- Keep only assistance availability, never chat content. Session deletion must
-- not erase evidence used to interpret a submitted answer.
CREATE TABLE public.exercise_hint_evidence (
    student_id varchar(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    exercise_id varchar(36) NOT NULL,
    status varchar(24) NOT NULL CHECK (status IN ('hinted', 'unknown')),
    observed_at timestamp NOT NULL,
    PRIMARY KEY (student_id, exercise_id)
);

-- Old tutor sessions cannot establish when a streamed hint first became
-- available. Preserve that uncertainty instead of inferring no assistance.
INSERT INTO public.exercise_hint_evidence (student_id, exercise_id, status, observed_at)
SELECT DISTINCT student_id, exercise_id, 'unknown', CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
FROM public.session_exercise_tutors;

ALTER TABLE public.content_attempts
    ADD COLUMN assistance_status varchar(24) NOT NULL DEFAULT 'unknown'
    CHECK (assistance_status IN ('unknown', 'hinted', 'no_recorded_hint'));
