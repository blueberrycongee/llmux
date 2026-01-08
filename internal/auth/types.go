// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"time"
)

// KeyType defines the type of API key and its default permissions.
type KeyType string

const (
	KeyTypeLLMAPI     KeyType = "llm_api"    // Can only call LLM API routes
	KeyTypeManagement KeyType = "management" // Can call management routes
	KeyTypeReadOnly   KeyType = "read_only"  // Can only call info/read routes
	KeyTypeDefault    KeyType = "default"    // Uses default allowed routes
)

// APIKey represents an API key with its associated permissions and limits.
type APIKey struct {
	ID        string  `json:"id"`
	KeyHash   string  `json:"-"`          // Never expose hash
	KeyPrefix string  `json:"key_prefix"` // First 8 chars for identification
	Name      string  `json:"name"`
	KeyAlias  *string `json:"key_alias,omitempty"` // Human-readable alias

	// Ownership
	TeamID         *string `json:"team_id,omitempty"`
	UserID         *string `json:"user_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`

	// Access control
	AllowedModels []string `json:"allowed_models,omitempty"` // Empty = all models
	KeyType       KeyType  `json:"key_type,omitempty"`

	// Rate limiting (LiteLLM compatible)
	TPMLimit            *int64           `json:"tpm_limit,omitempty"`             // Tokens per minute
	RPMLimit            *int64           `json:"rpm_limit,omitempty"`             // Requests per minute
	MaxParallelRequests *int             `json:"max_parallel_requests,omitempty"` // Concurrent requests
	ModelTPMLimit       map[string]int64 `json:"model_tpm_limit,omitempty"`       // Per-model TPM
	ModelRPMLimit       map[string]int64 `json:"model_rpm_limit,omitempty"`       // Per-model RPM

	// Budget management (LiteLLM compatible)
	MaxBudget      float64            `json:"max_budget,omitempty"`       // Hard budget limit
	SoftBudget     *float64           `json:"soft_budget,omitempty"`      // Alert threshold
	SpentBudget    float64            `json:"spent_budget"`               // Current spend
	ModelMaxBudget map[string]float64 `json:"model_max_budget,omitempty"` // Per-model budget
	ModelSpend     map[string]float64 `json:"model_spend,omitempty"`      // Per-model spend
	BudgetDuration BudgetDuration     `json:"budget_duration,omitempty"`  // Reset period
	BudgetResetAt  *time.Time         `json:"budget_reset_at,omitempty"`  // Next reset time

	// Metadata
	Metadata Metadata `json:"metadata,omitempty"`

	// Status
	IsActive bool `json:"is_active"`
	Blocked  bool `json:"blocked"` // Explicitly blocked

	// Timestamps
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// Team represents a team within an organization for multi-tenant isolation.
type Team struct {
	ID             string   `json:"team_id"`
	Alias          *string  `json:"team_alias,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	Members        []string `json:"members,omitempty"`

	// Budget management
	MaxBudget      float64            `json:"max_budget,omitempty"`
	SpentBudget    float64            `json:"spend"`
	ModelMaxBudget map[string]float64 `json:"model_max_budget,omitempty"`
	ModelSpend     map[string]float64 `json:"model_spend,omitempty"`
	BudgetDuration BudgetDuration     `json:"budget_duration,omitempty"`
	BudgetResetAt  *time.Time         `json:"budget_reset_at,omitempty"`

	// Rate limiting
	TPMLimit            *int64           `json:"tpm_limit,omitempty"`
	RPMLimit            *int64           `json:"rpm_limit,omitempty"`
	MaxParallelRequests *int             `json:"max_parallel_requests,omitempty"`
	ModelTPMLimit       map[string]int64 `json:"model_tpm_limit,omitempty"`
	ModelRPMLimit       map[string]int64 `json:"model_rpm_limit,omitempty"`

	// Access control
	Models []string `json:"models,omitempty"`

	// Status
	IsActive bool `json:"is_active"`
	Blocked  bool `json:"blocked"`

	// Metadata
	Metadata  Metadata  `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a user within a team.
type User struct {
	ID             string   `json:"user_id"`
	Alias          *string  `json:"user_alias,omitempty"`
	Email          *string  `json:"user_email,omitempty"`
	TeamID         *string  `json:"team_id,omitempty"`
	Teams          []string `json:"teams,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	Role           string   `json:"user_role"` // proxy_admin, internal_user, etc.

	// Budget management
	MaxBudget      float64            `json:"max_budget,omitempty"`
	Spend          float64            `json:"spend"`
	ModelMaxBudget map[string]float64 `json:"model_max_budget,omitempty"`
	ModelSpend     map[string]float64 `json:"model_spend,omitempty"`
	BudgetDuration BudgetDuration     `json:"budget_duration,omitempty"`
	BudgetResetAt  *time.Time         `json:"budget_reset_at,omitempty"`

	// Rate limiting
	TPMLimit            *int64 `json:"tpm_limit,omitempty"`
	RPMLimit            *int64 `json:"rpm_limit,omitempty"`
	MaxParallelRequests *int   `json:"max_parallel_requests,omitempty"`

	// Access control
	Models []string `json:"models,omitempty"`

	// Status
	IsActive bool `json:"is_active"`

	// Metadata
	Metadata  Metadata   `json:"metadata,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// UserRole defines the role of a user in the system.
// Aligned with LiteLLM's role hierarchy.
type UserRole string

const (
	// Admin Roles
	UserRoleProxyAdmin       UserRole = "proxy_admin"        // Admin over the platform
	UserRoleProxyAdminViewer UserRole = "proxy_admin_viewer" // View-only admin access

	// Organization Roles
	UserRoleOrgAdmin UserRole = "org_admin" // Admin over a specific organization

	// Internal User Roles
	UserRoleInternalUser       UserRole = "internal_user"        // Can login, view/create/delete their own keys
	UserRoleInternalUserViewer UserRole = "internal_user_viewer" // Can login, view their own keys only

	// Team and Customer Roles
	UserRoleTeam     UserRole = "team"     // Used for JWT auth
	UserRoleCustomer UserRole = "customer" // External users - customers
)

// UsageLog records API usage for billing and analytics.
type UsageLog struct {
	ID             int64     `json:"id"`
	RequestID      string    `json:"request_id"`
	APIKeyID       string    `json:"api_key"`
	TeamID         *string   `json:"team_id,omitempty"`
	OrganizationID *string   `json:"organization_id,omitempty"`
	UserID         *string   `json:"user,omitempty"`
	EndUserID      *string   `json:"end_user,omitempty"`
	Model          string    `json:"model"`
	ModelGroup     *string   `json:"model_group,omitempty"`
	Provider       string    `json:"custom_llm_provider"`
	CallType       string    `json:"call_type"` // completion, embedding, etc.
	InputTokens    int       `json:"prompt_tokens"`
	OutputTokens   int       `json:"completion_tokens"`
	TotalTokens    int       `json:"total_tokens"`
	Cost           float64   `json:"spend"`
	LatencyMs      int       `json:"latency_ms,omitempty"`
	StatusCode     *int      `json:"status_code,omitempty"`
	Status         *string   `json:"status,omitempty"`
	CacheHit       *string   `json:"cache_hit,omitempty"`
	RequestTags    []string  `json:"request_tags,omitempty"`
	Metadata       Metadata  `json:"metadata,omitempty"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
}

// Metadata is a flexible key-value store for custom attributes.
type Metadata map[string]any

// AuthContext holds authentication information for a request.
type AuthContext struct {
	APIKey     *APIKey
	Team       *Team
	User       *User
	RequestID  string
	UserRole   UserRole
	EndUserID  string   // End user ID for downstream customer tracking
	SSOUserID  string   // SSO provider user ID for identity linking
	JWTTeamIDs []string // Team IDs extracted from JWT claims
	JWTOrgID   string   // Organization ID extracted from JWT claims
}

// CanAccessModel checks if the API key is allowed to use the specified model.
func (k *APIKey) CanAccessModel(model string) bool {
	if len(k.AllowedModels) == 0 {
		return true // No restrictions
	}
	for _, m := range k.AllowedModels {
		if m == model || m == "*" {
			return true
		}
	}
	return false
}

// IsExpired checks if the API key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsOverBudget checks if the API key has exceeded its hard budget.
func (k *APIKey) IsOverBudget() bool {
	if k.MaxBudget <= 0 {
		return false // No budget limit
	}
	return k.SpentBudget >= k.MaxBudget
}

// IsOverSoftBudget checks if the API key has exceeded its soft budget.
func (k *APIKey) IsOverSoftBudget() bool {
	if k.SoftBudget == nil || *k.SoftBudget <= 0 {
		return false
	}
	return k.SpentBudget >= *k.SoftBudget
}

// IsBlocked checks if the API key is blocked or inactive.
func (k *APIKey) IsBlocked() bool {
	return k.Blocked || !k.IsActive
}

// NeedsBudgetReset checks if the API key budget needs to be reset.
func (k *APIKey) NeedsBudgetReset() bool {
	if k.BudgetResetAt == nil {
		return false
	}
	return time.Now().After(*k.BudgetResetAt)
}

// GetModelTPMLimit returns the TPM limit for a specific model.
func (k *APIKey) GetModelTPMLimit(model string) *int64 {
	if k.ModelTPMLimit == nil {
		return k.TPMLimit
	}
	if limit, ok := k.ModelTPMLimit[model]; ok {
		return &limit
	}
	return k.TPMLimit
}

// GetModelRPMLimit returns the RPM limit for a specific model.
func (k *APIKey) GetModelRPMLimit(model string) *int64 {
	if k.ModelRPMLimit == nil {
		return k.RPMLimit
	}
	if limit, ok := k.ModelRPMLimit[model]; ok {
		return &limit
	}
	return k.RPMLimit
}

// IsOverBudget checks if the team has exceeded its budget.
func (t *Team) IsOverBudget() bool {
	if t.MaxBudget <= 0 {
		return false
	}
	return t.SpentBudget >= t.MaxBudget
}

// IsBlocked checks if the team is blocked or inactive.
func (t *Team) IsBlocked() bool {
	return t.Blocked || !t.IsActive
}

// CanAccessModel checks if the team is allowed to use the specified model.
func (t *Team) CanAccessModel(model string) bool {
	if len(t.Models) == 0 {
		return true // No restrictions
	}
	for _, m := range t.Models {
		if m == model || m == "*" {
			return true
		}
	}
	return false
}

// IsProxyAdmin checks if the user has proxy admin role.
func (u *User) IsProxyAdmin() bool {
	return u.Role == string(UserRoleProxyAdmin)
}

// IsOrgAdmin checks if the user has organization admin role.
func (u *User) IsOrgAdmin() bool {
	return u.Role == string(UserRoleOrgAdmin)
}

// IsOverBudget checks if the user has exceeded their budget.
func (u *User) IsOverBudget() bool {
	if u.MaxBudget <= 0 {
		return false
	}
	return u.Spend >= u.MaxBudget
}

// Organization represents a top-level organization.
type Organization struct {
	ID         string             `json:"organization_id"`
	Alias      string             `json:"organization_alias"`
	BudgetID   *string            `json:"budget_id,omitempty"`
	Models     []string           `json:"models,omitempty"`
	MaxBudget  float64            `json:"max_budget,omitempty"`
	Spend      float64            `json:"spend"`
	ModelSpend map[string]float64 `json:"model_spend,omitempty"`
	Metadata   Metadata           `json:"metadata,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

// IsOverBudget checks if the organization has exceeded its budget.
func (o *Organization) IsOverBudget() bool {
	if o.MaxBudget <= 0 {
		return false
	}
	return o.Spend >= o.MaxBudget
}

// TeamMembership tracks a user's membership and spend within a team.
// Aligned with LiteLLM's LiteLLM_TeamMembership model.
type TeamMembership struct {
	UserID   string     `json:"user_id"`
	TeamID   string     `json:"team_id"`
	Role     string     `json:"role,omitempty"` // Role within the team (admin, member, viewer)
	Spend    float64    `json:"spend"`
	BudgetID *string    `json:"budget_id,omitempty"`
	Budget   *Budget    `json:"budget,omitempty"`
	JoinedAt *time.Time `json:"joined_at,omitempty"`
}

// IsOverBudget checks if the team member has exceeded their budget.
func (tm *TeamMembership) IsOverBudget() bool {
	if tm.Budget == nil || tm.Budget.MaxBudget == nil || *tm.Budget.MaxBudget <= 0 {
		return false
	}
	return tm.Spend >= *tm.Budget.MaxBudget
}

// OrganizationMembership tracks a user's membership within an organization.
// Aligned with LiteLLM's LiteLLM_OrganizationMembership model.
type OrganizationMembership struct {
	UserID         string     `json:"user_id"`
	OrganizationID string     `json:"organization_id"`
	UserRole       string     `json:"user_role,omitempty"` // Role within the organization (org_admin, member)
	Spend          float64    `json:"spend"`
	BudgetID       *string    `json:"budget_id,omitempty"`
	Budget         *Budget    `json:"budget,omitempty"`
	JoinedAt       *time.Time `json:"joined_at,omitempty"`
}

// IsOverBudget checks if the organization member has exceeded their budget.
func (om *OrganizationMembership) IsOverBudget() bool {
	if om.Budget == nil || om.Budget.MaxBudget == nil || *om.Budget.MaxBudget <= 0 {
		return false
	}
	return om.Spend >= *om.Budget.MaxBudget
}

// IsOrgAdmin checks if the member has organization admin privileges.
func (om *OrganizationMembership) IsOrgAdmin() bool {
	return om.UserRole == string(UserRoleOrgAdmin)
}

// EndUser represents an end-user passed via the 'user' parameter.
type EndUser struct {
	UserID   string  `json:"user_id"`
	Alias    *string `json:"alias,omitempty"`
	Spend    float64 `json:"spend"`
	BudgetID *string `json:"budget_id,omitempty"`
	Budget   *Budget `json:"budget,omitempty"`
	Blocked  bool    `json:"blocked"`
}

// IsOverBudget checks if the end user has exceeded their budget.
func (e *EndUser) IsOverBudget() bool {
	if e.Budget == nil || e.Budget.MaxBudget == nil || *e.Budget.MaxBudget <= 0 {
		return false
	}
	return e.Spend >= *e.Budget.MaxBudget
}

// IsBlocked checks if the end user is blocked.
func (e *EndUser) IsBlocked() bool {
	return e.Blocked
}
