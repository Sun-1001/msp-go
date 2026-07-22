ALTER TABLE public.llm_providers
    ADD COLUMN IF NOT EXISTS priority integer NOT NULL DEFAULT 0;

ALTER TABLE public.llm_providers
    ADD COLUMN IF NOT EXISTS weight integer NOT NULL DEFAULT 100;

ALTER TABLE public.agent_model_configs
    ADD COLUMN IF NOT EXISTS model_key character varying(100);

UPDATE public.agent_model_configs AS config
SET model_key = COALESCE(NULLIF(BTRIM(model.name), ''), model.model_id)
FROM public.llm_models AS model
WHERE config.model_id = model.id
  AND (config.model_key IS NULL OR BTRIM(config.model_key) = '');

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_llm_providers_priority_range'
          AND conrelid = 'public.llm_providers'::regclass
    ) THEN
        ALTER TABLE public.llm_providers
            ADD CONSTRAINT ck_llm_providers_priority_range
            CHECK (priority >= 0 AND priority <= 1000);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_llm_providers_weight_range'
          AND conrelid = 'public.llm_providers'::regclass
    ) THEN
        ALTER TABLE public.llm_providers
            ADD CONSTRAINT ck_llm_providers_weight_range
            CHECK (weight >= 1 AND weight <= 1000);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_agent_model_configs_model_key'
          AND conrelid = 'public.agent_model_configs'::regclass
    ) THEN
        ALTER TABLE public.agent_model_configs
            ADD CONSTRAINT ck_agent_model_configs_model_key
            CHECK (model_key IS NULL OR BTRIM(model_key) <> '');
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_llm_providers_active_priority
    ON public.llm_providers (priority DESC, id)
    WHERE is_active;

CREATE INDEX IF NOT EXISTS ix_llm_models_active_logical_name
    ON public.llm_models (name, provider_id)
    WHERE is_active;

CREATE INDEX IF NOT EXISTS ix_agent_model_configs_model_key
    ON public.agent_model_configs (model_key);
