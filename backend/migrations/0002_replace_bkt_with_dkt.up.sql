-- Replace per-concept Bayesian Knowledge Tracing storage with DKT state storage.
ALTER TABLE public.student_concept_bkt_states RENAME TO student_concept_dkt_states;

ALTER TABLE public.student_concept_dkt_states
    RENAME CONSTRAINT student_concept_bkt_states_pkey TO student_concept_dkt_states_pkey;

ALTER TABLE public.student_concept_dkt_states
    RENAME CONSTRAINT uq_student_concept_bkt_state TO uq_student_concept_dkt_state;

ALTER TABLE public.student_concept_dkt_states
    RENAME CONSTRAINT student_concept_bkt_states_student_id_fkey TO student_concept_dkt_states_student_id_fkey;

ALTER INDEX public.ix_bkt_concept RENAME TO ix_dkt_concept;
ALTER INDEX public.ix_bkt_student RENAME TO ix_dkt_student;
ALTER INDEX public.ix_bkt_updated_at RENAME TO ix_dkt_updated_at;
ALTER INDEX public.ix_student_concept_bkt_student RENAME TO ix_student_concept_dkt_student;

ALTER TABLE public.student_concept_dkt_states
    DROP COLUMN p_l0,
    DROP COLUMN p_t,
    DROP COLUMN p_g,
    DROP COLUMN p_s,
    ADD COLUMN sequence_length integer DEFAULT 0 NOT NULL,
    ADD COLUMN attention_weight double precision DEFAULT 0 NOT NULL,
    ADD COLUMN last_exercise_id character varying;

DROP TABLE public.concept_bkt_params;
