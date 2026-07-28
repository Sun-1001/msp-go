DROP INDEX IF EXISTS public.ix_llm_providers_code;

CREATE INDEX IF NOT EXISTS ix_llm_providers_code
    ON public.llm_providers USING btree (code);
