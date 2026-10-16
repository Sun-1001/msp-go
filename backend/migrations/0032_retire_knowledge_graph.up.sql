-- Preserve only the curriculum dimensions needed by exercises and immutable
-- learning evidence. This is not a private-note migration: the old records have
-- no student owner. No application CRUD is exposed for this read-only catalog.
CREATE TABLE public.course_concepts (
    id varchar(36) PRIMARY KEY,
    name varchar(200) NOT NULL,
    name_en varchar(200),
    description text NOT NULL,
    chapter varchar(100),
    difficulty double precision NOT NULL,
    learning_enabled boolean NOT NULL
);
INSERT INTO public.course_concepts
SELECT id,name,name_en,description,chapter,difficulty,node_type IN ('CONCEPT','THEOREM','METHOD')
FROM public.knowledge_nodes;
CREATE INDEX course_concepts_name ON public.course_concepts(name);

CREATE TABLE public.course_prerequisites (
    concept_id varchar(36) NOT NULL REFERENCES public.course_concepts(id),
    prerequisite_id varchar(36) NOT NULL REFERENCES public.course_concepts(id),
    PRIMARY KEY (concept_id,prerequisite_id)
);
INSERT INTO public.course_prerequisites
SELECT DISTINCT source_id,target_id FROM public.knowledge_relations WHERE relation_type='HAS_PREREQUISITE';

DROP TABLE public.knowledge_relations;
-- Retarget existing foreign keys without deleting or rewriting learning facts.
-- Unknown dependent objects cause DROP to fail; CASCADE is deliberately absent.
DO $$
DECLARE constraint_row record;
BEGIN
    FOR constraint_row IN
        SELECT conrelid::regclass AS relation,conname,pg_get_constraintdef(oid) AS definition
        FROM pg_constraint WHERE contype='f' AND confrelid='public.knowledge_nodes'::regclass
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',constraint_row.relation,constraint_row.conname);
        EXECUTE format('ALTER TABLE %s ADD CONSTRAINT %I %s',constraint_row.relation,
            replace(constraint_row.conname,'knowledge_node','course_concept'),
            replace(constraint_row.definition,'knowledge_nodes','course_concepts'));
    END LOOP;
END $$;
ALTER TABLE public.forum_posts RENAME COLUMN knowledge_node_id TO concept_id;
ALTER TABLE public.student_learning_goals RENAME COLUMN target_node_id TO target_concept_id;
ALTER TABLE public.student_learning_goals RENAME CONSTRAINT student_learning_goals_target_node_id_fkey TO student_learning_goals_target_concept_id_fkey;
ALTER INDEX public.ix_student_learning_goals_target_node_id RENAME TO ix_student_learning_goals_target_concept_id;
DROP TABLE public.knowledge_nodes;
DROP TYPE public.nodetype;
DROP TYPE public.relationtype;
