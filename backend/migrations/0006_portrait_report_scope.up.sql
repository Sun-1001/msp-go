-- Bind generated portrait narratives to the learning-statistics window they describe.

ALTER TABLE public.student_profiles
    ADD COLUMN portrait_range character varying(20),
    ADD COLUMN portrait_snapshot_at timestamp without time zone;

ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT ck_student_profiles_portrait_range
    CHECK (portrait_range IS NULL OR portrait_range IN ('week', 'month', 'semester', 'all'));
