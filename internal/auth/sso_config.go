// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"sync"
	"time"

	"github.com/goccy/go-json"
)

// SSOConfig represents the SSO configuration stored in the database.
// This aligns with LiteLLM's LiteLLM_SSOConfig table.
type SSOConfig struct {
	ID          string      `json:"id"` // Always "sso_config" for singleton
	SSOSettings SSOSettings `json:"sso_settings"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// SSOSettings contains all SSO-related settings.
type SSOSettings struct {
	// Role Mappings
	RoleMappings *RoleMappings `json:"role_mappings,omitempty"`

	// General SSO settings
	GeneralSettings *GeneralSSOSettings `json:"general_settings,omitempty"`

	// Default team/org settings
	DefaultTeamID         string `json:"default_team_id,omitempty"`
	DefaultOrganizationID string `json:"default_organization_id,omitempty"`

	// Raw settings for extensibility
	RawSettings map[string]any `json:"raw_settings,omitempty"`
}

// RoleMappings defines how IdP roles map to internal roles.
// Aligned with LiteLLM's RoleMappings structure.
type RoleMappings struct {
	// Claim to role mappings
	ProxyAdminRoles         []string `json:"proxy_admin_roles,omitempty"`
	ProxyAdminViewerRoles   []string `json:"proxy_admin_viewer_roles,omitempty"`
	OrgAdminRoles           []string `json:"org_admin_roles,omitempty"`
	InternalUserRoles       []string `json:"internal_user_roles,omitempty"`
	InternalUserViewerRoles []string `json:"internal_user_viewer_roles,omitempty"`
	TeamRoles               []string `json:"team_roles,omitempty"`

	// Default role when no mapping matches
	DefaultRole string `json:"default_role,omitempty"`

	// Role claim configuration
	RoleClaim        string `json:"role_claim,omitempty"` // e.g., "groups", "roles"
	UseRoleHierarchy bool   `json:"use_role_hierarchy,omitempty"`
}

// GeneralSSOSettings contains general SSO configuration.
type GeneralSSOSettings struct {
	// Email domain restriction
	UserAllowedEmailDomains []string `json:"user_allowed_email_domains,omitempty"`

	// Auto-provisioning
	UserIDUpsert bool `json:"user_id_upsert,omitempty"`
	TeamIDUpsert bool `json:"team_id_upsert,omitempty"`

	// Sync settings
	SyncUserRoleAndTeams bool `json:"sync_user_role_and_teams,omitempty"`

	// RBAC enforcement
	EnforceRbac bool `json:"enforce_rbac,omitempty"`

	// UserInfo settings
	OIDCUserInfoEnabled bool `json:"oidc_userinfo_enabled,omitempty"`
	UserInfoCacheTTL    int  `json:"userinfo_cache_ttl,omitempty"` // seconds
}

// SSOConfigStore defines the interface for SSO configuration storage.
type SSOConfigStore interface {
	// GetSSOConfig retrieves the SSO configuration.
	GetSSOConfig(ctx context.Context) (*SSOConfig, error)

	// SaveSSOConfig saves the SSO configuration.
	SaveSSOConfig(ctx context.Context, config *SSOConfig) error

	// DeleteSSOConfig deletes the SSO configuration.
	DeleteSSOConfig(ctx context.Context) error
}

// SSOConfigManager manages SSO configuration with caching.
type SSOConfigManager struct {
	store       SSOConfigStore
	cache       *SSOConfig
	cacheTTL    time.Duration
	cacheExpiry time.Time
	mu          sync.RWMutex
}

// NewSSOConfigManager creates a new SSO configuration manager.
func NewSSOConfigManager(store SSOConfigStore, cacheTTL time.Duration) *SSOConfigManager {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &SSOConfigManager{
		store:    store,
		cacheTTL: cacheTTL,
	}
}

// GetConfig retrieves the current SSO configuration.
func (m *SSOConfigManager) GetConfig(ctx context.Context) (*SSOConfig, error) {
	// Check cache first
	m.mu.RLock()
	if m.cache != nil && time.Now().Before(m.cacheExpiry) {
		config := m.cache
		m.mu.RUnlock()
		return config, nil
	}
	m.mu.RUnlock()

	// Fetch from store
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.cache != nil && time.Now().Before(m.cacheExpiry) {
		return m.cache, nil
	}

	config, err := m.store.GetSSOConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	m.cache = config
	m.cacheExpiry = time.Now().Add(m.cacheTTL)

	return config, nil
}

// UpdateConfig updates the SSO configuration.
func (m *SSOConfigManager) UpdateConfig(ctx context.Context, config *SSOConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config.ID = "sso_config"
	config.UpdatedAt = time.Now()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = config.UpdatedAt
	}

	if err := m.store.SaveSSOConfig(ctx, config); err != nil {
		return err
	}

	// Update cache
	m.cache = config
	m.cacheExpiry = time.Now().Add(m.cacheTTL)

	return nil
}

// InvalidateCache invalidates the cached configuration.
func (m *SSOConfigManager) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = nil
	m.cacheExpiry = time.Time{}
}

// GetRoleMappings returns the role mappings from the current configuration.
func (m *SSOConfigManager) GetRoleMappings(ctx context.Context) (*RoleMappings, error) {
	config, err := m.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config == nil || config.SSOSettings.RoleMappings == nil {
		return nil, nil
	}
	return config.SSOSettings.RoleMappings, nil
}

// MemorySSOConfigStore implements SSOConfigStore using in-memory storage.
type MemorySSOConfigStore struct {
	mu     sync.RWMutex
	config *SSOConfig
}

// NewMemorySSOConfigStore creates a new in-memory SSO config store.
func NewMemorySSOConfigStore() *MemorySSOConfigStore {
	return &MemorySSOConfigStore{}
}

// GetSSOConfig retrieves the SSO configuration.
func (s *MemorySSOConfigStore) GetSSOConfig(_ context.Context) (*SSOConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config == nil {
		return nil, nil
	}
	// Return a copy
	configCopy := *s.config
	return &configCopy, nil
}

// SaveSSOConfig saves the SSO configuration.
func (s *MemorySSOConfigStore) SaveSSOConfig(_ context.Context, config *SSOConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	configCopy := *config
	s.config = &configCopy
	return nil
}

// DeleteSSOConfig deletes the SSO configuration.
func (s *MemorySSOConfigStore) DeleteSSOConfig(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = nil
	return nil
}

// MapRoleFromClaims maps IdP roles to internal roles using role mappings.
func (rm *RoleMappings) MapRoleFromClaims(claimRoles []string) UserRole {
	if rm == nil || len(claimRoles) == 0 {
		return UserRole(rm.defaultRoleOrFallback())
	}

	// Check each role mapping in hierarchy order
	roleChecks := []struct {
		idpRoles []string
		role     UserRole
	}{
		{rm.ProxyAdminRoles, UserRoleProxyAdmin},
		{rm.ProxyAdminViewerRoles, UserRoleProxyAdminViewer},
		{rm.OrgAdminRoles, UserRoleOrgAdmin},
		{rm.InternalUserRoles, UserRoleInternalUser},
		{rm.InternalUserViewerRoles, UserRoleInternalUserViewer},
		{rm.TeamRoles, UserRoleTeam},
	}

	// Check each role mapping in hierarchy order (highest priority first)
	// UseRoleHierarchy flag is stored for future use but currently
	// the hierarchy is always applied through the order of roleChecks
	for _, check := range roleChecks {
		if containsAny(claimRoles, check.idpRoles) {
			return check.role
		}
	}

	return UserRole(rm.defaultRoleOrFallback())
}

// defaultRoleOrFallback returns the default role or a fallback.
func (rm *RoleMappings) defaultRoleOrFallback() string {
	if rm != nil && rm.DefaultRole != "" {
		return rm.DefaultRole
	}
	return string(UserRoleInternalUser)
}

// containsAny checks if any element from subset exists in set.
func containsAny(set, subset []string) bool {
	for _, item := range subset {
		for _, s := range set {
			if s == item {
				return true
			}
		}
	}
	return false
}

// ToJSON serializes the SSO settings to JSON.
func (s *SSOSettings) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON deserializes the SSO settings from JSON.
func (s *SSOSettings) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}

// Ensure interfaces are satisfied
var _ SSOConfigStore = (*MemorySSOConfigStore)(nil)
