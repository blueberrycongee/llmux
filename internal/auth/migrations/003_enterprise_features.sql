-- Add missing tables for enterprise features
-- Organization Memberships and Audit Logs

-- ============================================================================
-- Organization Memberships Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS organization_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_role VARCHAR(50) DEFAULT 'member',
    spend DECIMAL(12, 4) DEFAULT 0,
    budget_id UUID REFERENCES budgets(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, organization_id)
);

CREATE INDEX idx_organization_memberships_user_id ON organization_memberships(user_id);
CREATE INDEX idx_organization_memberships_organization_id ON organization_memberships(organization_id);

-- ============================================================================
-- Audit Logs Table
-- ============================================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Actor (who performed the action)
    actor_id VARCHAR(255) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_email VARCHAR(255),
    actor_ip VARCHAR(50),
    
    -- Action details
    action VARCHAR(50) NOT NULL,
    object_type VARCHAR(50) NOT NULL,
    object_id VARCHAR(255) NOT NULL,
    
    -- Context
    team_id UUID,
    organization_id UUID,
    
    -- Change tracking
    before_value JSONB,
    after_value JSONB,
    diff JSONB,
    
    -- Request context
    request_id VARCHAR(64),
    user_agent TEXT,
    request_uri TEXT,
    
    -- Status
    success BOOLEAN NOT NULL DEFAULT true,
    error TEXT,
    
    -- Metadata
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_audit_logs_timestamp ON audit_logs (timestamp DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id, actor_type);
CREATE INDEX idx_audit_logs_action ON audit_logs (action);
CREATE INDEX idx_audit_logs_object ON audit_logs (object_type, object_id);
CREATE INDEX idx_audit_logs_team_id ON audit_logs (team_id) WHERE team_id IS NOT NULL;
CREATE INDEX idx_audit_logs_organization_id ON audit_logs (organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_audit_logs_success ON audit_logs (success);

-- Add trigger for organization_memberships updated_at
CREATE TRIGGER update_organization_memberships_updated_at
    BEFORE UPDATE ON organization_memberships
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
