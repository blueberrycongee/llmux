-- LLMux Invitation Links (LiteLLM compatible)
-- Adds persistence for invitation endpoints in distributed mode.

CREATE TABLE IF NOT EXISTS invitation_links (
    id UUID PRIMARY KEY,
    token_hash VARCHAR(255) NOT NULL UNIQUE,

    team_id UUID REFERENCES teams(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    role VARCHAR(50),

    max_uses INT DEFAULT 0,
    current_uses INT DEFAULT 0,
    max_budget DECIMAL(12, 4),

    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,

    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    description TEXT,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_invitation_links_team_id ON invitation_links(team_id);
CREATE INDEX IF NOT EXISTS idx_invitation_links_org_id ON invitation_links(organization_id);
CREATE INDEX IF NOT EXISTS idx_invitation_links_created_by ON invitation_links(created_by);
CREATE INDEX IF NOT EXISTS idx_invitation_links_is_active ON invitation_links(is_active);
CREATE INDEX IF NOT EXISTS idx_invitation_links_created_at ON invitation_links(created_at);

