package auth

import (
	"context"
	"time"
)

type Store interface {
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	GetAPIKeyByID(ctx context.Context, keyID string) (*APIKey, error)
	GetAPIKeyByAlias(ctx context.Context, alias string) (*APIKey, error)
	CreateAPIKey(ctx context.Context, key *APIKey) error
	UpdateAPIKey(ctx context.Context, key *APIKey) error
	UpdateAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error
	UpdateAPIKeySpent(ctx context.Context, keyID string, amount float64) error
	UpdateAPIKeyModelSpent(ctx context.Context, keyID, model string, amount float64) error
	ResetAPIKeyBudget(ctx context.Context, keyID string) error
	DeleteAPIKey(ctx context.Context, keyID string) error
	ListAPIKeys(ctx context.Context, filter APIKeyFilter) ([]*APIKey, int64, error)
	BlockAPIKey(ctx context.Context, keyID string, blocked bool) error
	GetBudget(ctx context.Context, budgetID string) (*Budget, error)
	CreateBudget(ctx context.Context, budget *Budget) error
	UpdateBudget(ctx context.Context, budget *Budget) error
	DeleteBudget(ctx context.Context, budgetID string) error
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	CreateOrganization(ctx context.Context, org *Organization) error
	UpdateOrganization(ctx context.Context, org *Organization) error
	UpdateOrganizationSpent(ctx context.Context, orgID string, amount float64) error
	DeleteOrganization(ctx context.Context, orgID string) error
	ListOrganizations(ctx context.Context, limit, offset int) ([]*Organization, int64, error)
	GetTeam(ctx context.Context, teamID string) (*Team, error)
	CreateTeam(ctx context.Context, team *Team) error
	UpdateTeam(ctx context.Context, team *Team) error
	UpdateTeamSpent(ctx context.Context, teamID string, amount float64) error
	UpdateTeamModelSpent(ctx context.Context, teamID, model string, amount float64) error
	ResetTeamBudget(ctx context.Context, teamID string) error
	DeleteTeam(ctx context.Context, teamID string) error
	ListTeams(ctx context.Context, filter TeamFilter) ([]*Team, int64, error)
	BlockTeam(ctx context.Context, teamID string, blocked bool) error
	GetTeamMembership(ctx context.Context, userID, teamID string) (*TeamMembership, error)
	CreateTeamMembership(ctx context.Context, membership *TeamMembership) error
	UpdateTeamMembershipSpent(ctx context.Context, userID, teamID string, amount float64) error
	DeleteTeamMembership(ctx context.Context, userID, teamID string) error
	ListTeamMembers(ctx context.Context, teamID string) ([]*TeamMembership, error)
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserBySSOID(ctx context.Context, ssoID string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	UpdateUserSpent(ctx context.Context, userID string, amount float64) error
	ResetUserBudget(ctx context.Context, userID string) error
	DeleteUser(ctx context.Context, userID string) error
	ListUsers(ctx context.Context, filter UserFilter) ([]*User, int64, error)
	GetEndUser(ctx context.Context, userID string) (*EndUser, error)
	CreateEndUser(ctx context.Context, endUser *EndUser) error
	UpdateEndUserSpent(ctx context.Context, userID string, amount float64) error
	BlockEndUser(ctx context.Context, userID string, blocked bool) error
	DeleteEndUser(ctx context.Context, userID string) error
	LogUsage(ctx context.Context, log *UsageLog) error
	GetUsageStats(ctx context.Context, filter UsageFilter) (*UsageStats, error)
	GetDailyUsage(ctx context.Context, filter DailyUsageFilter) ([]*DailyUsage, error)
	GetKeysNeedingBudgetReset(ctx context.Context) ([]*APIKey, error)
	GetTeamsNeedingBudgetReset(ctx context.Context) ([]*Team, error)
	GetUsersNeedingBudgetReset(ctx context.Context) ([]*User, error)
	Ping(ctx context.Context) error
	Close() error
}

type APIKeyFilter struct {
	TeamID         *string
	UserID         *string
	OrganizationID *string
	KeyType        *KeyType
	IsActive       *bool
	Blocked        *bool
	Limit          int
	Offset         int
}

type TeamFilter struct {
	OrganizationID *string
	IsActive       *bool
	Blocked        *bool
	Limit          int
	Offset         int
}

type UserFilter struct {
	TeamID         *string
	OrganizationID *string
	Role           *UserRole
	IsActive       *bool
	Limit          int
	Offset         int
}

type UsageFilter struct {
	APIKeyID  *string
	TeamID    *string
	Model     *string
	Provider  *string
	StartTime time.Time
	EndTime   time.Time
}

type DailyUsageFilter struct {
	APIKeyID  *string
	TeamID    *string
	Model     *string
	Provider  *string
	StartDate string
	EndDate   string
	GroupBy   []string
}

type UsageStats struct {
	TotalRequests   int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	TotalCost       float64
	AvgLatencyMs    float64
	SuccessRate     float64
	UniqueModels    int
	UniqueProviders int
}

type DailyUsage struct {
	ID           string
	Date         string
	APIKeyID     string
	TeamID       *string
	Model        *string
	Provider     *string
	InputTokens  int64
	OutputTokens int64
	Spend        float64
	APIRequests  int64
}
