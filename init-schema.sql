CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;

CREATE TABLE IF NOT EXISTS locations (
    id BIGSERIAL PRIMARY KEY,
    city_name TEXT NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', coalesce(city_name, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(state, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(country, '')), 'C')
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

CREATE INDEX IF NOT EXISTS idx_locations_geom ON locations USING GIST (geom);

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_city_no_state
ON locations (city_name, country) 
WHERE state IS NULL;
CREATE INDEX IF NOT EXISTS idx_locations_fts_vector ON locations USING GIN (search_vector);

CREATE INDEX IF NOT EXISTS idx_locations_city_trgm ON locations USING GIN (city_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_local_names_en_trgm ON locations USING GIN 
  ((local_names->>'en') gin_trgm_ops);

CREATE TABLE IF NOT EXISTS weather_current_cache (
    location_id BIGINT PRIMARY KEY REFERENCES locations(id) ON DELETE CASCADE,
    full_data JSONB NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS weather_history (
    id BIGSERIAL PRIMARY KEY,
    location_id BIGINT REFERENCES locations(id) ON DELETE CASCADE,
    recorded_date DATE NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    update_count INTEGER DEFAULT 1,
    uvi_avg FLOAT NOT NULL,
    pressure_avg FLOAT NOT NULL,
    wind_speed_avg FLOAT NOT NULL,
    temp_avg FLOAT NOT NULL,
    temp_avg_feel FLOAT NOT NULL,
    weather_description TEXT NOT NULL,                  
    raw_data JSONB,                
    UNIQUE(location_id, recorded_date)
);

CREATE INDEX IF NOT EXISTS idx_history_loc_date ON weather_history (location_id, recorded_date DESC);

CREATE TABLE IF NOT EXISTS users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    provider TEXT DEFAULT 'email',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email_username ON users (email, username);

CREATE TABLE IF NOT EXISTS refresh_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
