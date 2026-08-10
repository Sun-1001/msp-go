-- Retire Top P from the active AI configuration and align generated model
-- defaults with the current Cherry Studio assistant/runtime values.

UPDATE public.llm_models
SET default_temperature = CASE
        WHEN default_temperature = 0.7 THEN 1.0
        ELSE default_temperature
    END,
    default_max_tokens = COALESCE(default_max_tokens, 4096),
    default_top_p = NULL,
    default_timeout = CASE
        WHEN default_timeout = 60 THEN 1800
        ELSE default_timeout
    END,
    default_max_retries = CASE
        WHEN default_max_retries = 2 THEN 3
        ELSE default_max_retries
    END
WHERE default_temperature = 0.7
   OR default_max_tokens IS NULL
   OR default_top_p IS NOT NULL
   OR default_timeout = 60
   OR default_max_retries = 2;

UPDATE public.agent_model_configs
SET top_p_override = NULL
WHERE top_p_override IS NOT NULL;
