-- Read-only PostgreSQL search diagnostics for production-like data.
-- Usage: psql "$DATABASE_URL" -v search_term=calculus -f scripts/explain-search-hotspots.sql
-- Run during a low-traffic window. Each statement is capped to avoid an unbounded scan.

\if :{?search_term}
\else
\set search_term calculus
\endif

SET statement_timeout = '15s';

SELECT
    relname AS table_name,
    n_live_tup AS estimated_rows,
    seq_scan,
    idx_scan
FROM pg_stat_user_tables
WHERE schemaname = 'public'
  AND relname IN ('users', 'knowledge_nodes', 'contents')
ORDER BY relname;

SELECT extname, extversion
FROM pg_extension
WHERE extname = 'pg_trgm';

SELECT tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
  AND tablename IN ('users', 'knowledge_nodes', 'contents')
ORDER BY tablename, indexname;

-- Admin user search: admin_user_repository.go.
EXPLAIN (ANALYZE, BUFFERS, SETTINGS)
SELECT id
FROM public.users
WHERE role <> 'ADMIN'::public.userrole
  AND (
      username ILIKE '%' || :'search_term' || '%' OR
      email ILIKE '%' || :'search_term' || '%' OR
      display_name ILIKE '%' || :'search_term' || '%'
  )
ORDER BY created_at DESC
LIMIT 20;

-- Knowledge node search shared by admin and student graph views.
EXPLAIN (ANALYZE, BUFFERS, SETTINGS)
SELECT id
FROM public.knowledge_nodes
WHERE
    name ILIKE '%' || :'search_term' || '%' OR
    name_en ILIKE '%' || :'search_term' || '%' OR
    description ILIKE '%' || :'search_term' || '%'
ORDER BY created_at
LIMIT 100;

-- Published resource search: resource_repository.go.
EXPLAIN (ANALYZE, BUFFERS, SETTINGS)
SELECT c.id
FROM public.contents c
WHERE c.status = 'PUBLISHED'
  AND c.deleted_at IS NULL
  AND c.type IN ('VIDEO', 'ARTICLE')
  AND (
      c.title ILIKE '%' || :'search_term' || '%' OR
      c.meta->>'topic' ILIKE '%' || :'search_term' || '%'
  )
ORDER BY c.created_at DESC
LIMIT 20;

-- Question bank search. Production review should repeat this with a representative
-- owner_teacher_id predicate because per-teacher cardinality affects the chosen plan.
EXPLAIN (ANALYZE, BUFFERS, SETTINGS)
SELECT c.id
FROM public.contents c
WHERE c.deleted_at IS NULL
  AND c.type = 'PROBLEM'::public.contenttype
  AND (
      c.title ILIKE '%' || :'search_term' || '%' OR
      c.body ILIKE '%' || :'search_term' || '%'
  )
ORDER BY c.created_at DESC
LIMIT 20;

-- Add pg_trgm indexes only when representative terms show a material sequential scan
-- (high actual rows removed, buffer reads/hits, and latency) at production cardinality.
-- Index expressions must exactly match the repository predicates, including JSON text
-- expressions such as (meta->>'topic'). Re-run every plan after any candidate index.
