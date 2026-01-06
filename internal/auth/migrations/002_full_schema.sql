-- LLMux Full Authentication Schema
-- PostgreSQL migration for complete multi-tenant support (LiteLLM compatible)

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- Organizations Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_alias VARCHAR(255) NOT NULL,
    budget_id UUID,
    models JSONB DEFAULT '[]',
    max_budget DECIMAL(12, 4) DEFAULT 0,
    spend DECIMAL(12, 4) DEFAULT 0,
    model_spend JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

CREATE INDEX idx_organizations_alias ON organizations(organization_alias);

-- ============================================================================
-- Budgets Table (Reusable budget configurations)
-- ============================================================================
CREATE TABLE IF NOT EXISTS budgets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    max_budget DECIMAL(12, 4),
    soft_budget DECIMAL(12, 4),
    max_parallel_requests INT,
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    model_max_budget JSONB DEFAULT '{}',
    budget_duration VARCHAR(20),
    budget_reset_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

-- ============================================================================
-- Teams Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_alias VARCHAR(255),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    members JSONB DEFAULT '[]',
    admins JSONB DEFAULT '[]',
    members_with_roles JSONB DEFAULT '[]',
    
    -- Budget management
    max_budget DECIMAL(12, 4) DEFAULT 0,
    spend DECIMAL(12, 4) DEFAULT 0,
    model_max_budget JSONB DEFAULT '{}',
    model_spend JSONB DEFAULT '{}',
    budget_duration VARCHAR(20),
    budget_reset_at TIMESTAMP WITH TIME ZONE,
    budget_id UUID REFERENCES budgets(id) ON DELETE SET NULL,
    
    -- Rate limiting
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    max_parallel_requests INT,
    model_tpm_limit JSONB DEFAULT '{}',
    model_rpm_limit JSONB DEFAULT '{}',
    
    -- Access control
    models JSONB DEFAULT '[]',
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    blocked BOOLEAN DEFAULT false,
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_teams_organization_id ON teams(organization_id);
CREATE INDEX idx_teams_is_active ON teams(is_active);
CREATE INDEX idx_teams_budget_reset_at ON teams(budget_reset_at);

-- ============================================================================
-- Users Table (Internal Users)
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_alias VARCHAR(255),
    user_email VARCHAR(255) UNIQUE,
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    teams JSONB DEFAULT '[]',
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    user_role VARCHAR(50) DEFAULT 'internal_user',
    sso_id VARCHAR(255),
    
    -- Budget management
    max_budget DECIMAL(12, 4) DEFAULT 0,
    spend DECIMAL(12, 4) DEFAULT 0,
    model_max_budget JSONB DEFAULT '{}',
    model_spend JSONB DEFAULT '{}',
    budget_duration VARCHAR(20),
    budget_reset_at TIMESTAMP WITH TIME ZONE,
    
    -- Rate limiting
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    max_parallel_requests INT,
    
    -- Access control
    models JSONB DEFAULT '[]',
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(user_email);
CREATE INDEX idx_users_team_id ON users(team_id);
CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_users_sso_id ON users(sso_id);
CREATE INDEX idx_users_budget_reset_at ON users(budget_reset_at);

-- ============================================================================
-- API Keys Table (Verification Tokens)
-- ============================================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    key_name VARCHAR(255),
    key_alias VARCHAR(255) UNIQUE,
    
    -- Ownership
    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    budget_id UUID REFERENCES budgets(id) ON DELETE SET NULL,
    
    -- Access control
    allowed_models JSONB DEFAULT '[]',
    key_type VARCHAR(50) DEFAULT 'default',
    allowed_routes JSONB DEFAULT '[]',
    
    -- Rate limiting
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    max_parallel_requests INT,
    model_tpm_limit JSONB DEFAULT '{}',
    model_rpm_limit JSONB DEFAULT '{}',
    
    -- Budget management
    max_budget DECIMAL(12, 4) DEFAULT 0,
    soft_budget DECIMAL(12, 4),
    spent_budget DECIMAL(12, 4) DEFAULT 0,
    model_max_budget JSONB DEFAULT '{}',
    model_spend JSONB DEFAULT '{}',
    budget_duration VARCHAR(20),
    budget_reset_at TIMESTAMP WITH TIME ZONE,
    
    -- Key rotation (Enterprise feature)
    auto_rotate BOOLEAN DEFAULT false,
    rotation_interval VARCHAR(20),
    key_rotation_at TIMESTAMP WITH TIME ZONE,
    last_rotation_at TIMESTAMP WITH TIME ZONE,
    rotation_count INT DEFAULT 0,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    blocked BOOLEAN DEFAULT false,
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_by VARCHAR(255),
    updated_by VARCHAR(255)
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_key_alias ON api_keys(key_alias);
CREATE INDEX idx_api_keys_team_id ON api_keys(team_id);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX idx_api_keys_budget_reset_at ON api_keys(budget_reset_at);
CREATE INDEX idx_api_keys_key_rotation_at ON api_keys(key_rotation_at) WHERE auto_rotate = true;

-- ============================================================================
-- Team Memberships Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS team_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_role VARCHAR(50) DEFAULT 'user',
    spend DECIMAL(12, 4) DEFAULT 0,
    budget_id UUID REFERENCES budgets(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, team_id)
);

CREATE INDEX idx_team_memberships_user_id ON team_memberships(user_id);
CREATE INDEX idx_team_memberships_team_id ON team_memberships(team_id);

-- ============================================================================
-- End Users Table (External customers)
-- ============================================================================
CREATE TABLE IF NOT EXISTS end_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(255) NOT NULL UNIQUE,
    alias VARCHAR(255),
    spend DECIMAL(12, 4) DEFAULT 0,
    budget_id UUID REFERENCES budgets(id) ON DELETE SET NULL,
    blocked BOOLEAN DEFAULT false,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_end_users_user_id ON end_users(user_id);

-- ============================================================================
-- Usage Logs Table (Partitioned by month)
-- ============================================================================
CREATE TABLE IF NOT EXISTS usage_logs (
    id BIGSERIAL,
    request_id VARCHAR(64),
    api_key_id UUID,
    team_id UUID,
    organization_id UUID,
    user_id UUID,
    end_user_id VARCHAR(255),
    
    -- Request details
    model VARCHAR(100) NOT NULL,
    model_group VARCHAR(100),
    provider VARCHAR(50) NOT NULL,
    call_type VARCHAR(50) DEFAULT 'completion',
    
    -- Token usage
    prompt_tokens INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    
    -- Cost and performance
    spend DECIMAL(12, 6) DEFAULT 0,
    latency_ms INT DEFAULT 0,
    
    -- Status
    status_code INT,
    status VARCHAR(50),
    cache_hit VARCHAR(10),
    
    -- Metadata
    request_tags JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    
    -- Timestamps
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
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
CREATE INDEX idx_usage_logs_organization_id ON usage_logs(organization_id);
CREATE INDEX idx_usage_logs_model ON usage_logs(model);
CREATE INDEX idx_usage_logs_provider ON usage_logs(provider);
CREATE INDEX idx_usage_logs_start_time ON usage_logs(start_time);

-- ============================================================================
-- Daily Usage Aggregation Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS daily_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    date DATE NOT NULL,
    api_key_id UUID,
    team_id UUID,
    organization_id UUID,
    model VARCHAR(100),
    provider VARCHAR(50),
    
    -- Aggregated metrics
    prompt_tokens BIGINT DEFAULT 0,
    completion_tokens BIGINT DEFAULT 0,
    spend DECIMAL(12, 6) DEFAULT 0,
    api_requests BIGINT DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(date, api_key_id, team_id, organization_id, model, provider)
);

CREATE INDEX idx_daily_usage_date ON daily_usage(date);
CREATE INDEX idx_daily_usage_api_key_id ON daily_usage(api_key_id);
CREATE INDEX idx_daily_usage_team_id ON daily_usage(team_id);

-- ============================================================================
-- Triggers for updated_at
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_budgets_updated_at
    BEFORE UPDATE ON budgets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_team_memberships_updated_at
    BEFORE UPDATE ON team_memberships
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_end_users_updated_at
    BEFORE UPDATE ON end_users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Views for Analytics
-- ============================================================================

-- API Key usage summary
CREATE OR REPLACE VIEW api_key_usage_summary AS
SELECT 
    ak.id AS api_key_id,
    ak.key_name,
    ak.key_prefix,
    ak.team_id,
    t.team_alias,
    ak.organization_id,
    ak.max_budget,
    ak.spent_budget,
    ak.is_active,
    ak.blocked,
    COUNT(ul.id) AS total_requests,
    COALESCE(SUM(ul.total_tokens), 0) AS total_tokens,
    COALESCE(SUM(ul.spend), 0) AS total_cost,
    COALESCE(AVG(ul.latency_ms), 0) AS avg_latency_ms,
    MAX(ul.created_at) AS last_request_at
FROM api_keys ak
LEFT JOIN teams t ON ak.team_id = t.id
LEFT JOIN usage_logs ul ON ak.id = ul.api_key_id
WHERE ak.is_active = true
GROUP BY ak.id, ak.key_name, ak.key_prefix, ak.team_id, t.team_alias, 
         ak.organization_id, ak.max_budget, ak.spent_budget, ak.is_active, ak.blocked;

-- Team usage summary
CREATE OR REPLACE VIEW team_usage_summary AS
SELECT 
    t.id AS team_id,
    t.team_alias,
    t.organization_id,
    t.max_budget,
    t.spend AS spent_budget,
    t.is_active,
    t.blocked,
    COUNT(DISTINCT ak.id) AS api_key_count,
    COUNT(ul.id) AS total_requests,
    COALESCE(SUM(ul.total_tokens), 0) AS total_tokens,
    COALESCE(SUM(ul.spend), 0) AS total_cost
FROM teams t
LEFT JOIN api_keys ak ON t.id = ak.team_id AND ak.is_active = true
LEFT JOIN usage_logs ul ON ak.id = ul.api_key_id
WHERE t.is_active = true
GROUP BY t.id, t.team_alias, t.organization_id, t.max_budget, t.spend, t.is_active, t.blocked;

-- Organization usage summary
CREATE OR REPLACE VIEW organization_usage_summary AS
SELECT 
    o.id AS organization_id,
    o.organization_alias,
    o.max_budget,
    o.spend AS spent_budget,
    COUNT(DISTINCT t.id) AS team_count,
    COUNT(DISTINCT ak.id) AS api_key_count,
    COUNT(ul.id) AS total_requests,
    COALESCE(SUM(ul.total_tokens), 0) AS total_tokens,
    COALESCE(SUM(ul.spend), 0) AS total_cost
FROM organizations o
LEFT JOIN teams t ON o.id = t.organization_id AND t.is_active = true
LEFT JOIN api_keys ak ON t.id = ak.team_id AND ak.is_active = true
LEFT JOIN usage_logs ul ON ak.id = ul.api_key_id
GROUP BY o.id, o.organization_alias, o.max_budget, o.spend;
