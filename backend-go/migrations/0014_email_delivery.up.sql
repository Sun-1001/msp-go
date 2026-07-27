-- 0014: Administrator-managed notification email template overrides.

CREATE TABLE IF NOT EXISTS public.email_templates (
    event character varying(64) NOT NULL,
    locale character varying(16) NOT NULL,
    subject character varying(200) NOT NULL,
    html_body text NOT NULL,
    updated_by character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT pk_email_templates PRIMARY KEY (event, locale),
    CONSTRAINT ck_email_templates_event_not_blank CHECK (btrim(event) <> ''),
    CONSTRAINT ck_email_templates_locale_not_blank CHECK (btrim(locale) <> ''),
    CONSTRAINT ck_email_templates_subject_not_blank CHECK (btrim(subject) <> ''),
    CONSTRAINT ck_email_templates_html_body_not_blank CHECK (btrim(html_body) <> '')
);

CREATE INDEX IF NOT EXISTS ix_email_templates_updated_by
    ON public.email_templates USING btree (updated_by)
    WHERE updated_by IS NOT NULL;
