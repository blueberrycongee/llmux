-- LLMux Authentication Schema
-- PostgreSQL migration for API key authentication and multi-tenant support

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Teams table
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    max_budget DECIMAL(12, 4) DEFAULT 0,
    spent_budget DECIMAL(12, 4) DEFAULT 0,
    rate_limit INT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX idx_teams_is_active ON teams(is_active);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    role VARCHAR(50) DEFAULT 'member',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_team_id ON users(team_id);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    name VARCHAR(255),
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    allowed_models JSONB DEFAULT '[]',
    rate_limit INT DEFAULT 0,
    max_budget DECIMAL(12, 4) DEFAULT 0,
    spent_budget DECIMAL(12, 4) DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_team_id ON api_keys(team_id);
CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);

-- Usage logs table (partitioned by month for performance)
CREATE TABLE IF NOT EXISTS usage_logs (
    id BIGSERIAL,
    api_key_id UUID NOT NULL,
    team_id UUID,
    model VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    cost DECIMAL(12, 6) DEFAULT 0,
    latency_ms INT DEFAULT 0,
    status_code INT DEFAULT 200,
    request_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create partitions for the next 12 months
DO $$
DECLARE
    start_date DATE := DATE_TRUNC('month', CURRENT_DATE);
    end_date DATE;
    partition_name TEXT;
BEGIN
    FOR i IN 0..11 LOOP
        end_date := start_date + INTERVAL '1 month';
        partition_name := 'usage_logs_' || TO_CHAR(start_date, 'YYYY_MM');
        
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF usage_logs
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
        
        start_date := end_date;
    END LOOP;
END $$;

CREATE INDEX idx_usage_logs_api_key_id ON usage_logs(api_key_id);
CREATE INDEX idx_usage_logs_team_id ON usage_logs(team_id);
CREATE INDEX idx_usage_logs_model ON usage_logs(model);
CREATE INDEX idx_usage_logs_created_at ON usage_logs(created_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Monthly budget reset function (run via cron job)
CREATE OR REPLACE FUNCTION reset_monthly_budgets()
RETURNS void AS $$
BEGIN
    UPDATE teams SET spent_budget = 0;
    UPDATE api_keys SET spent_budget = 0;
END;
$$ LANGUAGE plpgsql;

-- View for API key usage summary
CREATE OR REPLACE VIEW api_key_usage_summary AS
SELECT 
    ak.id AS api_key_id,
    ak.name AS api_key_name,
    ak.team_id,
    t.name AS team_name,
    COUNT(ul.id) AS total_requests,
    COALESCE(SUM(ul.total_tokens), 0) AS total_tokens,
    COALESCE(SUM(ul.cost), 0) AS total_cost,
    COALESCE(AVG(ul.latency_ms), 0) AS avg_latency_ms,
    MAX(ul.created_at) AS last_request_at
FROM api_keys ak
LEFT JOIN teams t ON ak.team_id = t.id
LEFT JOIN usage_logs ul ON ak.id = ul.api_key_id
WHERE ak.is_active = true
GROUP BY ak.id, ak.name, ak.team_id, t.name;

-- View for team usage summary
CREATE OR REPLACE VIEW team_usage_summary AS
SELECT 
    t.id AS team_id,
    t.name AS team_name,
    t.max_budget,
    t.spent_budget,
    COUNT(DISTINCT ak.id) AS api_key_count,
    COUNT(ul.id) AS total_requests,
    COALESCE(SUM(ul.total_tokens), 0) AS total_tokens,
    COALESCE(SUM(ul.cost), 0) AS total_cost
FROM teams t
LEFT JOIN api_keys ak ON t.id = ak.team_id AND ak.is_active = true
LEFT JOIN usage_logs ul ON ak.id = ul.api_key_id
WHERE t.is_active = true
GROUP BY t.id, t.name, t.max_budget, t.spent_budget;
