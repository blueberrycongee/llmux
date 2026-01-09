package auth

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryStore implements Store using in-memory maps.
type MemoryStore struct {
	mu              sync.RWMutex
	apiKeys         map[string]*APIKey
	apiKeysByID     map[string]*APIKey
	budgets         map[string]*Budget
	organizations   map[string]*Organization
	teams           map[string]*Team
	teamMemberships map[string]*TeamMembership
	orgMemberships  map[string]*OrganizationMembership
	users           map[string]*User
	endUsers        map[string]*EndUser
	usageLogs       []*UsageLog
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		apiKeys:         make(map[string]*APIKey),
		apiKeysByID:     make(map[string]*APIKey),
		budgets:         make(map[string]*Budget),
		organizations:   make(map[string]*Organization),
		teams:           make(map[string]*Team),
		teamMemberships: make(map[string]*TeamMembership),
		orgMemberships:  make(map[string]*OrganizationMembership),
		users:           make(map[string]*User),
		endUsers:        make(map[string]*EndUser),
		usageLogs:       make([]*UsageLog, 0),
	}
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.apiKeys[hash]
	if !ok {
		return nil, nil
	}
	// Return a copy to prevent mutation
	keyCopy := *key
	return &keyCopy, nil
}

func (s *MemoryStore) CreateAPIKey(_ context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keyCopy := *key
	s.apiKeys[key.KeyHash] = &keyCopy
	s.apiKeysByID[key.ID] = &keyCopy
	return nil
}

func (s *MemoryStore) GetAPIKeyByID(_ context.Context, keyID string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, ok := s.apiKeysByID[keyID]
	if !ok {
		return nil, nil
	}
	keyCopy := *key
	return &keyCopy, nil
}

func (s *MemoryStore) GetAPIKeyByAlias(_ context.Context, alias string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, key := range s.apiKeys {
		if key.KeyAlias != nil && *key.KeyAlias == alias {
			keyCopy := *key
			return &keyCopy, nil
		}
	}
	return nil, nil
}

func (s *MemoryStore) UpdateAPIKey(_ context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.apiKeysByID[key.ID]; ok {
		keyCopy := *key
		s.apiKeys[existing.KeyHash] = &keyCopy
		s.apiKeysByID[key.ID] = &keyCopy
	}
	return nil
}

func (s *MemoryStore) UpdateAPIKeyLastUsed(_ context.Context, keyID string, lastUsed time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		key.LastUsedAt = &lastUsed
	}
	return nil
}

func (s *MemoryStore) UpdateAPIKeySpent(_ context.Context, keyID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		key.SpentBudget += amount
	}
	return nil
}

func (s *MemoryStore) UpdateAPIKeyModelSpent(_ context.Context, keyID, model string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		if key.ModelSpend == nil {
			key.ModelSpend = make(map[string]float64)
		}
		key.ModelSpend[model] += amount
	}
	return nil
}

func (s *MemoryStore) ResetAPIKeyBudget(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		key.SpentBudget = 0
		key.ModelSpend = make(map[string]float64)
		key.BudgetResetAt = key.BudgetDuration.NextResetTime()
	}
	return nil
}

func (s *MemoryStore) DeleteAPIKey(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		key.IsActive = false
	}
	return nil
}

func (s *MemoryStore) ListAPIKeys(_ context.Context, filter APIKeyFilter) ([]*APIKey, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*APIKey, 0, len(s.apiKeys))
	for _, key := range s.apiKeys {
		// By default, only return active keys (soft delete behavior)
		if filter.IsActive == nil {
			if !key.IsActive {
				continue
			}
		} else if key.IsActive != *filter.IsActive {
			continue
		}
		if filter.Blocked != nil && key.Blocked != *filter.Blocked {
			continue
		}
		if filter.TeamID != nil && (key.TeamID == nil || *key.TeamID != *filter.TeamID) {
			continue
		}
		keyCopy := *key
		result = append(result, &keyCopy)
	}

	total := int64(len(result))
	if filter.Offset >= len(result) {
		return []*APIKey{}, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(result) || filter.Limit == 0 {
		end = len(result)
	}
	return result[filter.Offset:end], total, nil
}

func (s *MemoryStore) BlockAPIKey(_ context.Context, keyID string, blocked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.apiKeysByID[keyID]; ok {
		key.Blocked = blocked
	}
	return nil
}

func (s *MemoryStore) GetTeam(_ context.Context, teamID string) (*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	team, ok := s.teams[teamID]
	if !ok {
		return nil, nil
	}
	teamCopy := *team
	return &teamCopy, nil
}

func (s *MemoryStore) CreateTeam(_ context.Context, team *Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	teamCopy := *team
	s.teams[team.ID] = &teamCopy
	return nil
}

func (s *MemoryStore) UpdateTeam(_ context.Context, team *Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	teamCopy := *team
	s.teams[team.ID] = &teamCopy
	return nil
}

func (s *MemoryStore) UpdateTeamSpent(_ context.Context, teamID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.SpentBudget += amount
	}
	return nil
}

func (s *MemoryStore) UpdateTeamModelSpent(_ context.Context, teamID, model string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		if team.ModelSpend == nil {
			team.ModelSpend = make(map[string]float64)
		}
		team.ModelSpend[model] += amount
	}
	return nil
}

func (s *MemoryStore) ResetTeamBudget(_ context.Context, teamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.SpentBudget = 0
		team.ModelSpend = make(map[string]float64)
		team.BudgetResetAt = team.BudgetDuration.NextResetTime()
	}
	return nil
}

func (s *MemoryStore) DeleteTeam(_ context.Context, teamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.IsActive = false
	}
	return nil
}

func (s *MemoryStore) ListTeams(_ context.Context, filter TeamFilter) ([]*Team, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Team, 0, len(s.teams))
	for _, team := range s.teams {
		// By default, only return active teams (soft delete behavior)
		if filter.IsActive == nil {
			if !team.IsActive {
				continue
			}
		} else if team.IsActive != *filter.IsActive {
			continue
		}
		if filter.Blocked != nil && team.Blocked != *filter.Blocked {
			continue
		}
		teamCopy := *team
		result = append(result, &teamCopy)
	}

	total := int64(len(result))
	if filter.Offset >= len(result) {
		return []*Team{}, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(result) || filter.Limit == 0 {
		end = len(result)
	}
	return result[filter.Offset:end], total, nil
}

func (s *MemoryStore) BlockTeam(_ context.Context, teamID string, blocked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.Blocked = blocked
	}
	return nil
}

func (s *MemoryStore) GetUser(_ context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return nil, nil
	}
	userCopy := *user
	return &userCopy, nil
}

func (s *MemoryStore) GetUserByEmail(_ context.Context, email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, user := range s.users {
		if user.Email != nil && *user.Email == email && user.IsActive {
			userCopy := *user
			return &userCopy, nil
		}
	}
	return nil, nil
}

func (s *MemoryStore) GetUserBySSOID(_ context.Context, _ string) (*User, error) {
	return nil, nil
}

func (s *MemoryStore) CreateUser(_ context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userCopy := *user
	s.users[user.ID] = &userCopy
	return nil
}

func (s *MemoryStore) UpdateUser(_ context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userCopy := *user
	s.users[user.ID] = &userCopy
	return nil
}

func (s *MemoryStore) UpdateUserSpent(_ context.Context, userID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.users[userID]; ok {
		user.Spend += amount
	}
	return nil
}

func (s *MemoryStore) ResetUserBudget(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.users[userID]; ok {
		user.Spend = 0
		user.ModelSpend = make(map[string]float64)
		user.BudgetResetAt = user.BudgetDuration.NextResetTime()
	}
	return nil
}

func (s *MemoryStore) DeleteUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.users[userID]; ok {
		user.IsActive = false
	}
	return nil
}

func (s *MemoryStore) ListUsers(_ context.Context, filter UserFilter) ([]*User, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		if filter.IsActive != nil && user.IsActive != *filter.IsActive {
			continue
		}
		if filter.TeamID != nil && (user.TeamID == nil || *user.TeamID != *filter.TeamID) {
			continue
		}
		if filter.OrganizationID != nil && (user.OrganizationID == nil || *user.OrganizationID != *filter.OrganizationID) {
			continue
		}
		if filter.Role != nil && user.Role != string(*filter.Role) {
			continue
		}
		// Search filter: match ID, alias, or email (case-insensitive)
		if filter.Search != nil && *filter.Search != "" {
			searchLower := strings.ToLower(*filter.Search)
			idMatch := strings.Contains(strings.ToLower(user.ID), searchLower)
			aliasMatch := user.Alias != nil && strings.Contains(strings.ToLower(*user.Alias), searchLower)
			emailMatch := user.Email != nil && strings.Contains(strings.ToLower(*user.Email), searchLower)
			if !idMatch && !aliasMatch && !emailMatch {
				continue
			}
		}
		userCopy := *user
		result = append(result, &userCopy)
	}

	total := int64(len(result))
	if filter.Offset >= len(result) {
		return []*User{}, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(result) || filter.Limit == 0 {
		end = len(result)
	}
	return result[filter.Offset:end], total, nil
}

func (s *MemoryStore) LogUsage(_ context.Context, log *UsageLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logCopy := *log
	logCopy.ID = int64(len(s.usageLogs) + 1)
	s.usageLogs = append(s.usageLogs, &logCopy)
	return nil
}

func (s *MemoryStore) GetUsageStats(_ context.Context, filter UsageFilter) (*UsageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &UsageStats{}
	models := make(map[string]bool)
	providers := make(map[string]bool)
	var successCount int64

	for _, log := range s.usageLogs {
		if log.StartTime.Before(filter.StartTime) || log.StartTime.After(filter.EndTime) {
			continue
		}
		if filter.APIKeyID != nil && log.APIKeyID != *filter.APIKeyID {
			continue
		}
		if filter.TeamID != nil && (log.TeamID == nil || *log.TeamID != *filter.TeamID) {
			continue
		}
		if filter.Model != nil && log.Model != *filter.Model {
			continue
		}
		if filter.Provider != nil && log.Provider != *filter.Provider {
			continue
		}

		stats.TotalRequests++
		stats.TotalTokens += int64(log.TotalTokens)
		stats.InputTokens += int64(log.InputTokens)
		stats.OutputTokens += int64(log.OutputTokens)
		stats.TotalCost += log.Cost
		stats.AvgLatencyMs += float64(log.LatencyMs)
		models[log.Model] = true
		providers[log.Provider] = true
		if log.StatusCode == nil || *log.StatusCode < 400 {
			successCount++
		}
	}

	if stats.TotalRequests > 0 {
		stats.AvgLatencyMs /= float64(stats.TotalRequests)
		stats.SuccessRate = float64(successCount) / float64(stats.TotalRequests)
	}
	stats.UniqueModels = len(models)
	stats.UniqueProviders = len(providers)

	return stats, nil
}

func (s *MemoryStore) GetDailyUsage(_ context.Context, _ DailyUsageFilter) ([]*DailyUsage, error) {
	return []*DailyUsage{}, nil
}

// Budget operations

func (s *MemoryStore) GetBudget(_ context.Context, budgetID string) (*Budget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.budgets[budgetID]
	if !ok {
		return nil, nil
	}
	bCopy := *b
	return &bCopy, nil
}

func (s *MemoryStore) CreateBudget(_ context.Context, budget *Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bCopy := *budget
	s.budgets[budget.ID] = &bCopy
	return nil
}

func (s *MemoryStore) UpdateBudget(_ context.Context, budget *Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bCopy := *budget
	s.budgets[budget.ID] = &bCopy
	return nil
}

func (s *MemoryStore) DeleteBudget(_ context.Context, budgetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.budgets, budgetID)
	return nil
}

// Organization operations

func (s *MemoryStore) GetOrganization(_ context.Context, orgID string) (*Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	org, ok := s.organizations[orgID]
	if !ok {
		return nil, nil
	}
	orgCopy := *org
	return &orgCopy, nil
}

func (s *MemoryStore) CreateOrganization(_ context.Context, org *Organization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	orgCopy := *org
	s.organizations[org.ID] = &orgCopy
	return nil
}

func (s *MemoryStore) UpdateOrganization(_ context.Context, org *Organization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	orgCopy := *org
	s.organizations[org.ID] = &orgCopy
	return nil
}

func (s *MemoryStore) UpdateOrganizationSpent(_ context.Context, orgID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if org, ok := s.organizations[orgID]; ok {
		org.Spend += amount
	}
	return nil
}

func (s *MemoryStore) DeleteOrganization(_ context.Context, orgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.organizations, orgID)
	return nil
}

func (s *MemoryStore) ListOrganizations(_ context.Context, limit, offset int) ([]*Organization, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Organization, 0, len(s.organizations))
	for _, org := range s.organizations {
		orgCopy := *org
		result = append(result, &orgCopy)
	}

	total := int64(len(result))
	if offset >= len(result) {
		return []*Organization{}, total, nil
	}
	end := offset + limit
	if end > len(result) || limit == 0 {
		end = len(result)
	}
	return result[offset:end], total, nil
}

// Team membership operations

func membershipKey(userID, teamID string) string {
	return userID + ":" + teamID
}

func (s *MemoryStore) GetTeamMembership(_ context.Context, userID, teamID string) (*TeamMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.teamMemberships[membershipKey(userID, teamID)]
	if !ok {
		return nil, nil
	}
	mCopy := *m
	return &mCopy, nil
}

func (s *MemoryStore) CreateTeamMembership(_ context.Context, membership *TeamMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mCopy := *membership
	s.teamMemberships[membershipKey(membership.UserID, membership.TeamID)] = &mCopy
	return nil
}

func (s *MemoryStore) UpdateTeamMembershipSpent(_ context.Context, userID, teamID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.teamMemberships[membershipKey(userID, teamID)]; ok {
		m.Spend += amount
	}
	return nil
}

func (s *MemoryStore) DeleteTeamMembership(_ context.Context, userID, teamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.teamMemberships, membershipKey(userID, teamID))
	return nil
}

func (s *MemoryStore) ListTeamMembers(_ context.Context, teamID string) ([]*TeamMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TeamMembership
	for _, m := range s.teamMemberships {
		if m.TeamID == teamID {
			mCopy := *m
			result = append(result, &mCopy)
		}
	}
	return result, nil
}

func (s *MemoryStore) UpdateTeamMembership(_ context.Context, membership *TeamMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mCopy := *membership
	s.teamMemberships[membershipKey(membership.UserID, membership.TeamID)] = &mCopy
	return nil
}

func (s *MemoryStore) ListUserTeamMemberships(_ context.Context, userID string) ([]*TeamMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*TeamMembership
	for _, m := range s.teamMemberships {
		if m.UserID == userID {
			mCopy := *m
			result = append(result, &mCopy)
		}
	}
	return result, nil
}

// Organization membership operations

func orgMembershipKey(userID, orgID string) string {
	return userID + ":" + orgID
}

func (s *MemoryStore) GetOrganizationMembership(_ context.Context, userID, orgID string) (*OrganizationMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.orgMemberships[orgMembershipKey(userID, orgID)]
	if !ok {
		return nil, nil
	}
	mCopy := *m
	return &mCopy, nil
}

func (s *MemoryStore) CreateOrganizationMembership(_ context.Context, membership *OrganizationMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mCopy := *membership
	s.orgMemberships[orgMembershipKey(membership.UserID, membership.OrganizationID)] = &mCopy
	return nil
}

func (s *MemoryStore) UpdateOrganizationMembership(_ context.Context, membership *OrganizationMembership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mCopy := *membership
	s.orgMemberships[orgMembershipKey(membership.UserID, membership.OrganizationID)] = &mCopy
	return nil
}

func (s *MemoryStore) UpdateOrganizationMembershipSpent(_ context.Context, userID, orgID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.orgMemberships[orgMembershipKey(userID, orgID)]; ok {
		m.Spend += amount
	}
	return nil
}

func (s *MemoryStore) DeleteOrganizationMembership(_ context.Context, userID, orgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.orgMemberships, orgMembershipKey(userID, orgID))
	return nil
}

func (s *MemoryStore) ListOrganizationMembers(_ context.Context, orgID string) ([]*OrganizationMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*OrganizationMembership
	for _, m := range s.orgMemberships {
		if m.OrganizationID == orgID {
			mCopy := *m
			result = append(result, &mCopy)
		}
	}
	return result, nil
}

func (s *MemoryStore) ListUserOrganizationMemberships(_ context.Context, userID string) ([]*OrganizationMembership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*OrganizationMembership
	for _, m := range s.orgMemberships {
		if m.UserID == userID {
			mCopy := *m
			result = append(result, &mCopy)
		}
	}
	return result, nil
}

// End user operations

func (s *MemoryStore) GetEndUser(_ context.Context, userID string) (*EndUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	eu, ok := s.endUsers[userID]
	if !ok {
		return nil, nil
	}
	euCopy := *eu
	return &euCopy, nil
}

func (s *MemoryStore) CreateEndUser(_ context.Context, endUser *EndUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	euCopy := *endUser
	s.endUsers[endUser.UserID] = &euCopy
	return nil
}

func (s *MemoryStore) UpdateEndUserSpent(_ context.Context, userID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if eu, ok := s.endUsers[userID]; ok {
		eu.Spend += amount
	}
	return nil
}

func (s *MemoryStore) BlockEndUser(_ context.Context, userID string, blocked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if eu, ok := s.endUsers[userID]; ok {
		eu.Blocked = blocked
	}
	return nil
}

func (s *MemoryStore) DeleteEndUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.endUsers, userID)
	return nil
}

// Budget reset operations

func (s *MemoryStore) GetKeysNeedingBudgetReset(_ context.Context) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*APIKey
	now := time.Now()
	for _, key := range s.apiKeys {
		if key.IsActive && key.BudgetResetAt != nil && now.After(*key.BudgetResetAt) {
			keyCopy := *key
			result = append(result, &keyCopy)
		}
	}
	return result, nil
}

func (s *MemoryStore) GetTeamsNeedingBudgetReset(_ context.Context) ([]*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Team
	now := time.Now()
	for _, team := range s.teams {
		if team.IsActive && team.BudgetResetAt != nil && now.After(*team.BudgetResetAt) {
			teamCopy := *team
			result = append(result, &teamCopy)
		}
	}
	return result, nil
}

func (s *MemoryStore) GetUsersNeedingBudgetReset(_ context.Context) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*User
	now := time.Now()
	for _, user := range s.users {
		if user.IsActive && user.BudgetResetAt != nil && now.After(*user.BudgetResetAt) {
			userCopy := *user
			result = append(result, &userCopy)
		}
	}
	return result, nil
}

var _ Store = (*MemoryStore)(nil)
