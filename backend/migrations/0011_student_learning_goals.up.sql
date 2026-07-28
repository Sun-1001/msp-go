-- 0011: Student learning goals
-- Stores one active knowledge-node target per student.

CREATE TABLE public.student_learning_goals (
    student_id character varying(36) PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    target_node_id character varying(36) NOT NULL REFERENCES public.knowledge_nodes(id) ON DELETE CASCADE,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);

CREATE INDEX ix_student_learning_goals_target_node_id
    ON public.student_learning_goals USING btree (target_node_id);
