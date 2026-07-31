-- Provide a portrait-only optimistic concurrency token for generate/clear races.

ALTER TABLE public.student_profiles
    ADD COLUMN portrait_revision bigint DEFAULT 0 NOT NULL;

ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT ck_student_profiles_portrait_revision
    CHECK (portrait_revision >= 0);
