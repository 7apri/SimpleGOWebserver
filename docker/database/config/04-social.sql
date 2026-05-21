CREATE TABLE posts (
    id UUID PRIMARY KEY,
    like_count INT NOT NULL DEFAULT 0 CHECK (following_count >= 0),
    repost_count INT NOT NULL DEFAULT 0 CHECK (following_count >= 0),
    replies_count INT NOT NULL DEFAULT 0 CHECK (following_count >= 0),
);
