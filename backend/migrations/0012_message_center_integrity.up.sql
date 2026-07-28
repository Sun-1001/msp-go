-- 0012: Reconcile the message-center schema after migration version 0011 was
-- assigned to student learning goals upstream.

-- Some development databases recorded the earlier, uncommitted message-center
-- migration as version 0011. In those databases the canonical upstream 0011 is
-- skipped by version, so repair its schema here without changing that migration.
CREATE TABLE IF NOT EXISTS public.student_learning_goals (
    student_id character varying(36) PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    target_node_id character varying(36) NOT NULL REFERENCES public.knowledge_nodes(id) ON DELETE CASCADE,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_student_learning_goals_target_node_id
    ON public.student_learning_goals USING btree (target_node_id);

-- Keep private-message archives independent for students and teachers.
-- Historical is_archived values were written only by the student endpoint, so
-- they are copied to student_archived and teachers retain visibility.
ALTER TABLE public.conversations
    ADD COLUMN IF NOT EXISTS student_archived boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS teacher_archived boolean NOT NULL DEFAULT false;

UPDATE public.conversations
SET student_archived = is_archived
WHERE is_archived = true;

CREATE INDEX IF NOT EXISTS ix_conversations_student_participant_archived
    ON public.conversations USING btree (student_id, student_archived, last_message_at DESC);

CREATE INDEX IF NOT EXISTS ix_conversations_teacher_participant_archived
    ON public.conversations USING btree (teacher_id, teacher_archived, last_message_at DESC);

-- Track student acknowledgement of teacher replies in Q&A threads. Only
-- backfill when this migration creates the column: replaying the UPDATE on a
-- partially migrated database would incorrectly mark newer replies as read.
DO $$
DECLARE
    read_column_missing boolean;
BEGIN
    SELECT NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'question_thread_messages'
          AND column_name = 'read_at'
    ) INTO read_column_missing;

    IF read_column_missing THEN
        ALTER TABLE public.question_thread_messages
            ADD COLUMN read_at timestamp without time zone;

        UPDATE public.question_thread_messages
        SET read_at = created_at
        WHERE sender_role = 'teacher';
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS ix_question_thread_messages_teacher_unread
    ON public.question_thread_messages USING btree (thread_id)
    WHERE sender_role = 'teacher' AND read_at IS NULL;

-- Preserve message-center audience and participant semantics.

-- Block participant archive writes until calibration and the compatibility
-- trigger are both installed. Before this migration, only the legacy field is writable by
-- the deployed application, so it is the authoritative value.
LOCK TABLE public.conversations IN SHARE ROW EXCLUSIVE MODE;

-- Keep the legacy student archive field compatible with both old and new
-- application instances during a rolling deployment.
UPDATE public.conversations
SET student_archived = is_archived
WHERE is_archived IS DISTINCT FROM student_archived;

CREATE OR REPLACE FUNCTION public.sync_conversation_student_archive()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.is_archived IS DISTINCT FROM NEW.student_archived THEN
            -- Either an old or new writer may provide the non-default value.
            NEW.is_archived := NEW.is_archived OR NEW.student_archived;
            NEW.student_archived := NEW.is_archived;
        END IF;
    ELSIF NEW.is_archived IS DISTINCT FROM OLD.is_archived
          AND NEW.student_archived IS NOT DISTINCT FROM OLD.student_archived THEN
        NEW.student_archived := NEW.is_archived;
    ELSIF NEW.student_archived IS DISTINCT FROM OLD.student_archived
          AND NEW.is_archived IS NOT DISTINCT FROM OLD.is_archived THEN
        NEW.is_archived := NEW.student_archived;
    ELSIF NEW.is_archived IS DISTINCT FROM NEW.student_archived THEN
        NEW.is_archived := NEW.student_archived;
    END IF;
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_sync_conversation_student_archive ON public.conversations;
CREATE TRIGGER trg_sync_conversation_student_archive
BEFORE INSERT OR UPDATE OF is_archived, student_archived
ON public.conversations
FOR EACH ROW
EXECUTE FUNCTION public.sync_conversation_student_archive();

-- Recipient rows are immutable publication-time membership snapshots. The
-- student identifier intentionally has no users FK, so later account deletion
-- cannot silently change historical totals or recipient names.
CREATE TABLE IF NOT EXISTS public.notice_recipients (
    notice_id character varying(36) NOT NULL
        REFERENCES public.notices(id) ON DELETE CASCADE,
    student_id character varying(36) NOT NULL,
    recipient_name character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT pk_notice_recipients PRIMARY KEY (notice_id, student_id)
);

CREATE INDEX IF NOT EXISTS ix_notice_recipients_student_notice
    ON public.notice_recipients USING btree (student_id, notice_id);

-- Prevent old application instances from publishing between the historical
-- backfill and trigger installation. Also keep confirmation rows stable while
-- their ownership is moved from users to the immutable recipient snapshot.
LOCK TABLE public.notices IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE public.notice_confirmations IN SHARE ROW EXCLUSIVE MODE;

-- Preserve notice display metadata independently of mutable class and teacher
-- records. New and rolling old application instances still publish with a
-- concrete class_id; the trigger snapshots the class name before that class can
-- later be disbanded.
ALTER TABLE public.notices
    ADD COLUMN IF NOT EXISTS class_name character varying(200);

UPDATE public.notices n
SET class_name = c.name
FROM public.classes c
WHERE c.id = n.class_id
  AND n.class_name IS NULL;

CREATE OR REPLACE FUNCTION public.snapshot_notice_class_name()
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

DROP TRIGGER IF EXISTS trg_snapshot_notice_class_name ON public.notices;
CREATE TRIGGER trg_snapshot_notice_class_name
BEFORE INSERT OR UPDATE OF class_id ON public.notices
FOR EACH ROW
EXECUTE FUNCTION public.snapshot_notice_class_name();

ALTER TABLE public.notices
    ALTER COLUMN class_name SET NOT NULL,
    ALTER COLUMN class_id DROP NOT NULL,
    ALTER COLUMN teacher_id DROP NOT NULL,
    DROP CONSTRAINT IF EXISTS notices_class_id_fkey,
    DROP CONSTRAINT IF EXISTS notices_teacher_id_fkey;

ALTER TABLE public.notices
    ADD CONSTRAINT notices_class_id_fkey
        FOREIGN KEY (class_id) REFERENCES public.classes(id) ON DELETE SET NULL
        NOT VALID,
    ADD CONSTRAINT notices_teacher_id_fkey
        FOREIGN KEY (teacher_id) REFERENCES public.users(id) ON DELETE SET NULL
        NOT VALID;

ALTER TABLE public.notices
    VALIDATE CONSTRAINT notices_class_id_fkey,
    VALIDATE CONSTRAINT notices_teacher_id_fkey;

-- Preserve every historical confirmation, including confirmations by students
-- who are no longer enrolled.
INSERT INTO public.notice_recipients (notice_id, student_id, recipient_name, created_at)
SELECT n.id, nc.student_id, COALESCE(u.display_name, u.username), n.created_at
FROM public.notice_confirmations nc
JOIN public.notices n ON n.id = nc.notice_id
JOIN public.users u ON u.id = nc.student_id
ON CONFLICT (notice_id, student_id) DO NOTHING;

-- Historical audience membership cannot be reconstructed exactly. Current
-- active enrollments are the least-surprising safe backfill for unconfirmed
-- recipients and match the publication rule used after this migration. Skip
-- this lossy backfill when an earlier uncommitted integrity migration already
-- created an immutable snapshot; replaying it would add students who enrolled
-- after publication.
INSERT INTO public.notice_recipients (notice_id, student_id, recipient_name, created_at)
SELECT n.id, e.student_id, COALESCE(u.display_name, u.username), n.created_at
FROM public.notices n
JOIN public.class_enrollments e ON e.class_id = n.class_id
JOIN public.users u ON u.id = e.student_id
WHERE u.is_active = true
  AND u.role = 'STUDENT'::public.userrole
  AND NOT EXISTS (
      SELECT 1
      FROM public.go_schema_migrations gsm
      WHERE gsm.version IN (13, 14)
        AND gsm.name = 'message_center_integrity'
  )
ON CONFLICT (notice_id, student_id) DO NOTHING;

-- Old application instances only insert the notice row. Snapshot recipients in
-- the database as well so rolling deployments cannot create invisible notices.
CREATE OR REPLACE FUNCTION public.snapshot_notice_recipients()
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

DROP TRIGGER IF EXISTS trg_snapshot_notice_recipients ON public.notices;
CREATE TRIGGER trg_snapshot_notice_recipients
AFTER INSERT ON public.notices
FOR EACH ROW
EXECUTE FUNCTION public.snapshot_notice_recipients();

-- Confirmations belong to the publication-time audience, not to the mutable
-- user directory. Account deletion must not turn a historical confirmation
-- back into an unconfirmed recipient.
ALTER TABLE public.notice_confirmations
    DROP CONSTRAINT IF EXISTS notice_confirmations_student_id_fkey,
    DROP CONSTRAINT IF EXISTS notice_confirmations_recipient_fkey;

ALTER TABLE public.notice_confirmations
    ADD CONSTRAINT notice_confirmations_recipient_fkey
    FOREIGN KEY (notice_id, student_id)
    REFERENCES public.notice_recipients (notice_id, student_id)
    ON DELETE CASCADE
    NOT VALID;

ALTER TABLE public.notice_confirmations
    VALIDATE CONSTRAINT notice_confirmations_recipient_fkey;

-- Keep a relational class reference while retaining class_name as the display
-- snapshot for existing rows and clients. On databases that already ran an
-- earlier uncommitted integrity migration, do not rebind NULL rows: their
-- original class may have been deleted and a later class may now share the
-- same teacher/name pair. On other databases, only classes that existed when
-- the thread was created are eligible, so a deleted class cannot be replaced
-- by a subsequently created class with the same teacher/name pair.
ALTER TABLE public.question_threads
    ADD COLUMN IF NOT EXISTS class_id character varying(36);

WITH unambiguous_matches AS (
    SELECT qt.id, min(c.id) AS class_id
    FROM public.question_threads qt
    JOIN public.classes c
      ON c.teacher_id = qt.teacher_id
     AND c.name = qt.class_name
     AND c.created_at <= qt.created_at
    WHERE qt.class_id IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM public.go_schema_migrations gsm
          WHERE gsm.version IN (13, 14)
            AND gsm.name = 'message_center_integrity'
      )
    GROUP BY qt.id
    HAVING count(*) = 1
)
UPDATE public.question_threads qt
SET class_id = matches.class_id
FROM unambiguous_matches matches
WHERE qt.id = matches.id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'public.question_threads'::regclass
          AND conname = 'question_threads_class_id_fkey'
    ) THEN
        ALTER TABLE public.question_threads
            ADD CONSTRAINT question_threads_class_id_fkey
            FOREIGN KEY (class_id) REFERENCES public.classes(id) ON DELETE SET NULL
            NOT VALID;
    END IF;
END
$$;

ALTER TABLE public.question_threads
    VALIDATE CONSTRAINT question_threads_class_id_fkey;

CREATE INDEX IF NOT EXISTS ix_question_threads_teacher_class_status_updated
    ON public.question_threads USING btree (teacher_id, class_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS ix_question_threads_student_teacher_updated
    ON public.question_threads USING btree (student_id, teacher_id, updated_at DESC);
