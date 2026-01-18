CREATE TABLE IF NOT EXISTS locations (
    id SERIAL PRIMARY KEY,
    city_name TEXT NOT NULL,
    state TEXT,
    country TEXT NOT NULL,
    lat FLOAT NOT NULL,
    lon FLOAT NOT NULL,
    local_names JSONB,
    UNIQUE (city_name, state, country)
);

CREATE TABLE IF NOT EXISTS weather_current_cache (
    location_id INTEGER PRIMARY KEY REFERENCES locations(id) ON DELETE CASCADE,
    full_data JSONB,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS weather_history (
    id SERIAL PRIMARY KEY,
    location_id INTEGER REFERENCES locations(id) ON DELETE CASCADE,
    recorded_at TIMESTAMP NOT NULL,
    temp_day FLOAT NOT NULL,       
    weather_description TEXT NOT NULL,                  
    raw_data JSONB,                
    UNIQUE(location_id, recorded_at)
);

CREATE INDEX IF NOT EXISTS idx_city_name ON locations (city_name);

CREATE INDEX IF NOT EXISTS idx_current_loc ON weather_current_cache (location_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_history_loc_date ON weather_history (location_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_history_raw_data ON weather_history USING GIN (raw_data);
