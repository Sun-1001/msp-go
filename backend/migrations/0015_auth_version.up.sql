-- Bind access and refresh tokens to persistent account authorization state so
-- password and status changes invalidate every previously issued session.

ALTER TABLE public.users
    ADD COLUMN auth_version bigint DEFAULT 1 NOT NULL;

ALTER TABLE ONLY public.users
    ADD CONSTRAINT ck_users_auth_version
    CHECK (auth_version >= 1);
