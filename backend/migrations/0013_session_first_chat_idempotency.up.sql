-- Persist immutable first-chat identity and a recoverable generation claim.

CREATE TABLE public.session_first_chat_requests (
    session_id character varying(36) PRIMARY KEY
        REFERENCES public.learning_sessions(id) ON DELETE CASCADE,
    request_hash character(64) NOT NULL,
    assistant_message_id character varying(36) NOT NULL,
    claim_token character varying(36) NOT NULL,
    claim_expires_at timestamp without time zone NOT NULL,
    completed_at timestamp without time zone,
    CONSTRAINT uq_session_first_chat_requests_assistant_message
        UNIQUE (assistant_message_id),
    CONSTRAINT ck_session_first_chat_requests_request_hash
        CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_session_first_chat_requests_assistant_message_id
        CHECK (assistant_message_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    CONSTRAINT ck_session_first_chat_requests_claim_token
        CHECK (claim_token ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
);
