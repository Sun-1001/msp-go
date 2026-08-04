-- 0010: Complete mistake-book review, integrity, and question snapshots.
--
-- Versions 0010 through 0013 were never published beyond local development.
-- The shared migration chain ends at 0009, so their required final changes are
-- delivered atomically here as one forward migration.

-- Stage 1: persist review scheduling separately from immutable answer evidence.

CREATE TABLE public.mistake_review_tasks (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    content_id character varying(36) NOT NULL,
    question_title character varying(500) NOT NULL,
    question_body text NOT NULL,
    question_difficulty double precision NOT NULL,
    question_concept_ids json NOT NULL,
    question_meta json NOT NULL,
    question_generated_by_student_id character varying(36),
    source_attempt_id character varying(36),
    daily_assignment_id character varying(36),
    status character varying(24) DEFAULT 'pending' NOT NULL,
    stage integer DEFAULT 0 NOT NULL,
    review_count integer DEFAULT 0 NOT NULL,
    successful_review_count integer DEFAULT 0 NOT NULL,
    error_count integer DEFAULT 1 NOT NULL,
    due_at timestamp without time zone,
    last_review_attempt_id character varying(36),
    last_outcome boolean,
    last_reviewed_at timestamp without time zone,
    mastered_at timestamp without time zone,
    archived_at timestamp without time zone,
    revision bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);

ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT uq_mistake_review_task_student_content UNIQUE (student_id, content_id);
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_status
        CHECK (status IN ('pending', 'verification_due', 'mastered', 'archived'));
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_stage CHECK (stage >= 0 AND stage <= 3);
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_counts
        CHECK (review_count >= 0 AND successful_review_count >= 0 AND error_count > 0);
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_revision CHECK (revision >= 0);
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_state_dates CHECK (
        (status IN ('pending', 'verification_due') AND due_at IS NOT NULL AND mastered_at IS NULL AND archived_at IS NULL)
        OR (status = 'mastered' AND due_at IS NULL AND mastered_at IS NOT NULL AND archived_at IS NULL)
        OR (status = 'archived' AND due_at IS NULL AND archived_at IS NOT NULL)
    );
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_content_id_fkey
        FOREIGN KEY (content_id) REFERENCES public.contents(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_source_attempt_id_fkey
        FOREIGN KEY (source_attempt_id) REFERENCES public.content_attempts(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_daily_assignment_id_fkey
        FOREIGN KEY (daily_assignment_id) REFERENCES public.daily_question_assignments(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT mistake_review_tasks_last_review_attempt_id_fkey
        FOREIGN KEY (last_review_attempt_id) REFERENCES public.content_attempts(id) ON DELETE SET NULL;

CREATE INDEX ix_mistake_review_tasks_due
    ON public.mistake_review_tasks (student_id, due_at, id)
    WHERE status IN ('pending', 'verification_due');
CREATE INDEX ix_mistake_review_tasks_mastered
    ON public.mistake_review_tasks (student_id, mastered_at DESC, id)
    WHERE status = 'mastered';

CREATE TABLE public.mistake_record_archives (
    attempt_id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    archived_at timestamp without time zone NOT NULL
);

ALTER TABLE ONLY public.mistake_record_archives
    ADD CONSTRAINT mistake_record_archives_pkey PRIMARY KEY (attempt_id);
ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT uq_content_attempts_id_student UNIQUE (id, student_id);
ALTER TABLE ONLY public.mistake_record_archives
    ADD CONSTRAINT mistake_record_archives_attempt_student_fkey
        FOREIGN KEY (attempt_id, student_id)
        REFERENCES public.content_attempts(id, student_id)
        ON DELETE CASCADE;
ALTER TABLE ONLY public.mistake_record_archives
    ADD CONSTRAINT mistake_record_archives_student_id_fkey
        FOREIGN KEY (student_id) REFERENCES public.users(id) ON DELETE CASCADE;
CREATE INDEX ix_mistake_record_archives_student
    ON public.mistake_record_archives (student_id, archived_at DESC);

-- Only unresolved historical errors become active tasks. The latest wrong attempt ID
-- is already globally unique and provides a stable migration-time task ID.
WITH ranked_attempts AS (
    SELECT
        attempt.id,
        attempt.student_id,
        attempt.content_id,
        attempt.daily_assignment_id,
        attempt.is_correct,
        attempt.submitted_at,
        row_number() OVER (
            PARTITION BY attempt.student_id, attempt.content_id
            ORDER BY attempt.submitted_at DESC NULLS LAST, attempt.id DESC
        ) AS sequence
    FROM public.content_attempts attempt
    WHERE attempt.submitted_at IS NOT NULL
), unresolved AS (
    SELECT ranked.*
    FROM ranked_attempts ranked
    JOIN public.diagnosis_reports diagnosis ON diagnosis.attempt_id = ranked.id
    WHERE ranked.sequence = 1 AND ranked.is_correct = false
), error_counts AS (
    SELECT student_id, content_id, count(id)::integer AS error_count
    FROM public.content_attempts
    WHERE submitted_at IS NOT NULL AND is_correct = false
    GROUP BY student_id, content_id
)
INSERT INTO public.mistake_review_tasks (
    id,
    student_id,
    content_id,
    question_title,
    question_body,
    question_difficulty,
    question_concept_ids,
    question_meta,
    question_generated_by_student_id,
    source_attempt_id,
    daily_assignment_id,
    status,
    stage,
    review_count,
    successful_review_count,
    error_count,
    due_at,
    last_outcome,
    revision,
    created_at,
    updated_at
)
SELECT
    unresolved.id,
    unresolved.student_id,
    unresolved.content_id,
    coalesce(assignment.question_title, content.title),
    coalesce(assignment.question_body, content.body),
    coalesce(assignment.question_difficulty, content.difficulty),
    coalesce(assignment.question_concept_ids, content.concept_ids),
    coalesce(assignment.question_meta, content.meta),
    coalesce(assignment.question_generated_by_student_id, content.generated_by_student_id),
    unresolved.id,
    unresolved.daily_assignment_id,
    'pending',
    0,
    0,
    0,
    error_counts.error_count,
    unresolved.submitted_at + INTERVAL '1 day',
    false,
    0,
    unresolved.submitted_at,
    unresolved.submitted_at
FROM unresolved
JOIN error_counts
  ON error_counts.student_id = unresolved.student_id
 AND error_counts.content_id = unresolved.content_id
JOIN public.contents content ON content.id = unresolved.content_id
LEFT JOIN public.daily_question_assignments assignment
  ON assignment.id = unresolved.daily_assignment_id
 AND assignment.student_id = unresolved.student_id
 AND assignment.content_id = unresolved.content_id;

ALTER TABLE public.content_attempts
    ADD COLUMN review_task_id character varying(36),
    ADD COLUMN review_submission_key character varying(160),
    ADD COLUMN review_submission_response json,
    ADD COLUMN review_question_title character varying(500),
    ADD COLUMN review_question_body text,
    ADD COLUMN review_question_concept_ids json,
    ADD COLUMN review_question_difficulty double precision,
    ADD COLUMN review_question_meta json,
    ADD COLUMN review_question_generated_by_student_id character varying(36);

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_question_snapshot CHECK (
        (
            review_question_title IS NULL
            AND review_question_body IS NULL
            AND review_question_concept_ids IS NULL
            AND review_question_difficulty IS NULL
            AND review_question_meta IS NULL
            AND review_question_generated_by_student_id IS NULL
        )
        OR (
            review_question_title IS NOT NULL
            AND review_question_body IS NOT NULL
            AND review_question_concept_ids IS NOT NULL
            AND review_question_difficulty IS NOT NULL
            AND review_question_meta IS NOT NULL
        )
    );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT content_attempts_review_task_id_fkey
        FOREIGN KEY (review_task_id) REFERENCES public.mistake_review_tasks(id) ON DELETE SET NULL;

CREATE INDEX ix_content_attempts_review_task
    ON public.content_attempts (review_task_id, submitted_at DESC)
    WHERE review_task_id IS NOT NULL;
CREATE UNIQUE INDEX uq_content_attempts_review_submission
    ON public.content_attempts (review_task_id, review_submission_key)
    WHERE review_task_id IS NOT NULL AND review_submission_key IS NOT NULL;

-- Preserve how strongly each submitted attempt should influence mastery tracking.

ALTER TABLE public.content_attempts
    ADD COLUMN mastery_weight double precision DEFAULT 1 NOT NULL;

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_mastery_weight
        CHECK (mastery_weight > 0 AND mastery_weight <= 1);

-- Keep every concept ID collection used by mistake and review queries as a JSON array.

UPDATE public.contents content
SET concept_ids = CASE
    WHEN json_typeof(content.concept_ids) = 'string'
        THEN json_build_array(content.concept_ids #>> '{}')
    ELSE '[]'::json
END
WHERE json_typeof(content.concept_ids) IS DISTINCT FROM 'array';

UPDATE public.daily_question_assignments assignment
SET question_concept_ids = CASE
    WHEN json_typeof(assignment.question_concept_ids) = 'string'
        THEN json_build_array(assignment.question_concept_ids #>> '{}')
    ELSE coalesce(
        (
            SELECT content.concept_ids
            FROM public.contents content
            WHERE content.id = assignment.content_id
        ),
        '[]'::json
    )
END
WHERE assignment.question_concept_ids IS NOT NULL
  AND json_typeof(assignment.question_concept_ids) IS DISTINCT FROM 'array';

UPDATE public.daily_question_class_selections selection
SET question_concept_ids = CASE
    WHEN json_typeof(selection.question_concept_ids) = 'string'
        THEN json_build_array(selection.question_concept_ids #>> '{}')
    ELSE coalesce(
        (
            SELECT content.concept_ids
            FROM public.contents content
            WHERE content.id = selection.content_id
        ),
        '[]'::json
    )
END
WHERE selection.question_concept_ids IS NOT NULL
  AND json_typeof(selection.question_concept_ids) IS DISTINCT FROM 'array';

UPDATE public.mistake_review_tasks task
SET question_concept_ids = CASE
    WHEN json_typeof(task.question_concept_ids) = 'string'
        THEN json_build_array(task.question_concept_ids #>> '{}')
    ELSE coalesce(
        (
            SELECT content.concept_ids
            FROM public.contents content
            WHERE content.id = task.content_id
        ),
        '[]'::json
    )
END
WHERE json_typeof(task.question_concept_ids) IS DISTINCT FROM 'array';

UPDATE public.content_attempts attempt
SET review_question_concept_ids = CASE
    WHEN json_typeof(attempt.review_question_concept_ids) = 'string'
        THEN json_build_array(attempt.review_question_concept_ids #>> '{}')
    ELSE coalesce(
        (
            SELECT task.question_concept_ids
            FROM public.mistake_review_tasks task
            WHERE task.id = attempt.review_task_id
        ),
        (
            SELECT content.concept_ids
            FROM public.contents content
            WHERE content.id = attempt.content_id
        ),
        '[]'::json
    )
END
WHERE attempt.review_question_concept_ids IS NOT NULL
  AND json_typeof(attempt.review_question_concept_ids) IS DISTINCT FROM 'array';

ALTER TABLE ONLY public.contents
    ADD CONSTRAINT ck_contents_concept_ids_array
        CHECK (json_typeof(concept_ids) = 'array');

ALTER TABLE ONLY public.daily_question_assignments
    ADD CONSTRAINT ck_daily_question_assignment_concept_ids_array
        CHECK (question_concept_ids IS NULL OR json_typeof(question_concept_ids) = 'array');

ALTER TABLE ONLY public.daily_question_class_selections
    ADD CONSTRAINT ck_daily_question_selection_concept_ids_array
        CHECK (question_concept_ids IS NULL OR json_typeof(question_concept_ids) = 'array');

ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_concept_ids_array
        CHECK (json_typeof(question_concept_ids) = 'array');

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_concept_ids_array
        CHECK (
            review_question_concept_ids IS NULL
            OR json_typeof(review_question_concept_ids) = 'array'
        );

-- Give every problem and immutable problem snapshot a stable mastery dimension.

INSERT INTO public.knowledge_nodes (
    id,
    name,
    name_en,
    node_type,
    description,
    chapter,
    section,
    difficulty,
    latex_formula,
    tags,
    created_at,
    updated_at
)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    '未分类',
    'Uncategorized',
    'CONCEPT'::public.nodetype,
    '系统兜底知识点，用于追踪尚未匹配到具体知识点的题目。',
    NULL,
    NULL,
    0.5,
    NULL,
    '["系统"]'::json,
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC'
);

-- Keep only real knowledge-node IDs. Preserve the first occurrence order so
-- migration does not silently reorder immutable question snapshots.

UPDATE public.contents content
SET concept_ids = coalesce(
    (
        SELECT json_agg(valid.concept_id ORDER BY valid.first_position)
        FROM (
            SELECT node.id AS concept_id, min(concept.ordinality) AS first_position
            FROM json_array_elements_text(content.concept_ids)
                WITH ORDINALITY AS concept(value, ordinality)
            JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
            GROUP BY node.id
        ) valid
    ),
    '["00000000-0000-0000-0000-000000000001"]'::json
);

UPDATE public.daily_question_assignments assignment
SET question_concept_ids = coalesce(
    (
        SELECT json_agg(valid.concept_id ORDER BY valid.first_position)
        FROM (
            SELECT node.id AS concept_id, min(concept.ordinality) AS first_position
            FROM json_array_elements_text(assignment.question_concept_ids)
                WITH ORDINALITY AS concept(value, ordinality)
            JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
            GROUP BY node.id
        ) valid
    ),
    '["00000000-0000-0000-0000-000000000001"]'::json
)
WHERE assignment.question_concept_ids IS NOT NULL;

UPDATE public.daily_question_class_selections selection
SET question_concept_ids = coalesce(
    (
        SELECT json_agg(valid.concept_id ORDER BY valid.first_position)
        FROM (
            SELECT node.id AS concept_id, min(concept.ordinality) AS first_position
            FROM json_array_elements_text(selection.question_concept_ids)
                WITH ORDINALITY AS concept(value, ordinality)
            JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
            GROUP BY node.id
        ) valid
    ),
    '["00000000-0000-0000-0000-000000000001"]'::json
)
WHERE selection.question_concept_ids IS NOT NULL;

UPDATE public.mistake_review_tasks task
SET question_concept_ids = coalesce(
    (
        SELECT json_agg(valid.concept_id ORDER BY valid.first_position)
        FROM (
            SELECT node.id AS concept_id, min(concept.ordinality) AS first_position
            FROM json_array_elements_text(task.question_concept_ids)
                WITH ORDINALITY AS concept(value, ordinality)
            JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
            GROUP BY node.id
        ) valid
    ),
    '["00000000-0000-0000-0000-000000000001"]'::json
);

UPDATE public.content_attempts attempt
SET review_question_concept_ids = coalesce(
    (
        SELECT json_agg(valid.concept_id ORDER BY valid.first_position)
        FROM (
            SELECT node.id AS concept_id, min(concept.ordinality) AS first_position
            FROM json_array_elements_text(attempt.review_question_concept_ids)
                WITH ORDINALITY AS concept(value, ordinality)
            JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
            GROUP BY node.id
        ) valid
    ),
    '["00000000-0000-0000-0000-000000000001"]'::json
)
WHERE attempt.review_question_concept_ids IS NOT NULL;

ALTER TABLE ONLY public.contents
    ADD CONSTRAINT ck_contents_problem_concepts_nonempty
        CHECK (
            type <> 'PROBLEM'::public.contenttype
            OR CASE
                WHEN json_typeof(concept_ids) = 'array' THEN json_array_length(concept_ids) > 0
                ELSE false
            END
        );

ALTER TABLE ONLY public.daily_question_assignments
    ADD CONSTRAINT ck_daily_question_assignment_concepts_nonempty
        CHECK (
            question_concept_ids IS NULL
            OR CASE
                WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0
                ELSE false
            END
        );

ALTER TABLE ONLY public.daily_question_class_selections
    ADD CONSTRAINT ck_daily_question_selection_concepts_nonempty
        CHECK (
            question_concept_ids IS NULL
            OR CASE
                WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0
                ELSE false
            END
        );

ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_concepts_nonempty
        CHECK (
            CASE
                WHEN json_typeof(question_concept_ids) = 'array' THEN json_array_length(question_concept_ids) > 0
                ELSE false
            END
        );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_concepts_nonempty
        CHECK (
            review_question_concept_ids IS NULL
            OR CASE
                WHEN json_typeof(review_question_concept_ids) = 'array' THEN json_array_length(review_question_concept_ids) > 0
                ELSE false
            END
        );

-- Stage 2: mistake-book integrity, submission idempotency, and DKT reconciliation.
-- Close the remaining mistake-book integrity gaps after the core review schema.

-- Historical mistake redo submissions use a student-scoped client UUID. The
-- digest prevents one UUID from being reused for a different payload, while
-- the stored response makes a transport retry return the original result.

ALTER TABLE public.content_attempts
    ADD COLUMN review_submission_digest character(64),
    ADD COLUMN mistake_redo_original_attempt_id character varying(36),
    ADD COLUMN mistake_redo_submission_id uuid,
    ADD COLUMN mistake_redo_submission_digest character(64),
    ADD COLUMN mistake_redo_submission_response json,
    ADD COLUMN regular_submission_id uuid,
    ADD COLUMN regular_submission_digest character(64),
    ADD COLUMN regular_submission_response json;

-- Review attempts created by an earlier local draft can predate payload
-- binding. Preserve their stored responses while making any retry conflict
-- instead of accepting a request whose original digest cannot be reconstructed.
UPDATE public.content_attempts
SET review_submission_digest = repeat('0', 64)
WHERE review_submission_key IS NOT NULL;

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_submission_digest_pair CHECK (
        (review_submission_key IS NULL AND review_submission_digest IS NULL)
        OR (review_submission_key IS NOT NULL AND review_submission_digest IS NOT NULL)
    );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_submission_digest CHECK (
        review_submission_digest IS NULL
        OR review_submission_digest ~ '^[0-9a-f]{64}$'
    );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT content_attempts_mistake_redo_original_attempt_id_fkey
        FOREIGN KEY (mistake_redo_original_attempt_id, student_id)
        REFERENCES public.content_attempts(id, student_id)
        ON DELETE NO ACTION;

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_mistake_redo_submission CHECK (
        (
            mistake_redo_original_attempt_id IS NULL
            AND
            mistake_redo_submission_id IS NULL
            AND mistake_redo_submission_digest IS NULL
            AND mistake_redo_submission_response IS NULL
        )
        OR (
            mistake_redo_original_attempt_id IS NOT NULL
            AND
            mistake_redo_submission_id IS NOT NULL
            AND mistake_redo_submission_digest IS NOT NULL
            AND review_task_id IS NULL
            AND daily_assignment_id IS NULL
        )
    );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_mistake_redo_digest CHECK (
        mistake_redo_submission_digest IS NULL
        OR mistake_redo_submission_digest ~ '^[0-9a-f]{64}$'
    );

CREATE UNIQUE INDEX uq_content_attempts_mistake_redo_submission
    ON public.content_attempts (student_id, mistake_redo_submission_id)
    WHERE mistake_redo_submission_id IS NOT NULL;

CREATE INDEX ix_content_attempts_mistake_redo_original
    ON public.content_attempts (mistake_redo_original_attempt_id, submitted_at DESC, id DESC)
    WHERE mistake_redo_original_attempt_id IS NOT NULL;

-- Ordinary class and AI exercises use the same student-scoped UUID contract.
-- Existing attempts remain valid with all three columns null.

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_regular_submission CHECK (
        (
            regular_submission_id IS NULL
            AND regular_submission_digest IS NULL
            AND regular_submission_response IS NULL
        )
        OR (
            regular_submission_id IS NOT NULL
            AND regular_submission_digest IS NOT NULL
            AND daily_assignment_id IS NULL
            AND review_task_id IS NULL
            AND mistake_redo_original_attempt_id IS NULL
        )
    );

ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT ck_content_attempts_regular_submission_digest CHECK (
        regular_submission_digest IS NULL
        OR regular_submission_digest ~ '^[0-9a-f]{64}$'
    );

CREATE UNIQUE INDEX uq_content_attempts_regular_submission
    ON public.content_attempts (student_id, regular_submission_id)
    WHERE regular_submission_id IS NOT NULL;

-- The mistake library first selects the latest retained error per student and
-- content, then sorts by submission time. This index also supports grouped
-- historical error counts without indexing successful or draft attempts.

CREATE INDEX ix_content_attempts_mistake_aggregate
    ON public.content_attempts (student_id, content_id, submitted_at DESC, id DESC)
    WHERE is_correct = false AND submitted_at IS NOT NULL;

-- Normalize the only state combinations produced by the review state machine
-- before making those relationships database invariants.

UPDATE public.mistake_review_tasks
SET successful_review_count = CASE status
        WHEN 'pending' THEN 0
        WHEN 'verification_due' THEN least(greatest(successful_review_count, 1), 2)
        WHEN 'mastered' THEN 3
        ELSE least(greatest(successful_review_count, 0), 3)
    END;

UPDATE public.mistake_review_tasks
SET stage = successful_review_count,
    review_count = greatest(review_count, successful_review_count);

UPDATE public.mistake_review_tasks task
SET error_count = actual.error_count
FROM (
    SELECT student_id, content_id, count(*)::integer AS error_count
    FROM public.content_attempts
    WHERE is_correct = false AND submitted_at IS NOT NULL
    GROUP BY student_id, content_id
) actual
WHERE actual.student_id = task.student_id
  AND actual.content_id = task.content_id
  AND task.error_count IS DISTINCT FROM actual.error_count;

ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_count_order CHECK (
        successful_review_count <= review_count
    );

ALTER TABLE ONLY public.mistake_review_tasks
    ADD CONSTRAINT ck_mistake_review_tasks_stage_progress CHECK (
        stage = successful_review_count
        AND (
            (status = 'pending' AND stage = 0)
            OR (status = 'verification_due' AND stage IN (1, 2))
            OR (status = 'mastered' AND stage = 3)
            OR status = 'archived'
        )
    );

-- The initial normalization assigned old empty problem concept sets to the
-- Uncategorized concept. Attempts submitted before that assignment may have
-- incremented profile totals without producing a DKT state transition. Replay
-- is safe only when every submitted attempt for a student has an immutable
-- attempt or daily-question snapshot; current contents are mutable and must
-- never be used to reinterpret historical interactions. The helper functions
-- are removed before this migration commits.

CREATE FUNCTION public.msp_0011_valid_mastery_value(value json)
RETURNS boolean
LANGUAGE plpgsql
IMMUTABLE
STRICT
AS $$
DECLARE
    parsed_value double precision;
BEGIN
    IF json_typeof(value) IS DISTINCT FROM 'number' THEN
        RETURN false;
    END IF;
    parsed_value := (value #>> '{}')::double precision;
    RETURN parsed_value >= 0 AND parsed_value <= 1;
EXCEPTION
    WHEN invalid_text_representation OR numeric_value_out_of_range THEN
        RETURN false;
END
$$;

-- The old schema accepted any non-null JSON. Normalize every profile before
-- DKT target comparison so malformed values cannot abort replay or turn an
-- object merge into an array.
UPDATE public.student_profiles profile
SET mastery_vector = coalesce(
    (
        SELECT json_object_agg(entry.key, entry.value ORDER BY entry.key)
        FROM json_each(
            CASE
                WHEN json_typeof(profile.mastery_vector) = 'object'
                    THEN profile.mastery_vector
                ELSE '{}'::json
            END
        ) entry
        JOIN public.knowledge_nodes node ON node.id = entry.key
        WHERE public.msp_0011_valid_mastery_value(entry.value)
    ),
    '{}'::json
);

ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT ck_student_profiles_mastery_vector_object CHECK (
        json_typeof(mastery_vector) = 'object'
        AND NOT jsonb_path_exists(
            mastery_vector::jsonb,
            '$.* ? (@.type() != "number" || @ < 0 || @ > 1)'
        )
    );

-- Old invalid concept IDs are reclassified from immutable attempts below.
-- Remove their stale projections before rebuilding the valid dimensions.
DELETE FROM public.student_concept_dkt_states state
WHERE NOT EXISTS (
    SELECT 1
    FROM public.knowledge_nodes node
    WHERE node.id = state.concept_id
);

ALTER TABLE ONLY public.student_concept_dkt_states
    ADD CONSTRAINT student_concept_dkt_states_concept_id_fkey
        FOREIGN KEY (concept_id)
        REFERENCES public.knowledge_nodes(id)
        ON DELETE RESTRICT;

CREATE FUNCTION public.msp_0011_dkt_token(
    source double precision[],
    token text,
    token_weight double precision
) RETURNS double precision[]
LANGUAGE plpgsql
IMMUTABLE
STRICT
AS $$
DECLARE
    result double precision[] := source;
    hash_signed bigint := -3750763034362895579;
    hash_value numeric := 14695981039346656037;
    token_bytes bytea := convert_to(token, 'UTF8');
    byte_index integer;
    vector_index integer;
    secondary_index integer;
    token_sign double precision := 1;
BEGIN
    IF octet_length(token_bytes) > 0 THEN
        FOR byte_index IN 0..octet_length(token_bytes) - 1 LOOP
            hash_signed := hash_signed # get_byte(token_bytes, byte_index);
            hash_value := CASE
                WHEN hash_signed < 0
                    THEN hash_signed::numeric + 18446744073709551616
                ELSE hash_signed::numeric
            END;
            hash_value := mod(
                hash_value * 1099511628211,
                18446744073709551616
            );
            hash_signed := CASE
                WHEN hash_value >= 9223372036854775808
                    THEN (hash_value - 18446744073709551616)::bigint
                ELSE hash_value::bigint
            END;
        END LOOP;
    END IF;

    vector_index := mod(hash_value, 16)::integer + 1;
    IF mod(floor(hash_value / 256), 2) = 1 THEN
        token_sign := -1;
    END IF;
    secondary_index := mod(floor(hash_value / 65536), 16)::integer + 1;
    result[vector_index] := result[vector_index] + token_sign * token_weight;
    result[secondary_index] := result[secondary_index] + token_sign * token_weight * 0.37;
    RETURN result;
END
$$;

CREATE FUNCTION public.msp_0011_dkt_embedding(
    exercise_id text,
    concept_ids text[],
    is_correct boolean,
    difficulty double precision,
    sequence_position integer,
    sequence_length integer
) RETURNS double precision[]
LANGUAGE plpgsql
IMMUTABLE
STRICT
AS $$
DECLARE
    result double precision[] := array_fill(0::double precision, ARRAY[16]);
    concept_id text;
    coordinate integer;
    position_value double precision := sequence_position;
    denominator double precision;
    vector_norm double precision := 0;
BEGIN
    result := public.msp_0011_dkt_token(result, 'exercise:' || exercise_id, 1);
    FOREACH concept_id IN ARRAY concept_ids LOOP
        result := public.msp_0011_dkt_token(result, 'concept:' || concept_id, 0.85);
    END LOOP;
    result := public.msp_0011_dkt_token(
        result,
        CASE WHEN is_correct THEN 'response:correct' ELSE 'response:incorrect' END,
        0.65
    );
    result[16] := result[16] + least(greatest(difficulty, 0), 1) - 0.5;

    IF sequence_length > 1 THEN
        position_value := sequence_position::double precision
            / (sequence_length - 1)::double precision * 63;
    END IF;
    FOR coordinate IN 1..16 LOOP
        denominator := power(10000::double precision, (2 * ((coordinate - 1) / 2))::double precision / 16);
        IF mod(coordinate - 1, 2) = 0 THEN
            result[coordinate] := result[coordinate] + 0.25 * sin(position_value / denominator);
        ELSE
            result[coordinate] := result[coordinate] + 0.25 * cos(position_value / denominator);
        END IF;
        vector_norm := vector_norm + result[coordinate] * result[coordinate];
    END LOOP;
    vector_norm := sqrt(vector_norm);
    IF vector_norm > 0.000001 THEN
        FOR coordinate IN 1..16 LOOP
            result[coordinate] := result[coordinate] / vector_norm;
        END LOOP;
    END IF;
    RETURN result;
END
$$;

CREATE FUNCTION public.msp_0011_dkt_dot(
    left_vector double precision[],
    right_vector double precision[]
) RETURNS double precision
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
    SELECT coalesce(sum(left_vector[index] * right_vector[index]), 0)
    FROM generate_subscripts(left_vector, 1) AS index
    WHERE index <= coalesce(array_length(right_vector, 1), 0)
$$;

CREATE FUNCTION public.msp_0011_next_recommendation(
    is_correct boolean,
    mastery_update jsonb
) RETURNS text
LANGUAGE sql
IMMUTABLE
STRICT
AS $$
    SELECT CASE
        WHEN is_correct OR mastery_update = '{}'::jsonb THEN 'continue'
        WHEN (
            SELECT avg((entry.value #>> '{}')::double precision)
            FROM jsonb_each(mastery_update) entry
        ) < 0.3 THEN 'review'
        ELSE 'continue'
    END
$$;

CREATE TEMPORARY TABLE msp_0011_dkt_attempts ON COMMIT DROP AS
WITH daily_snapshot AS (
    SELECT
        attempt.id AS attempt_id,
        assignment.question_concept_ids,
        assignment.question_difficulty
    FROM public.content_attempts attempt
    LEFT JOIN LATERAL (
        SELECT
            candidate.question_concept_ids,
            candidate.question_difficulty
        FROM public.daily_question_assignments candidate
        WHERE candidate.student_id = attempt.student_id
          AND candidate.content_id = attempt.content_id
          AND (
              candidate.id = attempt.daily_assignment_id
              OR (
                  attempt.daily_assignment_id IS NULL
                  AND (
                      candidate.first_attempt_id = attempt.id
                      OR candidate.corrected_attempt_id = attempt.id
                  )
              )
          )
        ORDER BY (candidate.id = attempt.daily_assignment_id) DESC NULLS LAST,
                 candidate.id
        LIMIT 1
    ) assignment ON true
), trustworthy_students AS (
    SELECT attempt.student_id
    FROM public.content_attempts attempt
    LEFT JOIN daily_snapshot ON daily_snapshot.attempt_id = attempt.id
    WHERE attempt.submitted_at IS NOT NULL
    GROUP BY attempt.student_id
    HAVING bool_and(
        (
            attempt.review_question_concept_ids IS NOT NULL
            AND attempt.review_question_difficulty IS NOT NULL
        )
        OR (
            daily_snapshot.question_concept_ids IS NOT NULL
            AND daily_snapshot.question_difficulty IS NOT NULL
        )
    )
), normalized AS (
    SELECT
        attempt.id,
        attempt.student_id,
        attempt.content_id,
        attempt.is_correct,
        least(greatest(
            coalesce(attempt.review_question_difficulty, daily_snapshot.question_difficulty),
            0
        ), 1)::double precision AS difficulty,
        least(greatest(attempt.mastery_weight, 0.01), 1)::double precision AS mastery_weight,
        attempt.submitted_at,
        diagnosis.error_type::text AS error_type,
        ARRAY(
            SELECT valid.concept_id
            FROM (
                SELECT node.id::text AS concept_id,
                       min(concept.ordinality) AS first_position
                FROM json_array_elements_text(
                    coalesce(
                        attempt.review_question_concept_ids,
                        daily_snapshot.question_concept_ids
                    )
                ) WITH ORDINALITY AS concept(value, ordinality)
                JOIN public.knowledge_nodes node ON node.id = btrim(concept.value)
                GROUP BY node.id
            ) valid
            ORDER BY valid.first_position, valid.concept_id
        ) AS raw_concept_ids
    FROM public.content_attempts attempt
    JOIN trustworthy_students trustworthy ON trustworthy.student_id = attempt.student_id
    LEFT JOIN daily_snapshot ON daily_snapshot.attempt_id = attempt.id
    LEFT JOIN public.diagnosis_reports diagnosis ON diagnosis.attempt_id = attempt.id
    WHERE attempt.submitted_at IS NOT NULL
), tracked AS (
    SELECT
        normalized.*,
        CASE
            WHEN cardinality(raw_concept_ids) = 0
                THEN ARRAY['00000000-0000-0000-0000-000000000001']::text[]
            ELSE raw_concept_ids
        END AS concept_ids
    FROM normalized
)
SELECT
    tracked.*,
    row_number() OVER (
        PARTITION BY tracked.student_id
        ORDER BY tracked.submitted_at, tracked.id
    )::integer AS student_sequence
FROM tracked;

CREATE INDEX msp_0011_dkt_attempts_student_sequence
    ON msp_0011_dkt_attempts (student_id, student_sequence);

CREATE TEMPORARY TABLE msp_0011_dkt_expected ON COMMIT DROP AS
SELECT
    attempt.student_id,
    concept_id,
    count(*)::integer AS attempt_count,
    count(*) FILTER (WHERE attempt.is_correct)::integer AS correct_count,
    count(*) FILTER (WHERE NOT attempt.is_correct)::integer AS incorrect_count,
    (array_agg(attempt.id ORDER BY attempt.student_sequence DESC))[1] AS last_attempt_id,
    (array_agg(attempt.content_id ORDER BY attempt.student_sequence DESC))[1] AS last_exercise_id,
    (array_agg(attempt.is_correct ORDER BY attempt.student_sequence DESC))[1] AS last_outcome,
    (array_agg(attempt.submitted_at ORDER BY attempt.student_sequence DESC))[1] AS last_attempt_at,
    (array_agg(attempt.student_sequence ORDER BY attempt.student_sequence DESC))[1] AS last_sequence
FROM msp_0011_dkt_attempts attempt
CROSS JOIN LATERAL unnest(attempt.concept_ids) AS concept_id
GROUP BY attempt.student_id, concept_id;

CREATE TEMPORARY TABLE msp_0011_dkt_targets ON COMMIT DROP AS
SELECT expected.student_id, expected.concept_id
FROM msp_0011_dkt_expected expected
LEFT JOIN public.student_concept_dkt_states state
  ON state.student_id = expected.student_id
 AND state.concept_id = expected.concept_id
LEFT JOIN public.student_profiles profile ON profile.student_id = expected.student_id
WHERE state.student_id IS NULL
   OR state.attempt_count IS DISTINCT FROM expected.attempt_count
   OR state.correct_count IS DISTINCT FROM expected.correct_count
   OR state.incorrect_count IS DISTINCT FROM expected.incorrect_count
   OR state.last_exercise_id IS DISTINCT FROM expected.last_exercise_id
   OR state.last_outcome IS DISTINCT FROM expected.last_outcome
   OR state.last_attempt_at IS DISTINCT FROM expected.last_attempt_at
   OR jsonb_typeof(profile.mastery_vector::jsonb -> expected.concept_id) IS DISTINCT FROM 'number'
   OR (profile.mastery_vector::jsonb ->> expected.concept_id)::double precision
        IS DISTINCT FROM state.mastery_prob;

CREATE TEMPORARY TABLE msp_0011_dkt_replay_states (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    concept_id character varying(128) NOT NULL,
    mastery_prob double precision NOT NULL,
    confidence double precision NOT NULL,
    attempt_count integer NOT NULL,
    correct_count integer NOT NULL,
    incorrect_count integer NOT NULL,
    sequence_length integer NOT NULL,
    attention_weight double precision NOT NULL,
    last_outcome boolean NOT NULL,
    last_exercise_id character varying NOT NULL,
    last_attempt_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    PRIMARY KEY (student_id, concept_id)
) ON COMMIT DROP;

CREATE TEMPORARY TABLE msp_0011_dkt_replayed_updates (
    attempt_id character varying(36) PRIMARY KEY,
    mastery_update jsonb NOT NULL
) ON COMMIT DROP;

DO $$
DECLARE
    current_attempt record;
    target_concept text;
    profile_preferred double precision;
    profile_pace double precision;
    state_row record;
    prior_mastery double precision;
    attention_signal double precision;
    attention_weight double precision;
    current_gain double precision;
    current_loss double precision;
    next_mastery double precision;
    next_confidence double precision;
    replay_attempt_count integer;
    replay_correct_count integer;
    replay_incorrect_count integer;
    replay_created_at timestamp without time zone;
    replay_id character varying(36);
BEGIN
    FOR current_attempt IN
        SELECT attempt.*
        FROM msp_0011_dkt_attempts attempt
        WHERE EXISTS (
            SELECT 1
            FROM msp_0011_dkt_targets target
            WHERE target.student_id = attempt.student_id
              AND target.concept_id = ANY(attempt.concept_ids)
        )
        ORDER BY attempt.student_id, attempt.student_sequence
    LOOP
        SELECT profile.preferred_difficulty, profile.learning_pace
        INTO profile_preferred, profile_pace
        FROM public.student_profiles profile
        WHERE profile.student_id = current_attempt.student_id;
        profile_preferred := coalesce(profile_preferred, 0.5);
        profile_pace := coalesce(profile_pace, 1);

        FOREACH target_concept IN ARRAY current_attempt.concept_ids LOOP
            IF NOT EXISTS (
                SELECT 1
                FROM msp_0011_dkt_targets target
                WHERE target.student_id = current_attempt.student_id
                  AND target.concept_id = target_concept
            ) THEN
                CONTINUE;
            END IF;

            SELECT *
            INTO state_row
            FROM msp_0011_dkt_replay_states state
            WHERE state.student_id = current_attempt.student_id
              AND state.concept_id = target_concept;

            IF FOUND THEN
                replay_id := state_row.id;
                prior_mastery := state_row.mastery_prob;
                replay_attempt_count := state_row.attempt_count;
                replay_correct_count := state_row.correct_count;
                replay_incorrect_count := state_row.incorrect_count;
                replay_created_at := state_row.created_at;
                IF current_attempt.submitted_at > state_row.last_attempt_at
                   AND prior_mastery > 0.05 THEN
                    prior_mastery := 0.05 + (prior_mastery - 0.05) * exp(
                        -0.025 * extract(epoch FROM (
                            current_attempt.submitted_at - state_row.last_attempt_at
                        )) / 86400
                    );
                END IF;
            ELSE
                SELECT coalesce(existing.id, (
                    md5('dkt-state:' || current_attempt.student_id || ':' || target_concept)::uuid
                )::text)
                INTO replay_id
                FROM (SELECT 1) seed
                LEFT JOIN public.student_concept_dkt_states existing
                  ON existing.student_id = current_attempt.student_id
                 AND existing.concept_id = target_concept;
                IF target_concept = '00000000-0000-0000-0000-000000000001' THEN
                    prior_mastery := 0.5;
                ELSE
                    prior_mastery := least(greatest(
                        0.45
                        + 0.12 * (
                            least(greatest(profile_preferred, 0), 1)
                            - current_attempt.difficulty
                        )
                        + 0.05 * (least(greatest(profile_pace, 0.2), 2) - 1),
                        0.15
                    ), 0.75);
                END IF;
                replay_attempt_count := 0;
                replay_correct_count := 0;
                replay_incorrect_count := 0;
                replay_created_at := current_attempt.submitted_at;
            END IF;
            prior_mastery := least(greatest(prior_mastery, 0.001), 0.999);

            WITH sequence_items AS (
                SELECT
                    attempt.*,
                    row_number() OVER (ORDER BY attempt.student_sequence)::integer - 1 AS sequence_position,
                    count(*) OVER ()::integer AS sequence_length
                FROM msp_0011_dkt_attempts attempt
                WHERE attempt.student_id = current_attempt.student_id
                  AND attempt.student_sequence BETWEEN greatest(current_attempt.student_sequence - 63, 1)
                                                   AND current_attempt.student_sequence
            ), embedded AS (
                SELECT
                    item.*,
                    public.msp_0011_dkt_embedding(
                        item.content_id,
                        item.concept_ids,
                        item.is_correct,
                        item.difficulty,
                        item.sequence_position,
                        item.sequence_length
                    ) AS item_embedding,
                    public.msp_0011_dkt_embedding(
                        current_attempt.content_id,
                        current_attempt.concept_ids,
                        current_attempt.is_correct,
                        current_attempt.difficulty,
                        item.sequence_length - 1,
                        item.sequence_length
                    ) AS query_embedding
                FROM sequence_items item
            ), scores AS (
                SELECT embedded.*,
                       public.msp_0011_dkt_dot(query_embedding, item_embedding) / sqrt(16) AS score
                FROM embedded
            ), shifted_scores AS (
                SELECT scores.*, exp(score - max(score) OVER ()) AS shifted_score
                FROM scores
            ), normalized_scores AS (
                SELECT shifted_scores.*,
                       shifted_score
                           / greatest(sum(shifted_score) OVER (), 0.000001) AS attention
                FROM shifted_scores
            )
            SELECT
                least(greatest(sum(
                    attention
                    * CASE WHEN is_correct THEN 1 ELSE -1 END
                    * CASE
                        WHEN target_concept = ANY(concept_ids) THEN 1
                        WHEN current_attempt.concept_ids && concept_ids THEN 0.55
                        ELSE 0.15
                    END
                    * (0.75 + 0.25 * difficulty)
                    * mastery_weight
                ), -1), 1),
                least(greatest(max(attention) * max(sequence_length), 0), 1)
            INTO attention_signal, attention_weight
            FROM normalized_scores;

            current_gain := least(greatest(
                0.18
                + 0.05 * least(greatest(profile_preferred - current_attempt.difficulty, -1), 1)
                + 0.03 * (least(greatest(profile_pace, 0.2), 2) - 1),
                0.08
            ), 0.3);
            current_loss := 0.22 + 0.10 * current_attempt.difficulty
                - 0.03 * (least(greatest(profile_pace, 0.2), 2) - 1);
            IF current_attempt.error_type IN ('conceptual', 'logical') THEN
                current_loss := current_loss + 0.05;
            ELSIF current_attempt.error_type IN ('symbolic', 'calculation') THEN
                current_loss := current_loss + 0.02;
            END IF;
            current_loss := least(greatest(current_loss, 0.12), 0.38);

            IF current_attempt.is_correct THEN
                next_mastery := prior_mastery
                    + current_attempt.mastery_weight * current_gain * (1 - prior_mastery);
            ELSE
                next_mastery := prior_mastery
                    - current_attempt.mastery_weight * current_loss * prior_mastery;
            END IF;
            next_mastery := next_mastery
                + current_attempt.mastery_weight * 0.10 * coalesce(attention_signal, 0);
            next_mastery := round(
                least(greatest(next_mastery, 0.001), 0.999)::numeric,
                4
            )::double precision;
            next_confidence := round(least(greatest(
                0.18
                + 0.44 * (1 - exp(-(replay_attempt_count + 1)::double precision / 5))
                + 0.28 * ln(1 + least(current_attempt.student_sequence, 64)) / ln(65)
                + 0.10 * coalesce(attention_weight, 0),
                0
            ), 1)::numeric, 4)::double precision;

            INSERT INTO msp_0011_dkt_replay_states (
                id, student_id, concept_id, mastery_prob, confidence,
                attempt_count, correct_count, incorrect_count, sequence_length,
                attention_weight, last_outcome, last_exercise_id,
                last_attempt_at, created_at, updated_at
            ) VALUES (
                replay_id,
                current_attempt.student_id,
                target_concept,
                next_mastery,
                next_confidence,
                replay_attempt_count + 1,
                replay_correct_count + CASE WHEN current_attempt.is_correct THEN 1 ELSE 0 END,
                replay_incorrect_count + CASE WHEN current_attempt.is_correct THEN 0 ELSE 1 END,
                least(current_attempt.student_sequence, 64),
                round(coalesce(attention_weight, 0)::numeric, 4)::double precision,
                current_attempt.is_correct,
                current_attempt.content_id,
                current_attempt.submitted_at,
                replay_created_at,
                current_attempt.submitted_at
            )
            ON CONFLICT (student_id, concept_id) DO UPDATE SET
                mastery_prob = EXCLUDED.mastery_prob,
                confidence = EXCLUDED.confidence,
                attempt_count = EXCLUDED.attempt_count,
                correct_count = EXCLUDED.correct_count,
                incorrect_count = EXCLUDED.incorrect_count,
                sequence_length = EXCLUDED.sequence_length,
                attention_weight = EXCLUDED.attention_weight,
                last_outcome = EXCLUDED.last_outcome,
                last_exercise_id = EXCLUDED.last_exercise_id,
                last_attempt_at = EXCLUDED.last_attempt_at,
                updated_at = EXCLUDED.updated_at;

            INSERT INTO msp_0011_dkt_replayed_updates (attempt_id, mastery_update)
            VALUES (current_attempt.id, jsonb_build_object(target_concept, next_mastery))
            ON CONFLICT (attempt_id) DO UPDATE SET
                mastery_update = msp_0011_dkt_replayed_updates.mastery_update
                    || EXCLUDED.mastery_update;
        END LOOP;
    END LOOP;
END
$$;

INSERT INTO public.student_concept_dkt_states (
    id, student_id, concept_id, mastery_prob, confidence,
    attempt_count, correct_count, incorrect_count, sequence_length,
    attention_weight, last_outcome, last_exercise_id,
    last_attempt_at, created_at, updated_at
)
SELECT
    replay.id, replay.student_id, replay.concept_id, replay.mastery_prob, replay.confidence,
    replay.attempt_count, replay.correct_count, replay.incorrect_count, replay.sequence_length,
    replay.attention_weight, replay.last_outcome, replay.last_exercise_id,
    replay.last_attempt_at, replay.created_at, replay.updated_at
FROM msp_0011_dkt_replay_states replay
ON CONFLICT ON CONSTRAINT uq_student_concept_dkt_state DO UPDATE SET
    mastery_prob = EXCLUDED.mastery_prob,
    confidence = EXCLUDED.confidence,
    attempt_count = EXCLUDED.attempt_count,
    correct_count = EXCLUDED.correct_count,
    incorrect_count = EXCLUDED.incorrect_count,
    sequence_length = EXCLUDED.sequence_length,
    attention_weight = EXCLUDED.attention_weight,
    last_outcome = EXCLUDED.last_outcome,
    last_exercise_id = EXCLUDED.last_exercise_id,
    last_attempt_at = EXCLUDED.last_attempt_at,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;

WITH replayed_mastery AS (
    SELECT student_id,
           jsonb_object_agg(concept_id, mastery_prob ORDER BY concept_id) AS mastery_patch
    FROM msp_0011_dkt_replay_states
    GROUP BY student_id
), profile_counts AS (
    SELECT
        target_student.student_id,
        count(attempt.id)::integer AS total_exercises,
        count(attempt.id) FILTER (WHERE attempt.is_correct)::integer AS correct_count,
        max(attempt.submitted_at) AS last_attempt_at
    FROM (SELECT DISTINCT student_id FROM msp_0011_dkt_targets) target_student
    LEFT JOIN public.content_attempts attempt
      ON attempt.student_id = target_student.student_id
     AND attempt.submitted_at IS NOT NULL
    GROUP BY target_student.student_id
), error_counts AS (
    SELECT target_student.student_id,
           coalesce(weighted_errors.error_tendency, '{}'::jsonb) AS error_tendency
    FROM (SELECT DISTINCT student_id FROM msp_0011_dkt_targets) target_student
    LEFT JOIN (
        SELECT student_id,
               jsonb_object_agg(error_type, error_weight ORDER BY error_type) AS error_tendency
        FROM (
            SELECT attempt.student_id,
                   diagnosis.error_type::text AS error_type,
                   sum(attempt.mastery_weight)::double precision AS error_weight
            FROM public.content_attempts attempt
            JOIN public.diagnosis_reports diagnosis ON diagnosis.attempt_id = attempt.id
            WHERE attempt.submitted_at IS NOT NULL
              AND NOT attempt.is_correct
              AND diagnosis.error_type IS NOT NULL
            GROUP BY attempt.student_id, diagnosis.error_type
        ) counts
        GROUP BY student_id
    ) weighted_errors ON weighted_errors.student_id = target_student.student_id
), profile_projection AS (
    SELECT
        replayed_mastery.student_id,
        (coalesce(profile.mastery_vector, '{}'::json)::jsonb
            || replayed_mastery.mastery_patch)::json AS mastery_vector,
        error_counts.error_tendency::json AS error_tendency,
        profile_counts.total_exercises,
        profile_counts.correct_count,
        profile_counts.last_attempt_at
    FROM replayed_mastery
    JOIN profile_counts ON profile_counts.student_id = replayed_mastery.student_id
    JOIN error_counts ON error_counts.student_id = replayed_mastery.student_id
    LEFT JOIN public.student_profiles profile
      ON profile.student_id = replayed_mastery.student_id
)
INSERT INTO public.student_profiles (
    id,
    student_id,
    mastery_vector,
    error_tendency,
    preferred_difficulty,
    learning_pace,
    total_exercises,
    correct_count,
    total_study_time_minutes,
    recent_concepts,
    updated_at,
    portrait_version,
    portrait_revision
)
SELECT
    (md5('student-profile:' || projection.student_id)::uuid)::text,
    projection.student_id,
    projection.mastery_vector,
    projection.error_tendency,
    0.5,
    1,
    projection.total_exercises,
    projection.correct_count,
    0,
    '[]'::json,
    projection.last_attempt_at,
    0,
    0
FROM profile_projection projection
ON CONFLICT (student_id) DO UPDATE SET
    mastery_vector = EXCLUDED.mastery_vector,
    error_tendency = EXCLUDED.error_tendency,
    total_exercises = EXCLUDED.total_exercises,
    correct_count = EXCLUDED.correct_count,
    updated_at = greatest(public.student_profiles.updated_at, EXCLUDED.updated_at);

-- Reconstruct action baselines from facts before the action start so repaired
-- older attempts do not appear as newly completed practice.
WITH expected_baselines AS (
    SELECT action.id,
           count(attempt.id)::integer AS baseline_attempt_count
    FROM public.student_portrait_actions action
    JOIN msp_0011_dkt_targets target
      ON target.student_id = action.student_id
     AND target.concept_id = action.concept_id
    LEFT JOIN msp_0011_dkt_attempts attempt
      ON attempt.student_id = action.student_id
     AND action.concept_id = ANY(attempt.concept_ids)
     AND attempt.submitted_at < action.started_at
    GROUP BY action.id
)
UPDATE public.student_portrait_actions action
SET baseline_attempt_count = expected.baseline_attempt_count,
    updated_at = greatest(action.updated_at, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
FROM expected_baselines expected
WHERE expected.id = action.id
  AND action.baseline_attempt_count IS DISTINCT FROM expected.baseline_attempt_count;

UPDATE public.content_attempts attempt
SET review_submission_response = jsonb_set(
        jsonb_set(attempt.review_submission_response::jsonb, '{mastery_update}', replay.mastery_update, true),
        '{next_recommendation}',
        to_jsonb(public.msp_0011_next_recommendation(attempt.is_correct, replay.mastery_update)), true
    )::json
FROM msp_0011_dkt_replayed_updates replay
WHERE replay.attempt_id = attempt.id
  AND attempt.review_submission_response IS NOT NULL;

UPDATE public.content_attempts attempt
SET daily_submission_response = jsonb_set(
        jsonb_set(attempt.daily_submission_response::jsonb, '{mastery_update}', replay.mastery_update, true),
        '{next_recommendation}',
        to_jsonb(public.msp_0011_next_recommendation(attempt.is_correct, replay.mastery_update)), true
    )::json
FROM msp_0011_dkt_replayed_updates replay
WHERE replay.attempt_id = attempt.id
  AND attempt.daily_submission_response IS NOT NULL;

DROP FUNCTION public.msp_0011_next_recommendation(boolean, jsonb);
DROP FUNCTION public.msp_0011_dkt_dot(double precision[], double precision[]);
DROP FUNCTION public.msp_0011_dkt_embedding(
    text, text[], boolean, double precision, integer, integer
);
DROP FUNCTION public.msp_0011_dkt_token(double precision[], text, double precision);
DROP FUNCTION public.msp_0011_valid_mastery_value(json);

-- Freeze the question attributes used by every submitted learning interaction.
-- Historical ordinary attempts did not store a snapshot, so their original
-- question version cannot be reconstructed. Stabilize those rows using the
-- best evidence still available without replaying derived mastery state.

WITH snapshot_source AS (
    SELECT
        attempt.id,
        coalesce(assignment.question_title, content.title) AS question_title,
        coalesce(assignment.question_body, content.body) AS question_body,
        coalesce(assignment.question_concept_ids, content.concept_ids) AS question_concept_ids,
        coalesce(assignment.question_difficulty, content.difficulty) AS question_difficulty,
        coalesce(assignment.question_meta, content.meta) AS question_meta,
        coalesce(
            assignment.question_generated_by_student_id,
            content.generated_by_student_id
        ) AS question_generated_by_student_id
    FROM public.content_attempts attempt
    JOIN public.contents content ON content.id = attempt.content_id
    LEFT JOIN LATERAL (
        SELECT
            candidate.question_title,
            candidate.question_body,
            candidate.question_concept_ids,
            candidate.question_difficulty,
            candidate.question_meta,
            candidate.question_generated_by_student_id
        FROM public.daily_question_assignments candidate
        WHERE candidate.student_id = attempt.student_id
          AND candidate.content_id = attempt.content_id
          AND candidate.question_title IS NOT NULL
          AND candidate.question_body IS NOT NULL
          AND candidate.question_concept_ids IS NOT NULL
          AND candidate.question_difficulty IS NOT NULL
          AND candidate.question_meta IS NOT NULL
          AND (
              candidate.id = attempt.daily_assignment_id
              OR (
                  attempt.daily_assignment_id IS NULL
                  AND (
                      candidate.first_attempt_id = attempt.id
                      OR candidate.corrected_attempt_id = attempt.id
                  )
              )
          )
        ORDER BY
            (candidate.id = attempt.daily_assignment_id) DESC NULLS LAST,
            candidate.id
        LIMIT 1
    ) assignment ON true
    WHERE attempt.submitted_at IS NOT NULL
      AND (
          attempt.review_question_title IS NULL
          OR attempt.review_question_body IS NULL
          OR attempt.review_question_concept_ids IS NULL
          OR attempt.review_question_difficulty IS NULL
          OR attempt.review_question_meta IS NULL
      )
)
UPDATE public.content_attempts attempt
SET review_question_title = source.question_title,
    review_question_body = source.question_body,
    review_question_concept_ids = source.question_concept_ids,
    review_question_difficulty = source.question_difficulty,
    review_question_meta = source.question_meta,
    review_question_generated_by_student_id = source.question_generated_by_student_id
FROM snapshot_source source
WHERE source.id = attempt.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.content_attempts attempt
        WHERE attempt.submitted_at IS NOT NULL
          AND (
              attempt.review_question_title IS NULL
              OR attempt.review_question_body IS NULL
              OR attempt.review_question_concept_ids IS NULL
              OR attempt.review_question_difficulty IS NULL
              OR attempt.review_question_meta IS NULL
          )
    ) THEN
        RAISE EXCEPTION 'submitted content attempts require a complete question snapshot';
    END IF;
END
$$;

ALTER TABLE public.content_attempts
    DROP CONSTRAINT IF EXISTS ck_content_attempts_review_question_snapshot;

ALTER TABLE public.content_attempts
    ADD CONSTRAINT ck_content_attempts_review_question_snapshot CHECK (
        (
            submitted_at IS NULL
            AND review_question_title IS NULL
            AND review_question_body IS NULL
            AND review_question_concept_ids IS NULL
            AND review_question_difficulty IS NULL
            AND review_question_meta IS NULL
            AND review_question_generated_by_student_id IS NULL
        )
        OR (
            review_question_title IS NOT NULL
            AND review_question_body IS NOT NULL
            AND review_question_concept_ids IS NOT NULL
            AND review_question_difficulty IS NOT NULL
            AND review_question_meta IS NOT NULL
        )
    );
