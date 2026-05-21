CREATE TABLE analytics (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    path TEXT NOT NULL,
    method TEXT NOT NULL,
    status INT NOT NULL,
    duration_micro BIGINT NOT NULL,
    ip INET,
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
