CREATE TABLE posts (
    id UUID PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id     UUID REFERENCES posts(id) ON DELETE SET NULL,
    quote_id      UUID REFERENCES posts(id) ON DELETE SET NULL,

    content       VARCHAR(280),
    media_urls    TEXT[] NOT NULL DEFAULT '{}',

    likes_count    INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
    reposts_count  INT NOT NULL DEFAULT 0 CHECK (reposts_count >= 0),
    replies_count  INT NOT NULL DEFAULT 0 CHECK (replies_count >= 0),
    quotes_count   INT NOT NULL DEFAULT 0 CHECK (quotes_count >= 0),

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ DEFAULT NULL

    CONSTRAINT post_has_content CHECK (
        length(trim(content)) > 0 OR 
        array_length(media_urls, 1) > 0 OR 
        quote_id IS NOT NULL
    )
);

CREATE INDEX posts_user_id_idx ON posts (user_id) WHERE deleted_at IS NULL;
CREATE INDEX posts_parent_id_idx ON posts (parent_id) WHERE parent_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX posts_user_id_id_idx ON posts (user_id, id DESC);
CREATE INDEX posts_parent_created_idx ON posts (parent_id, id ASC);

CREATE TABLE follows (
    follower_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    followed_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK(follower_id != followed_id),
    PRIMARY KEY (follower_id, followed_id)
);
CREATE INDEX follows_followed_id_idx ON follows(followed_id);

CREATE TABLE reposts (
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    post_id    UUID REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, post_id)
);
CREATE INDEX reposts_post_id_idx ON reposts (post_id);

CREATE TABLE likes (
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    post_id    UUID REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, post_id)
);
CREATE INDEX likes_post_id_idx ON likes (post_id);