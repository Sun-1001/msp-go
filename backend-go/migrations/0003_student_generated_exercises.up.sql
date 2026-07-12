-- Allow published AI exercises to be owned by the student who generated them.
ALTER TABLE public.contents
    ADD COLUMN generated_by_student_id character varying(36),
    ALTER COLUMN owner_teacher_id DROP NOT NULL;

ALTER TABLE public.contents
    ADD CONSTRAINT contents_generated_by_student_id_fkey
        FOREIGN KEY (generated_by_student_id) REFERENCES public.users(id),
    ADD CONSTRAINT ck_contents_exactly_one_owner
        CHECK ((owner_teacher_id IS NOT NULL) <> (generated_by_student_id IS NOT NULL)),
    ADD CONSTRAINT ck_contents_student_generated_problem
        CHECK (generated_by_student_id IS NULL OR type = 'PROBLEM'::public.contenttype);

CREATE INDEX ix_contents_student_generated
    ON public.contents USING btree (generated_by_student_id, created_at DESC)
    WHERE generated_by_student_id IS NOT NULL AND deleted_at IS NULL;
