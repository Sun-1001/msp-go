-- Persist explicit student-started portrait actions independently from exercise writes.

CREATE TABLE public.student_portrait_actions (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    concept_id character varying(36) NOT NULL,
    action_type character varying(20) NOT NULL,
    target_count integer NOT NULL,
    baseline_attempt_count integer NOT NULL,
    started_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);

ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT student_portrait_actions_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT uq_student_portrait_action UNIQUE (student_id, concept_id, action_type);
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT ck_student_portrait_actions_type CHECK (action_type IN ('practice'));
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT ck_student_portrait_actions_target CHECK (target_count > 0);
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT ck_student_portrait_actions_baseline CHECK (baseline_attempt_count >= 0);
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT student_portrait_actions_student_id_fkey
    FOREIGN KEY (student_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.student_portrait_actions
    ADD CONSTRAINT student_portrait_actions_concept_id_fkey
    FOREIGN KEY (concept_id) REFERENCES public.knowledge_nodes(id) ON DELETE CASCADE;

CREATE INDEX ix_student_portrait_actions_student
    ON public.student_portrait_actions USING btree (student_id);
