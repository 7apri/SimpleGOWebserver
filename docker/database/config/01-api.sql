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
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE TRIGGER update_weather_cache_modtime
    BEFORE UPDATE ON weather_current_cache
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

CREATE TABLE weather_history (
    id BIGSERIAL PRIMARY KEY,
    location_id BIGINT NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    recorded_date DATE NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
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
CREATE TRIGGER update_weather_history_modtime
    BEFORE UPDATE ON weather_history
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();
