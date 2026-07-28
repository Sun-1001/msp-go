-- Generated from Python Alembic head 0019_performance_indexes_phase3.
-- Compact Go-owned one-step schema migration for a clean database.

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;
CREATE SCHEMA IF NOT EXISTS public;

-- Types
CREATE TYPE public.aclpermission AS ENUM (
    'EDITOR',
    'ADMIN'
);
CREATE TYPE public.agenttype AS ENUM (
    'ORCHESTRATOR',
    'SOLVER',
    'DIAGNOSTICIAN',
    'TUTOR',
    'PLANNER'
);
CREATE TYPE public.assetkind AS ENUM (
    'VIDEO',
    'IMAGE',
    'PDF',
    'AUDIO',
    'ATTACHMENT'
);
CREATE TYPE public.auditaction AS ENUM (
    'CREATE',
    'UPDATE',
    'PUBLISH',
    'ARCHIVE',
    'DELETE',
    'BULK_IMPORT',
    'ACL_GRANT',
    'ACL_REVOKE'
);
CREATE TYPE public.contentstatus AS ENUM (
    'DRAFT',
    'PUBLISHED',
    'ARCHIVED'
);
CREATE TYPE public.contenttype AS ENUM (
    'PROBLEM',
    'NOTE',
    'VIDEO',
    'ARTICLE'
);
CREATE TYPE public.distancemetric AS ENUM (
    'COSINE',
    'L2',
    'IP'
);
CREATE TYPE public.errortype AS ENUM (
    'conceptual',
    'procedural',
    'logical',
    'symbolic',
    'calculation'
);
CREATE TYPE public.importjobkind AS ENUM (
    'PROBLEMS_BULK_UPSERT',
    'PROBLEMS_BULK_DELETE',
    'NOTES_BULK_UPSERT'
);
CREATE TYPE public.importjobstatus AS ENUM (
    'PENDING',
    'RUNNING',
    'SUCCEEDED',
    'FAILED',
    'CANCELLED'
);
CREATE TYPE public.messagerole AS ENUM (
    'USER',
    'ASSISTANT',
    'SYSTEM'
);
CREATE TYPE public.nodetype AS ENUM (
    'CONCEPT',
    'THEOREM',
    'METHOD',
    'PROBLEM',
    'MISCONCEPTION',
    'RESOURCE'
);
CREATE TYPE public.outboxeventtype AS ENUM (
    'CONTENT_CHANGED',
    'CONTENT_DELETED',
    'CONTENT_PUBLISHED',
    'CONTENT_ARCHIVED',
    'CONTENT_KNOWLEDGE_LINKED',
    'EMBEDDING_REQUIRED'
);
CREATE TYPE public.passwordresetstatus AS ENUM (
    'pending',
    'approved',
    'rejected'
);
CREATE TYPE public.relationtype AS ENUM (
    'HAS_PREREQUISITE',
    'IS_A_SPECIAL_CASE_OF',
    'USED_IN',
    'PRONE_TO_ERROR',
    'RELATED_TO'
);
CREATE TYPE public.securityeventtype AS ENUM (
    'login_failed',
    'login_anomaly',
    'request_error',
    'request_blocked',
    'service_error',
    'service_recovered',
    'daily_report',
    'config_changed'
);
CREATE TYPE public.securityseverity AS ENUM (
    'info',
    'warning',
    'error',
    'critical'
);
CREATE TYPE public.userrole AS ENUM (
    'STUDENT',
    'TEACHER',
    'ADMIN'
);
CREATE TYPE public.userstatus AS ENUM (
    'ACTIVE',
    'INACTIVE',
    'SUSPENDED'
);
SET default_tablespace = '';
SET default_table_access_method = heap;

-- Tables
CREATE TABLE public.agent_model_configs (
    id character varying(36) NOT NULL,
    agent_type character varying(50) NOT NULL,
    model_id character varying(36),
    temperature_override double precision,
    max_tokens_override integer,
    top_p_override double precision,
    timeout_override integer,
    max_retries_override integer,
    extra_config json NOT NULL,
    is_active boolean NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);
CREATE TABLE public.class_enrollments (
    id character varying(36) NOT NULL,
    class_id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    joined_at timestamp without time zone NOT NULL
);
CREATE TABLE public.classes (
    id character varying(36) NOT NULL,
    name character varying(200) NOT NULL,
    code character varying(12) NOT NULL,
    teacher_id character varying(36) NOT NULL,
    description text,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);
CREATE TABLE public.concept_bkt_params (
    concept_id character varying(128) NOT NULL,
    p_l0 double precision DEFAULT '0.25'::double precision NOT NULL,
    p_t double precision DEFAULT '0.12'::double precision NOT NULL,
    p_g double precision DEFAULT '0.2'::double precision NOT NULL,
    p_s double precision DEFAULT '0.1'::double precision NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.content_acl (
    content_id character varying(36) NOT NULL,
    teacher_id character varying(36) NOT NULL,
    permission public.aclpermission NOT NULL,
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.content_assets (
    id character varying(36) NOT NULL,
    content_id character varying(36) NOT NULL,
    kind public.assetkind NOT NULL,
    url character varying(1000) NOT NULL,
    meta json NOT NULL,
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.content_attempts (
    id character varying(36) NOT NULL,
    content_id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    student_answer text NOT NULL,
    student_steps json NOT NULL,
    is_correct boolean NOT NULL,
    score double precision NOT NULL,
    started_at timestamp without time zone NOT NULL,
    submitted_at timestamp without time zone,
    time_spent_seconds integer NOT NULL
);
CREATE TABLE public.content_audit (
    id character varying(36) NOT NULL,
    content_id character varying(36) NOT NULL,
    actor_user_id character varying(36) NOT NULL,
    action public.auditaction NOT NULL,
    at timestamp without time zone NOT NULL,
    diff json NOT NULL
);
CREATE TABLE public.contents (
    id character varying(36) NOT NULL,
    type public.contenttype NOT NULL,
    owner_teacher_id character varying(36) NOT NULL,
    status public.contentstatus NOT NULL,
    title character varying(500) NOT NULL,
    body text NOT NULL,
    difficulty double precision NOT NULL,
    concept_ids json NOT NULL,
    tags json NOT NULL,
    meta json NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    published_at timestamp without time zone,
    deleted_at timestamp without time zone
);
CREATE TABLE public.diagnosis_reports (
    id character varying(36) NOT NULL,
    attempt_id character varying(36) NOT NULL,
    error_step_index integer,
    bifurcation_point text,
    error_type public.errortype,
    error_subtype character varying(100),
    severity character varying(20) NOT NULL,
    related_concept_ids json NOT NULL,
    related_misconception_ids json NOT NULL,
    explanation text NOT NULL,
    suggestion text NOT NULL,
    recommended_resources json NOT NULL,
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.embedding_models (
    name character varying(100) NOT NULL,
    dim integer NOT NULL,
    distance public.distancemetric NOT NULL,
    is_active boolean NOT NULL,
    description text NOT NULL,
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.import_jobs (
    id character varying(36) NOT NULL,
    kind public.importjobkind NOT NULL,
    status public.importjobstatus NOT NULL,
    created_by character varying(36) NOT NULL,
    params json NOT NULL,
    stats json NOT NULL,
    created_at timestamp without time zone NOT NULL,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    error_message text
);
CREATE TABLE public.knowledge_nodes (
    id character varying(36) NOT NULL,
    name character varying(200) NOT NULL,
    name_en character varying(200),
    node_type public.nodetype NOT NULL,
    description text NOT NULL,
    chapter character varying(100),
    section character varying(100),
    difficulty double precision NOT NULL,
    latex_formula text,
    tags json NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);
CREATE TABLE public.knowledge_relations (
    id character varying(36) NOT NULL,
    source_id character varying(36) NOT NULL,
    target_id character varying(36) NOT NULL,
    relation_type public.relationtype NOT NULL,
    weight double precision NOT NULL,
    description text,
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.learning_sessions (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    is_active boolean NOT NULL,
    current_topic character varying(36),
    current_content_id character varying(36),
    contents_attempted json NOT NULL,
    concepts_discussed json NOT NULL,
    started_at timestamp without time zone NOT NULL,
    ended_at timestamp without time zone
);
CREATE TABLE public.llm_models (
    id character varying(36) NOT NULL,
    provider_id character varying(36) NOT NULL,
    name character varying(100) NOT NULL,
    model_id character varying(100) NOT NULL,
    default_temperature double precision NOT NULL,
    default_max_tokens integer,
    default_top_p double precision,
    default_timeout integer NOT NULL,
    default_max_retries integer NOT NULL,
    is_active boolean NOT NULL,
    is_default boolean NOT NULL,
    capabilities json NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.llm_providers (
    id character varying(36) NOT NULL,
    name character varying(100) NOT NULL,
    code character varying(50) NOT NULL,
    base_url character varying(500) NOT NULL,
    encrypted_api_key text NOT NULL,
    is_active boolean NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.outbox_events (
    id character varying(36) NOT NULL,
    type public.outboxeventtype NOT NULL,
    payload json NOT NULL,
    created_at timestamp without time zone NOT NULL,
    processed_at timestamp without time zone,
    retry_count integer NOT NULL,
    last_error text
);
CREATE TABLE public.password_reset_requests (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(100) NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    status public.passwordresetstatus DEFAULT 'pending'::public.passwordresetstatus NOT NULL,
    reviewed_by character varying(36),
    reviewed_at timestamp without time zone,
    reject_reason text,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.security_logs (
    id character varying(36) NOT NULL,
    event_type public.securityeventtype NOT NULL,
    severity public.securityseverity NOT NULL,
    title character varying(200) NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    ip_address character varying(45),
    user_id character varying(36),
    username character varying(50),
    metadata json DEFAULT '{}'::json NOT NULL,
    archived boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.session_messages (
    id character varying(36) NOT NULL,
    session_id character varying(36) NOT NULL,
    role public.messagerole NOT NULL,
    content text NOT NULL,
    agent_type public.agenttype,
    attachments json NOT NULL,
    related_concept_ids json NOT NULL,
    related_content_id character varying(36),
    created_at timestamp without time zone NOT NULL
);
CREATE TABLE public.student_concept_bkt_states (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    concept_id character varying(128) NOT NULL,
    mastery_prob double precision DEFAULT '0.25'::double precision NOT NULL,
    confidence double precision DEFAULT '0'::double precision NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    correct_count integer DEFAULT 0 NOT NULL,
    incorrect_count integer DEFAULT 0 NOT NULL,
    p_l0 double precision DEFAULT '0.25'::double precision NOT NULL,
    p_t double precision DEFAULT '0.12'::double precision NOT NULL,
    p_g double precision DEFAULT '0.2'::double precision NOT NULL,
    p_s double precision DEFAULT '0.1'::double precision NOT NULL,
    last_outcome boolean,
    last_attempt_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.student_profiles (
    id character varying(36) NOT NULL,
    student_id character varying(36) NOT NULL,
    mastery_vector json NOT NULL,
    error_tendency json NOT NULL,
    preferred_difficulty double precision NOT NULL,
    learning_pace double precision NOT NULL,
    total_exercises integer NOT NULL,
    correct_count integer NOT NULL,
    total_study_time_minutes integer NOT NULL,
    recent_concepts json NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    portrait_content text,
    portrait_generated_at timestamp without time zone,
    portrait_version integer DEFAULT 0 NOT NULL
);
CREATE TABLE public.system_settings (
    key character varying(100) NOT NULL,
    value text NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.user_favorites (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    content_id character varying(36) NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);
CREATE TABLE public.users (
    id character varying(36) NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(100) NOT NULL,
    hashed_password character varying(255) NOT NULL,
    role public.userrole NOT NULL,
    display_name character varying(100),
    avatar_url character varying(500),
    is_active boolean NOT NULL,
    status public.userstatus NOT NULL,
    last_login_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);
CREATE TABLE public.xidian_accounts (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    username character varying(50) NOT NULL,
    encrypted_password text NOT NULL,
    is_postgraduate boolean,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    last_verified_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    session_cookies json,
    cookies_updated_at timestamp without time zone
);
CREATE TABLE public.xidian_snapshots (
    id character varying(36) NOT NULL,
    user_id character varying(36) NOT NULL,
    data_type character varying(20) NOT NULL,
    semester_code character varying(20),
    payload json DEFAULT '{}'::json NOT NULL,
    fetched_at timestamp without time zone NOT NULL
);

-- Seed data
INSERT INTO public.alembic_version (version_num) VALUES ('0019_performance_indexes_phase3');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('b810b819-4bc9-4469-a22f-035b73dc138c', '极限', 'Limit', 'CONCEPT', '极限是微积分的基础概念，描述函数在某点附近的变化趋势。', '第一章', '1.1', 0.4, '\lim_{x \to a} f(x) = L', '["基础概念", "微积分"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('88dd49ff-dd01-4e13-ae7b-75f4d4e9360e', '导数', 'Derivative', 'CONCEPT', '导数描述函数在某点的瞬时变化率，是微分学的核心概念。', '第二章', '2.1', 0.5, 'f''(x) = \lim_{h \to 0} \frac{f(x+h) - f(x)}{h}', '["基础概念", "微分学"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('d018e683-6770-4b1a-a2df-d5537b141908', '洛必达法则', 'L''Hôpital''s Rule', 'THEOREM', '洛必达法则用于求解不定式极限，通过求导简化计算。', '第二章', '2.3', 0.6, '\lim_{x \to a} \frac{f(x)}{g(x)} = \lim_{x \to a} \frac{f''(x)}{g''(x)}', '["定理", "极限计算"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('19b3a7d3-53c0-462a-8487-40e5a8dd5e8b', '泰勒公式', 'Taylor Formula', 'THEOREM', '泰勒公式将函数展开为多项式形式，用于函数逼近和误差分析。', '第三章', '3.2', 0.7, 'f(x) = \sum_{n=0}^{\infty} \frac{f^{(n)}(a)}{n!}(x-a)^n', '["定理", "级数展开"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('92ae59c3-fdc2-4831-9703-214fe84d0534', '不定积分', 'Indefinite Integral', 'CONCEPT', '不定积分是导数的逆运算，求原函数的过程。', '第四章', '4.1', 0.5, '\int f(x) dx = F(x) + C', '["基础概念", "积分学"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('438f44d7-6e86-49bd-920e-e403d5f61010', '分部积分法', 'Integration by Parts', 'METHOD', '分部积分法用于求解复杂函数的积分，是处理乘积函数的导数的重要方法。', '第四章', '4.3', 0.6, '\int u dv = uv - \int v du', '["积分方法", "技巧"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('6874b84e-e8ba-404c-bb34-a41add97075a', '定积分', 'Definite Integral', 'CONCEPT', '定积分表示函数在区间上的累积量，具有明确的几何和物理意义。', '第五章', '5.1', 0.6, '\int_a^b f(x) dx', '["基础概念", "积分学"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_nodes (id, name, name_en, node_type, description, chapter, section, difficulty, latex_formula, tags, created_at, updated_at) VALUES ('99d5e22f-8058-4b13-9ab2-ed18bc21894b', '微分中值定理', 'Mean Value Theorem', 'THEOREM', '微分中值定理揭示了函数在区间上的平均变化率与某点导数的关系。', '第二章', '2.4', 0.6, 'f''(c) = \frac{f(b) - f(a)}{b - a}', '["定理", "微分学"]', '2026-04-18 12:43:27.62304', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('26933f42-a57e-4876-8c55-39dbce78612b', 'b810b819-4bc9-4469-a22f-035b73dc138c', '88dd49ff-dd01-4e13-ae7b-75f4d4e9360e', 'HAS_PREREQUISITE', 0.9, '极限是导数的前置知识', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('561961d7-4146-4919-b224-1569077074b9', '88dd49ff-dd01-4e13-ae7b-75f4d4e9360e', 'd018e683-6770-4b1a-a2df-d5537b141908', 'USED_IN', 0.8, '导数用于洛必达法则', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('cae21e36-aef8-47bb-8c3b-0701cbaa62ea', '88dd49ff-dd01-4e13-ae7b-75f4d4e9360e', '99d5e22f-8058-4b13-9ab2-ed18bc21894b', 'USED_IN', 0.85, '导数用于微分中值定理', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('6144f438-1efd-452f-8b78-c0b023337c71', 'd018e683-6770-4b1a-a2df-d5537b141908', '19b3a7d3-53c0-462a-8487-40e5a8dd5e8b', 'HAS_PREREQUISITE', 0.7, '洛必达法则是泰勒公式的前置知识', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('0ee0b3de-317e-4a4b-a265-a0f628513b78', '88dd49ff-dd01-4e13-ae7b-75f4d4e9360e', '92ae59c3-fdc2-4831-9703-214fe84d0534', 'HAS_PREREQUISITE', 0.9, '导数是不定积分的前置知识', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('f7f7e3b4-6f0e-4bc4-932b-6ab06fd93ffd', '92ae59c3-fdc2-4831-9703-214fe84d0534', '438f44d7-6e86-49bd-920e-e403d5f61010', 'USED_IN', 0.8, '不定积分用于分部积分法', '2026-04-18 12:43:27.62304');
INSERT INTO public.knowledge_relations (id, source_id, target_id, relation_type, weight, description, created_at) VALUES ('c7e08092-0791-4a2f-ad0d-aabf33243252', '92ae59c3-fdc2-4831-9703-214fe84d0534', '6874b84e-e8ba-404c-bb34-a41add97075a', 'HAS_PREREQUISITE', 0.95, '不定积分是定积分的前置知识', '2026-04-18 12:43:27.62304');
INSERT INTO public.system_settings (key, value, description, updated_at) VALUES ('allow_student_registration', 'true', '是否允许学生注册', '2026-04-18 12:43:27.62304');
INSERT INTO public.system_settings (key, value, description, updated_at) VALUES ('allow_teacher_registration', 'false', '是否允许教师注册', '2026-04-18 12:43:27.62304');

-- Primary and unique constraints
ALTER TABLE ONLY public.agent_model_configs
    ADD CONSTRAINT agent_model_configs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);
ALTER TABLE ONLY public.class_enrollments
    ADD CONSTRAINT class_enrollments_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.classes
    ADD CONSTRAINT classes_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.concept_bkt_params
    ADD CONSTRAINT concept_bkt_params_pkey PRIMARY KEY (concept_id);
ALTER TABLE ONLY public.content_acl
    ADD CONSTRAINT content_acl_pkey PRIMARY KEY (content_id, teacher_id);
ALTER TABLE ONLY public.content_assets
    ADD CONSTRAINT content_assets_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT content_attempts_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.content_audit
    ADD CONSTRAINT content_audit_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.contents
    ADD CONSTRAINT contents_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_attempt_id_key UNIQUE (attempt_id);
ALTER TABLE ONLY public.diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.embedding_models
    ADD CONSTRAINT embedding_models_pkey PRIMARY KEY (name);
ALTER TABLE ONLY public.import_jobs
    ADD CONSTRAINT import_jobs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.knowledge_nodes
    ADD CONSTRAINT knowledge_nodes_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.knowledge_relations
    ADD CONSTRAINT knowledge_relations_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.learning_sessions
    ADD CONSTRAINT learning_sessions_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.llm_models
    ADD CONSTRAINT llm_models_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.llm_providers
    ADD CONSTRAINT llm_providers_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.outbox_events
    ADD CONSTRAINT outbox_events_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.password_reset_requests
    ADD CONSTRAINT password_reset_requests_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.security_logs
    ADD CONSTRAINT security_logs_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.session_messages
    ADD CONSTRAINT session_messages_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.student_concept_bkt_states
    ADD CONSTRAINT student_concept_bkt_states_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT student_profiles_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT student_profiles_student_id_key UNIQUE (student_id);
ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_pkey PRIMARY KEY (key);
ALTER TABLE ONLY public.class_enrollments
    ADD CONSTRAINT uq_class_enrollment UNIQUE (class_id, student_id);
ALTER TABLE ONLY public.class_enrollments
    ADD CONSTRAINT uq_class_enrollment_student UNIQUE (student_id);
ALTER TABLE ONLY public.llm_models
    ADD CONSTRAINT uq_provider_model UNIQUE (provider_id, model_id);
ALTER TABLE ONLY public.student_concept_bkt_states
    ADD CONSTRAINT uq_student_concept_bkt_state UNIQUE (student_id, concept_id);
ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.xidian_accounts
    ADD CONSTRAINT xidian_accounts_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.xidian_snapshots
    ADD CONSTRAINT xidian_snapshots_pkey PRIMARY KEY (id);

-- Indexes
CREATE UNIQUE INDEX ix_agent_model_configs_agent_type ON public.agent_model_configs USING btree (agent_type);
CREATE INDEX ix_agent_model_configs_model_id ON public.agent_model_configs USING btree (model_id);
CREATE INDEX ix_bkt_concept ON public.student_concept_bkt_states USING btree (concept_id);
CREATE INDEX ix_bkt_student ON public.student_concept_bkt_states USING btree (student_id);
CREATE INDEX ix_bkt_updated_at ON public.student_concept_bkt_states USING btree (updated_at);
CREATE INDEX ix_class_enrollments_class_id ON public.class_enrollments USING btree (class_id);
CREATE UNIQUE INDEX ix_class_enrollments_student_id ON public.class_enrollments USING btree (student_id);
CREATE UNIQUE INDEX ix_classes_code ON public.classes USING btree (code);
CREATE INDEX ix_classes_teacher_id ON public.classes USING btree (teacher_id);
CREATE INDEX ix_content_acl_teacher_id ON public.content_acl USING btree (teacher_id);
CREATE INDEX ix_content_assets_content_id ON public.content_assets USING btree (content_id);
CREATE INDEX ix_content_attempts_content_id ON public.content_attempts USING btree (content_id);
CREATE INDEX ix_content_attempts_is_correct ON public.content_attempts USING btree (is_correct);
CREATE INDEX ix_content_attempts_student_id ON public.content_attempts USING btree (student_id);
CREATE INDEX ix_content_attempts_student_recent ON public.content_attempts USING btree (student_id, started_at DESC);
CREATE INDEX ix_content_attempts_student_submitted ON public.content_attempts USING btree (student_id, submitted_at);
CREATE INDEX ix_content_audit_actor_user_id ON public.content_audit USING btree (actor_user_id);
CREATE INDEX ix_content_audit_at ON public.content_audit USING btree (at);
CREATE INDEX ix_content_audit_content_id ON public.content_audit USING btree (content_id);
CREATE INDEX ix_contents_deleted_at ON public.contents USING btree (deleted_at);
CREATE INDEX ix_contents_owner_deleted ON public.contents USING btree (owner_teacher_id, deleted_at);
CREATE INDEX ix_contents_owner_status ON public.contents USING btree (owner_teacher_id, status);
CREATE INDEX ix_contents_owner_teacher_id ON public.contents USING btree (owner_teacher_id);
CREATE INDEX ix_contents_published ON public.contents USING btree (status, deleted_at);
CREATE INDEX ix_contents_status ON public.contents USING btree (status);
CREATE INDEX ix_contents_status_deleted_type ON public.contents USING btree (status, deleted_at, type);
CREATE INDEX ix_contents_teacher_published_difficulty ON public.contents USING btree (owner_teacher_id, status, type, difficulty) WHERE (deleted_at IS NULL);
CREATE INDEX ix_contents_type ON public.contents USING btree (type);
CREATE INDEX ix_diagnosis_reports_attempt_id ON public.diagnosis_reports USING btree (attempt_id);
CREATE INDEX ix_diagnosis_reports_error_type ON public.diagnosis_reports USING btree (error_type);
CREATE INDEX ix_diagnosis_reports_severity ON public.diagnosis_reports USING btree (severity);
CREATE INDEX ix_embedding_models_is_active ON public.embedding_models USING btree (is_active);
CREATE INDEX ix_import_jobs_created_by ON public.import_jobs USING btree (created_by);
CREATE INDEX ix_import_jobs_status ON public.import_jobs USING btree (status);
CREATE INDEX ix_knowledge_nodes_name ON public.knowledge_nodes USING btree (name);
CREATE INDEX ix_knowledge_relations_source_id ON public.knowledge_relations USING btree (source_id);
CREATE INDEX ix_knowledge_relations_target_id ON public.knowledge_relations USING btree (target_id);
CREATE INDEX ix_learning_sessions_student_active ON public.learning_sessions USING btree (student_id, is_active);
CREATE INDEX ix_learning_sessions_student_id ON public.learning_sessions USING btree (student_id);
CREATE INDEX ix_learning_sessions_student_started ON public.learning_sessions USING btree (student_id, started_at);
CREATE INDEX ix_llm_models_provider_id ON public.llm_models USING btree (provider_id);
CREATE UNIQUE INDEX ix_llm_providers_code ON public.llm_providers USING btree (code);
CREATE UNIQUE INDEX ix_llm_providers_name ON public.llm_providers USING btree (name);
CREATE INDEX ix_outbox_events_processed_at ON public.outbox_events USING btree (processed_at);
CREATE INDEX ix_outbox_events_type ON public.outbox_events USING btree (type);
CREATE INDEX ix_outbox_pending ON public.outbox_events USING btree (processed_at, created_at);
CREATE INDEX ix_password_reset_created_at ON public.password_reset_requests USING btree (created_at);
CREATE INDEX ix_password_reset_status ON public.password_reset_requests USING btree (status);
CREATE INDEX ix_password_reset_status_created ON public.password_reset_requests USING btree (status, created_at);
CREATE INDEX ix_password_reset_user_id ON public.password_reset_requests USING btree (user_id);
CREATE INDEX ix_security_logs_archived ON public.security_logs USING btree (archived);
CREATE INDEX ix_security_logs_archived_date ON public.security_logs USING btree (archived, created_at);
CREATE INDEX ix_security_logs_created_at ON public.security_logs USING btree (created_at);
CREATE INDEX ix_security_logs_date_type ON public.security_logs USING btree (created_at, event_type);
CREATE INDEX ix_security_logs_event_created ON public.security_logs USING btree (event_type, created_at);
CREATE INDEX ix_security_logs_event_type ON public.security_logs USING btree (event_type);
CREATE INDEX ix_security_logs_severity ON public.security_logs USING btree (severity);
CREATE INDEX ix_security_logs_user_created ON public.security_logs USING btree (user_id, created_at);
CREATE INDEX ix_security_logs_user_id ON public.security_logs USING btree (user_id);
CREATE INDEX ix_session_messages_session_id ON public.session_messages USING btree (session_id);
CREATE INDEX ix_student_concept_bkt_student ON public.student_concept_bkt_states USING btree (student_id);
CREATE INDEX ix_user_favorites_content_id ON public.user_favorites USING btree (content_id);
CREATE UNIQUE INDEX ix_user_favorites_user_content ON public.user_favorites USING btree (user_id, content_id);
CREATE INDEX ix_user_favorites_user_id ON public.user_favorites USING btree (user_id);
CREATE INDEX ix_users_active_role ON public.users USING btree (is_active, role);
CREATE INDEX ix_users_created_active ON public.users USING btree (created_at, is_active);
CREATE UNIQUE INDEX ix_users_email ON public.users USING btree (email);
CREATE INDEX ix_users_last_login_at ON public.users USING btree (last_login_at);
CREATE INDEX ix_users_role_active_created ON public.users USING btree (role, is_active, created_at) WHERE (is_active = true);
CREATE INDEX ix_users_status ON public.users USING btree (status);
CREATE INDEX ix_users_status_role ON public.users USING btree (status, role);
CREATE UNIQUE INDEX ix_users_username ON public.users USING btree (username);
CREATE UNIQUE INDEX ix_xidian_accounts_user_id ON public.xidian_accounts USING btree (user_id);
CREATE INDEX ix_xidian_accounts_username ON public.xidian_accounts USING btree (username);
CREATE INDEX ix_xidian_snapshots_user_type_fetched ON public.xidian_snapshots USING btree (user_id, data_type, fetched_at);

-- Foreign key constraints
ALTER TABLE ONLY public.agent_model_configs
    ADD CONSTRAINT agent_model_configs_model_id_fkey FOREIGN KEY (model_id) REFERENCES public.llm_models(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.class_enrollments
    ADD CONSTRAINT class_enrollments_class_id_fkey FOREIGN KEY (class_id) REFERENCES public.classes(id);
ALTER TABLE ONLY public.class_enrollments
    ADD CONSTRAINT class_enrollments_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.classes
    ADD CONSTRAINT classes_teacher_id_fkey FOREIGN KEY (teacher_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.content_acl
    ADD CONSTRAINT content_acl_content_id_fkey FOREIGN KEY (content_id) REFERENCES public.contents(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.content_acl
    ADD CONSTRAINT content_acl_teacher_id_fkey FOREIGN KEY (teacher_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.content_assets
    ADD CONSTRAINT content_assets_content_id_fkey FOREIGN KEY (content_id) REFERENCES public.contents(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT content_attempts_content_id_fkey FOREIGN KEY (content_id) REFERENCES public.contents(id);
ALTER TABLE ONLY public.content_attempts
    ADD CONSTRAINT content_attempts_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.contents
    ADD CONSTRAINT contents_owner_teacher_id_fkey FOREIGN KEY (owner_teacher_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_attempt_id_fkey FOREIGN KEY (attempt_id) REFERENCES public.content_attempts(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.import_jobs
    ADD CONSTRAINT import_jobs_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id);
ALTER TABLE ONLY public.knowledge_relations
    ADD CONSTRAINT knowledge_relations_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.knowledge_nodes(id);
ALTER TABLE ONLY public.knowledge_relations
    ADD CONSTRAINT knowledge_relations_target_id_fkey FOREIGN KEY (target_id) REFERENCES public.knowledge_nodes(id);
ALTER TABLE ONLY public.learning_sessions
    ADD CONSTRAINT learning_sessions_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.llm_models
    ADD CONSTRAINT llm_models_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES public.llm_providers(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.password_reset_requests
    ADD CONSTRAINT password_reset_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.security_logs
    ADD CONSTRAINT security_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.session_messages
    ADD CONSTRAINT session_messages_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.learning_sessions(id);
ALTER TABLE ONLY public.student_concept_bkt_states
    ADD CONSTRAINT student_concept_bkt_states_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.student_profiles
    ADD CONSTRAINT student_profiles_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_content_id_fkey FOREIGN KEY (content_id) REFERENCES public.contents(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.user_favorites
    ADD CONSTRAINT user_favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.xidian_accounts
    ADD CONSTRAINT xidian_accounts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
ALTER TABLE ONLY public.xidian_snapshots
    ADD CONSTRAINT xidian_snapshots_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
