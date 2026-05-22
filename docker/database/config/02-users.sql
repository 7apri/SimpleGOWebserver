CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    role TEXT NOT NULL DEFAULT 'basic',
    username CITEXT NOT NULL,
    email    CITEXT NOT NULL,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    banner_url TEXT,
    bio VARCHAR(280),
    
    following_count INT NOT NULL DEFAULT 0 CHECK (following_count >= 0),
    followers_count INT NOT NULL DEFAULT 0 CHECK (followers_count >= 0),

    settings JSONB NOT NULL DEFAULT '{"lang":"en"}'::jsonb,

    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL
);
CREATE UNIQUE INDEX users_active_username_idx ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_active_email_idx ON users (email) WHERE deleted_at IS NULL;

CREATE TRIGGER update_user_modtime
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

CREATE TABLE user_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind   TEXT NOT NULL,
    secret TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX user_kind ON user_credentials (user_id, kind);
CREATE UNIQUE INDEX idx_user_single_credentials 
ON user_credentials (user_id, kind) 
WHERE kind IN ('passkey', 'totp');

CREATE TRIGGER update_user_credentials_modtime
    BEFORE UPDATE ON user_credentials
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();
