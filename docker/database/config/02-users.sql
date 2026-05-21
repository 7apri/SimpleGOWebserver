CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    role TEXT NOT NULL DEFAULT 'basic',
    username CITEXT UNIQUE NOT NULL,
    email    CITEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    banner_url TEXT,
    
    following_count BIGINT,
    followers_count BIGINT,

    preferred_lang VARCHAR(5) NOT NULL DEFAULT 'en',
    units VARCHAR(10) NOT NULL DEFAULT 'metric',

    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL
);
CREATE INDEX idx_users_active ON users(id) WHERE deleted_at IS NULL;
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
