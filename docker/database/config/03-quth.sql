CREATE TABLE refresh_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    remember_me BOOLEAN NOT NULL DEFAULT FALSE,
    token_hash CHAR(64) NOT NULL,
    device_name TEXT,
    user_agent  TEXT,
    ip_address  INET,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_sessions_user_device ON refresh_sessions (user_id, ip_address, device_name);

CREATE TYPE challenge_kind AS ENUM ('verify', 'reset', 'lock');

CREATE TABLE user_challenges (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    challenge_type challenge_kind NOT NULL,
    token_hash CHAR(64) NOT NULL,
    code_hash  CHAR(64) NOT NULL,
    attempts   SMALLINT DEFAULT 0 CHECK (attempts >= 0 AND attempts <= 10),
    expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, challenge_type)
);
CREATE INDEX idx_challenges_token ON user_challenges(token_hash);
CREATE INDEX idx_challenges_code ON user_challenges(code_hash);
CREATE TRIGGER update_user_challenges_modtime
    BEFORE UPDATE ON user_challenges
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();
