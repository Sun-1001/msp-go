DROP TABLE IF EXISTS public.xidian_snapshots;

ALTER TABLE IF EXISTS public.xidian_accounts
    DROP COLUMN IF EXISTS encrypted_password,
    DROP COLUMN IF EXISTS is_postgraduate,
    DROP COLUMN IF EXISTS session_cookies,
    DROP COLUMN IF EXISTS cookies_updated_at;
