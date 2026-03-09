CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE locations (
    id BIGSERIAL PRIMARY KEY,
    city_name TEXT NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', city_name), 'A') ||
    setweight(to_tsvector('simple', coalesce(state, '')), 'B') ||
    setweight(to_tsvector('simple', country), 'C')
    ) STORED,
    geom geometry(Point, 4326) GENERATED ALWAYS AS (
        ST_SetSRID(ST_MakePoint(lon, lat), 4326)
    ) STORED,
    state TEXT,
    country CHAR(2) NOT NULL,
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL,
    local_names JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (city_name, country, state) 
);

CREATE INDEX idx_locations_geom ON locations USING GIST (geom);

CREATE UNIQUE INDEX idx_locations_unique_identity ON locations (city_name, country, COALESCE(state, ''));

CREATE INDEX idx_locations_fts_vector ON locations USING GIN (search_vector);

CREATE INDEX idx_locations_city_trgm ON locations USING GIN (city_name gin_trgm_ops);
CREATE INDEX idx_local_names_en_trgm ON locations USING GIN 
  ((local_names->>'en') gin_trgm_ops);

CREATE UNLOGGED TABLE weather_current_cache (
    location_id BIGINT PRIMARY KEY REFERENCES locations(id) ON DELETE CASCADE,
    full_data JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE weather_history (
    id BIGSERIAL PRIMARY KEY,
    location_id BIGINT NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    recorded_date DATE NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    update_count INTEGER DEFAULT 1,
    is_forecast BOOLEAN NOT NULL,
    
    wind_speed_avg FLOAT,
    pressure_avg FLOAT,
    uvi_avg FLOAT,    
    humidity_avg FLOAT,


    temp_avg FLOAT,
    temp_avg_feel FLOAT,
    
    temp_morning FLOAT,
    temp_day FLOAT,
    temp_evening FLOAT,
    temp_night FLOAT,
    temp_min FLOAT,
    temp_max FLOAT,
    
    weather_id SMALLINT CHECK (weather_id >= 0 AND weather_id <= 999),
    raw_data JSONB,                
    UNIQUE(location_id, recorded_date)
);

CREATE INDEX idx_history_loc_date ON weather_history (location_id, recorded_date DESC);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username CITEXT UNIQUE NOT NULL,
    email    CITEXT UNIQUE NOT NULL,
    role TEXT NOT NULL DEFAULT 'basic',
    preferred_lang VARCHAR(12) NOT NULL DEFAULT 'en',
    units VARCHAR(10) NOT NULL DEFAULT 'metric',
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

CREATE TABLE user_credentials (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind   TEXT NOT NULL,
    secret TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, kind),
    UNIQUE (kind, secret)
);

CREATE TABLE refresh_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

CREATE TABLE tasks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT,
    is_completed BOOLEAN DEFAULT FALSE,
    due_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tasks_user_id ON tasks(user_id);

CREATE TABLE analytics (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    path TEXT NOT NULL,
    method TEXT NOT NULL,
    status INT NOT NULL,
    duration_micro BIGINT NOT NULL,
    ip TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_analytics_created_at ON analytics(created_at);
CREATE INDEX idx_analytics_created_by ON analytics(user_id);
CREATE INDEX idx_analytics_path ON analytics(path);

CREATE TABLE logs (
    id BIGSERIAL PRIMARY KEY,
    level VARCHAR(10) NOT NULL,
    message TEXT NOT NULL,
    context JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_logs_created_at ON logs(created_at);
CREATE INDEX idx_logs_critical ON logs(created_at) 
WHERE level IN ('WARN', 'ERROR');
