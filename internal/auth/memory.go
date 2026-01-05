package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryStore implements Store using in-memory maps.
// Useful for testing and single-instance deployments without a database.
type MemoryStore struct {
	mu        sync.RWMutex
	apiKeys   map[string]*APIKey // keyed by hash
	teams     map[string]*Team
	users     map[string]*User
	usageLogs []*UsageLog
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		apiKeys:   make(map[string]*APIKey),
		teams:     make(map[string]*Team),
		users:     make(map[string]*User),
		usageLogs: make([]*UsageLog, 0),
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

func (s *MemoryStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keyCopy := *key
	s.apiKeys[key.KeyHash] = &keyCopy
	return nil
}

func (s *MemoryStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range s.apiKeys {
		if key.ID == keyID {
			key.LastUsedAt = &lastUsed
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) UpdateAPIKeySpent(ctx context.Context, keyID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range s.apiKeys {
		if key.ID == keyID {
			key.SpentBudget += amount
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) DeleteAPIKey(ctx context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, key := range s.apiKeys {
		if key.ID == keyID {
			key.IsActive = false
			s.apiKeys[hash] = key
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) ListAPIKeys(ctx context.Context, teamID *string, limit, offset int) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*APIKey, 0, len(s.apiKeys))
	for _, key := range s.apiKeys {
		if !key.IsActive {
			continue
		}
		if teamID != nil && (key.TeamID == nil || *key.TeamID != *teamID) {
			continue
		}
		keyCopy := *key
		result = append(result, &keyCopy)
	}

	// Apply pagination
	if offset >= len(result) {
		return []*APIKey{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (s *MemoryStore) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	team, ok := s.teams[teamID]
	if !ok {
		return nil, nil
	}
	teamCopy := *team
	return &teamCopy, nil
}

func (s *MemoryStore) CreateTeam(ctx context.Context, team *Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	teamCopy := *team
	s.teams[team.ID] = &teamCopy
	return nil
}

func (s *MemoryStore) UpdateTeamSpent(ctx context.Context, teamID string, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.SpentBudget += amount
	}
	return nil
}

func (s *MemoryStore) DeleteTeam(ctx context.Context, teamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if team, ok := s.teams[teamID]; ok {
		team.IsActive = false
	}
	return nil
}

func (s *MemoryStore) ListTeams(ctx context.Context, limit, offset int) ([]*Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Team
	for _, team := range s.teams {
		if team.IsActive {
			teamCopy := *team
			result = append(result, &teamCopy)
		}
	}

	if offset >= len(result) {
		return []*Team{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (s *MemoryStore) GetUser(ctx context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return nil, nil
	}
	userCopy := *user
	return &userCopy, nil
}

func (s *MemoryStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
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

func (s *MemoryStore) CreateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userCopy := *user
	s.users[user.ID] = &userCopy
	return nil
}

func (s *MemoryStore) DeleteUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user, ok := s.users[userID]; ok {
		user.IsActive = false
	}
	return nil
}

func (s *MemoryStore) LogUsage(ctx context.Context, log *UsageLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logCopy := *log
	logCopy.ID = int64(len(s.usageLogs) + 1)
	s.usageLogs = append(s.usageLogs, &logCopy)
	return nil
}

func (s *MemoryStore) GetUsageStats(ctx context.Context, filter UsageFilter) (*UsageStats, error) {
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
