CREATE TABLE public.note_materials (
    id varchar(36) PRIMARY KEY,
    owner_id varchar(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    filename varchar(255) NOT NULL,
    content_type varchar(128) NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size > 0 AND byte_size <= 10485760),
    object_uri text NOT NULL,
    storage_key text NOT NULL,
    created_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
    UNIQUE (owner_id, id)
);
CREATE INDEX note_materials_owner_created ON public.note_materials(owner_id, created_at DESC);

CREATE TABLE public.notes (
    id varchar(36) PRIMARY KEY,
    owner_id varchar(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    title varchar(200) NOT NULL CHECK (length(btrim(title)) > 0),
    body text NOT NULL DEFAULT '' CHECK (length(body) <= 200000),
    material_id varchar(36),
    source_info varchar(1000) NOT NULL DEFAULT '',
    revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
    updated_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
    UNIQUE (owner_id, id),
    FOREIGN KEY (owner_id, material_id) REFERENCES public.note_materials(owner_id, id)
);
CREATE INDEX notes_owner_updated ON public.notes(owner_id, updated_at DESC, id);

CREATE TABLE public.note_links (
    owner_id varchar(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    source_id varchar(36) NOT NULL,
    target_id varchar(36) NOT NULL,
    source_type varchar(16) NOT NULL DEFAULT 'markdown' CHECK (source_type = 'markdown'),
    created_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
    PRIMARY KEY (owner_id, source_id, target_id),
    FOREIGN KEY (owner_id, source_id) REFERENCES public.notes(owner_id, id) ON DELETE CASCADE,
    FOREIGN KEY (owner_id, target_id) REFERENCES public.notes(owner_id, id) ON DELETE CASCADE,
    CHECK (source_id <> target_id)
);
CREATE INDEX note_links_incoming ON public.note_links(owner_id, target_id);
