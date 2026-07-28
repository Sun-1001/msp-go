-- 0015: Structured image and document attachments for message-center messages.

ALTER TABLE public.conversation_messages
    ADD COLUMN attachments jsonb DEFAULT '[]'::jsonb NOT NULL;

ALTER TABLE public.question_thread_messages
    ADD COLUMN attachments jsonb DEFAULT '[]'::jsonb NOT NULL;

ALTER TABLE public.conversation_messages
    ADD CONSTRAINT ck_conversation_message_attachments_array
        CHECK (jsonb_typeof(attachments) = 'array');

ALTER TABLE public.question_thread_messages
    ADD CONSTRAINT ck_question_thread_message_attachments_array
        CHECK (jsonb_typeof(attachments) = 'array');
