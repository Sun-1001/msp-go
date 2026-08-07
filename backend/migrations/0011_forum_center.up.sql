-- Global forum embedded in the message center.

CREATE TABLE public.forum_boards (
    id character varying(36) PRIMARY KEY,
    slug character varying(50) NOT NULL UNIQUE,
    name character varying(100) NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_forum_boards_slug CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    CONSTRAINT ck_forum_boards_name CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)
);

INSERT INTO public.forum_boards (id, slug, name, description, sort_order) VALUES
    ('f0000000-0000-4000-8000-000000000001', 'calculus', '高等数学', '极限、微分、积分与级数讨论', 10),
    ('f0000000-0000-4000-8000-000000000002', 'linear-algebra', '线性代数', '矩阵、向量空间与线性变换讨论', 20),
    ('f0000000-0000-4000-8000-000000000003', 'probability-statistics', '概率统计', '概率论与数理统计讨论', 30),
    ('f0000000-0000-4000-8000-000000000004', 'learning-methods', '学习方法', '学习计划、复习方法与经验交流', 40),
    ('f0000000-0000-4000-8000-000000000005', 'resources', '资源分享', '学习资料与工具分享', 50),
    ('f0000000-0000-4000-8000-000000000006', 'feedback', '平台反馈', '产品建议与问题反馈', 60);

CREATE TABLE public.forum_posts (
    id character varying(36) PRIMARY KEY,
    board_id character varying(36) NOT NULL REFERENCES public.forum_boards(id),
    author_id character varying(36) NOT NULL REFERENCES public.users(id),
    post_type character varying(20) NOT NULL,
    title character varying(200) NOT NULL,
    content text NOT NULL,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    knowledge_node_id character varying(36) REFERENCES public.knowledge_nodes(id) ON DELETE SET NULL,
    status character varying(20) DEFAULT 'open'::character varying NOT NULL,
    accepted_reply_id character varying(36),
    is_featured boolean DEFAULT false NOT NULL,
    featured_by character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    featured_at timestamp without time zone,
    view_count integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    deleted_at timestamp without time zone,
    CONSTRAINT ck_forum_posts_type CHECK (post_type IN ('question', 'discussion', 'resource')),
    CONSTRAINT ck_forum_posts_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT ck_forum_posts_content CHECK (char_length(btrim(content)) BETWEEN 1 AND 50000),
    CONSTRAINT ck_forum_posts_attachments CHECK (jsonb_typeof(attachments) = 'array'),
    CONSTRAINT ck_forum_posts_tags CHECK (jsonb_typeof(tags) = 'array'),
    CONSTRAINT ck_forum_posts_status CHECK (status IN ('open', 'resolved', 'hidden', 'deleted')),
    CONSTRAINT ck_forum_posts_view_count CHECK (view_count >= 0)
);

CREATE TABLE public.forum_replies (
    id character varying(36) PRIMARY KEY,
    post_id character varying(36) NOT NULL REFERENCES public.forum_posts(id) ON DELETE CASCADE,
    parent_reply_id character varying(36) REFERENCES public.forum_replies(id) ON DELETE SET NULL,
    author_id character varying(36) NOT NULL REFERENCES public.users(id),
    content text NOT NULL,
    attachments jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    updated_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    deleted_at timestamp without time zone,
    CONSTRAINT ck_forum_replies_content CHECK (char_length(btrim(content)) BETWEEN 1 AND 20000),
    CONSTRAINT ck_forum_replies_attachments CHECK (jsonb_typeof(attachments) = 'array'),
    CONSTRAINT ck_forum_replies_status CHECK (status IN ('active', 'hidden', 'deleted'))
);

ALTER TABLE public.forum_posts
    ADD CONSTRAINT fk_forum_posts_accepted_reply
    FOREIGN KEY (accepted_reply_id) REFERENCES public.forum_replies(id) ON DELETE SET NULL;

CREATE TABLE public.forum_post_likes (
    post_id character varying(36) NOT NULL REFERENCES public.forum_posts(id) ON DELETE CASCADE,
    user_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    PRIMARY KEY (post_id, user_id)
);

CREATE TABLE public.forum_post_favorites (
    post_id character varying(36) NOT NULL REFERENCES public.forum_posts(id) ON DELETE CASCADE,
    user_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    PRIMARY KEY (post_id, user_id)
);

CREATE TABLE public.forum_reports (
    id character varying(36) PRIMARY KEY,
    reporter_id character varying(36) NOT NULL REFERENCES public.users(id),
    target_type character varying(20) NOT NULL,
    target_id character varying(36) NOT NULL,
    reason character varying(50) NOT NULL,
    detail text DEFAULT ''::text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    reviewed_by character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    reviewed_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_forum_reports_target_type CHECK (target_type IN ('post', 'reply')),
    CONSTRAINT ck_forum_reports_reason CHECK (reason IN ('spam', 'abuse', 'answer_leak', 'misinformation', 'copyright', 'other')),
    CONSTRAINT ck_forum_reports_detail CHECK (char_length(detail) <= 2000),
    CONSTRAINT ck_forum_reports_status CHECK (status IN ('pending', 'resolved', 'dismissed'))
);

CREATE UNIQUE INDEX uq_forum_reports_pending_reporter_target
    ON public.forum_reports (reporter_id, target_type, target_id)
    WHERE status = 'pending';

CREATE TABLE public.forum_notifications (
    id character varying(36) PRIMARY KEY,
    recipient_id character varying(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    actor_id character varying(36) REFERENCES public.users(id) ON DELETE SET NULL,
    event_type character varying(30) NOT NULL,
    event_key character varying(160) NOT NULL,
    post_id character varying(36) REFERENCES public.forum_posts(id) ON DELETE CASCADE,
    reply_id character varying(36) REFERENCES public.forum_replies(id) ON DELETE CASCADE,
    title character varying(200) NOT NULL,
    summary character varying(500) NOT NULL,
    read_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai') NOT NULL,
    CONSTRAINT ck_forum_notifications_event_type CHECK (event_type IN ('reply', 'mention', 'like', 'accepted', 'featured')),
    CONSTRAINT uq_forum_notifications_event UNIQUE (recipient_id, event_key)
);

CREATE INDEX ix_forum_posts_board_updated
    ON public.forum_posts (board_id, updated_at DESC, id DESC)
    WHERE status IN ('open', 'resolved');
CREATE INDEX ix_forum_posts_type_updated
    ON public.forum_posts (post_type, updated_at DESC, id DESC)
    WHERE status IN ('open', 'resolved');
CREATE INDEX ix_forum_posts_featured_updated
    ON public.forum_posts (is_featured DESC, updated_at DESC, id DESC)
    WHERE status IN ('open', 'resolved');
CREATE INDEX ix_forum_posts_author
    ON public.forum_posts (author_id, created_at DESC);
CREATE INDEX ix_forum_replies_post_created
    ON public.forum_replies (post_id, created_at, id);
CREATE INDEX ix_forum_replies_parent
    ON public.forum_replies (parent_reply_id);
CREATE INDEX ix_forum_post_likes_user
    ON public.forum_post_likes (user_id, created_at DESC);
CREATE INDEX ix_forum_post_favorites_user
    ON public.forum_post_favorites (user_id, created_at DESC);
CREATE INDEX ix_forum_reports_status_created
    ON public.forum_reports (status, created_at DESC, id DESC);
CREATE INDEX ix_forum_notifications_recipient_unread
    ON public.forum_notifications (recipient_id, created_at DESC, id DESC)
    WHERE read_at IS NULL;
CREATE INDEX ix_forum_notifications_recipient_created
    ON public.forum_notifications (recipient_id, created_at DESC, id DESC);
