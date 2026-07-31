-- 0009: Daily question integrity, frozen uniform schedules, and roster history.
--
-- 0005 is already published. All later daily-question consistency changes are
-- delivered here as one forward migration.

CREATE INDEX IF NOT EXISTS ix_daily_question_assignments_first_attempt
    ON public.daily_question_assignments (first_attempt_id)
    WHERE first_attempt_id IS NOT NULL;

ALTER TABLE public.content_attempts
    ADD COLUMN IF NOT EXISTS daily_assignment_id character varying(36),
    ADD COLUMN IF NOT EXISTS daily_submission_key character varying(160),
    ADD COLUMN IF NOT EXISTS daily_submission_response json;

DO $$
BEGIN
    IF EXISTS (
        WITH attempt_references AS (
            SELECT first_attempt_id AS attempt_id, id AS assignment_id
            FROM public.daily_question_assignments
            WHERE first_attempt_id IS NOT NULL
            UNION ALL
            SELECT corrected_attempt_id, id
            FROM public.daily_question_assignments
            WHERE corrected_attempt_id IS NOT NULL
        )
        SELECT 1
        FROM attempt_references
        GROUP BY attempt_id
        HAVING count(DISTINCT assignment_id) > 1
    ) THEN
        RAISE EXCEPTION 'one content attempt is referenced by multiple daily assignments';
    END IF;

    IF EXISTS (
        WITH attempt_references AS (
            SELECT first_attempt_id AS attempt_id, id AS assignment_id
            FROM public.daily_question_assignments
            WHERE first_attempt_id IS NOT NULL
            UNION ALL
            SELECT corrected_attempt_id, id
            FROM public.daily_question_assignments
            WHERE corrected_attempt_id IS NOT NULL
        )
        SELECT 1
        FROM attempt_references reference
        JOIN public.daily_question_assignments assignment
          ON assignment.id = reference.assignment_id
        JOIN public.content_attempts attempt
          ON attempt.id = reference.attempt_id
        WHERE attempt.student_id IS DISTINCT FROM assignment.student_id
           OR attempt.content_id IS DISTINCT FROM assignment.content_id
           OR (
               attempt.daily_assignment_id IS NOT NULL
               AND attempt.daily_assignment_id IS DISTINCT FROM assignment.id
           )
    ) THEN
        RAISE EXCEPTION 'daily assignment attempt ownership is inconsistent';
    END IF;
END
$$;

UPDATE public.content_attempts attempt
SET daily_assignment_id = assignment.id
FROM public.daily_question_assignments assignment
WHERE attempt.daily_assignment_id IS NULL
  AND (
      assignment.first_attempt_id = attempt.id
      OR assignment.corrected_attempt_id = attempt.id
  );

-- Version 5 did not record whether intermediate wrong attempts were corrections
-- or regular practice. Only explicit first/corrected references are safe to bind.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.content_attempts'::regclass
          AND conname = 'fk_content_attempts_daily_assignment'
    ) THEN
        ALTER TABLE public.content_attempts
            ADD CONSTRAINT fk_content_attempts_daily_assignment
            FOREIGN KEY (daily_assignment_id)
            REFERENCES public.daily_question_assignments(id)
            ON DELETE SET NULL;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS ix_content_attempts_daily_assignment
    ON public.content_attempts (daily_assignment_id, submitted_at DESC)
    WHERE daily_assignment_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_content_attempts_daily_submission
    ON public.content_attempts (daily_assignment_id, daily_submission_key)
    WHERE daily_assignment_id IS NOT NULL
      AND daily_submission_key IS NOT NULL;

DROP INDEX IF EXISTS public.uq_daily_question_wechat_event_student_reminder;

CREATE UNIQUE INDEX uq_daily_question_wechat_event_student_reminder
    ON public.daily_question_wechat_events (class_id, assignment_date)
    WHERE kind = 'automatic_student_reminder';

ALTER TABLE public.daily_question_class_settings
    ADD COLUMN IF NOT EXISTS schedule_version bigint;

UPDATE public.daily_question_class_settings
SET schedule_version = 0
WHERE schedule_version IS NULL;

ALTER TABLE public.daily_question_class_settings
    ALTER COLUMN schedule_version SET DEFAULT 0,
    ALTER COLUMN schedule_version SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_class_settings'::regclass
          AND conname = 'ck_daily_question_class_schedule_version'
    ) THEN
        ALTER TABLE public.daily_question_class_settings
            ADD CONSTRAINT ck_daily_question_class_schedule_version
            CHECK (schedule_version >= 0);
    END IF;
END
$$;

ALTER TABLE public.daily_question_class_selections
    ADD COLUMN IF NOT EXISTS question_title character varying(500),
    ADD COLUMN IF NOT EXISTS question_body text,
    ADD COLUMN IF NOT EXISTS question_difficulty double precision,
    ADD COLUMN IF NOT EXISTS question_concept_ids json,
    ADD COLUMN IF NOT EXISTS question_meta json;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.daily_question_assignments assignment
        JOIN public.daily_question_class_selections selection
          ON selection.class_id = assignment.class_id
         AND selection.assignment_date = assignment.assignment_date
        WHERE assignment.selection_reason = 'teacher_uniform'
          AND assignment.question_body IS NOT NULL
          AND selection.content_id IS DISTINCT FROM assignment.content_id
    ) THEN
        RAISE EXCEPTION 'uniform class selection content differs from delivered assignment';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM public.daily_question_assignments assignment
        JOIN public.daily_question_class_selections selection
          ON selection.class_id = assignment.class_id
         AND selection.assignment_date = assignment.assignment_date
        WHERE assignment.selection_reason = 'teacher_uniform'
          AND assignment.question_body IS NOT NULL
        GROUP BY assignment.class_id, assignment.assignment_date
        HAVING count(DISTINCT (
            assignment.content_id,
            assignment.question_title,
            assignment.question_body,
            assignment.question_difficulty,
            assignment.question_concept_ids::jsonb,
            assignment.question_meta::jsonb
        )) > 1
    ) THEN
        RAISE EXCEPTION 'uniform daily assignments contain divergent question snapshots';
    END IF;
END
$$;

WITH delivered_snapshot AS (
    SELECT DISTINCT ON (assignment.class_id, assignment.assignment_date)
           assignment.class_id,
           assignment.assignment_date,
           assignment.content_id,
           assignment.question_title,
           assignment.question_body,
           assignment.question_difficulty,
           assignment.question_concept_ids,
           assignment.question_meta
    FROM public.daily_question_assignments assignment
    WHERE assignment.selection_reason = 'teacher_uniform'
      AND assignment.content_id IS NOT NULL
      AND assignment.question_title IS NOT NULL
      AND assignment.question_body IS NOT NULL
      AND assignment.question_difficulty IS NOT NULL
      AND assignment.question_concept_ids IS NOT NULL
      AND assignment.question_meta IS NOT NULL
    ORDER BY assignment.class_id,
             assignment.assignment_date,
             assignment.assigned_at,
             assignment.id
)
UPDATE public.daily_question_class_selections selection
SET question_title = snapshot.question_title,
    question_body = snapshot.question_body,
    question_difficulty = snapshot.question_difficulty,
    question_concept_ids = snapshot.question_concept_ids,
    question_meta = snapshot.question_meta
FROM delivered_snapshot snapshot
WHERE selection.class_id = snapshot.class_id
  AND selection.assignment_date = snapshot.assignment_date
  AND selection.content_id IS NOT DISTINCT FROM snapshot.content_id
  AND selection.source = 'teacher_bank'
  AND selection.selection_reason = 'teacher_uniform';

UPDATE public.daily_question_class_selections selection
SET question_title = content.title,
    question_body = content.body,
    question_difficulty = content.difficulty,
    question_concept_ids = content.concept_ids,
    question_meta = content.meta
FROM public.contents content
WHERE selection.content_id = content.id
  AND (
      selection.question_title IS NULL
      OR selection.question_body IS NULL
      OR selection.question_difficulty IS NULL
      OR selection.question_concept_ids IS NULL
      OR selection.question_meta IS NULL
  );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.daily_question_class_selections'::regclass
          AND conname = 'ck_daily_question_class_selection_snapshot'
    ) THEN
        ALTER TABLE public.daily_question_class_selections
            ADD CONSTRAINT ck_daily_question_class_selection_snapshot
            CHECK (
                content_id IS NULL
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

ALTER TABLE public.daily_question_class_selections
    VALIDATE CONSTRAINT ck_daily_question_class_selection_snapshot;

CREATE TABLE IF NOT EXISTS public.class_enrollment_history (
    enrollment_id character varying(36) PRIMARY KEY,
    class_id character varying(36) NOT NULL
        REFERENCES public.classes(id) ON DELETE CASCADE,
    student_id character varying(36) NOT NULL
        REFERENCES public.users(id) ON DELETE CASCADE,
    joined_at timestamp without time zone NOT NULL,
    left_at timestamp without time zone,
    CONSTRAINT ck_class_enrollment_history_interval
        CHECK (left_at IS NULL OR left_at >= joined_at)
);

INSERT INTO public.class_enrollment_history (
    enrollment_id, class_id, student_id, joined_at, left_at
)
SELECT enrollment.id,
       enrollment.class_id,
       enrollment.student_id,
       enrollment.joined_at,
       NULL
FROM public.class_enrollments enrollment
ON CONFLICT (enrollment_id) DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS uq_class_enrollment_history_active_student
    ON public.class_enrollment_history (student_id)
    WHERE left_at IS NULL;

CREATE INDEX IF NOT EXISTS ix_class_enrollment_history_class_interval
    ON public.class_enrollment_history (class_id, joined_at, left_at, student_id);
