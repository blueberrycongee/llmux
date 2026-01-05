package auth

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_APIKey(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create API key
	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:        "key-1",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Test Key",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	err := store.CreateAPIKey(ctx, key)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	// Get by hash
	retrieved, err := store.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetAPIKeyByHash() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetAPIKeyByHash() returned nil")
	}
	if retrieved.ID != key.ID {
		t.Errorf("ID = %q, want %q", retrieved.ID, key.ID)
	}

	// Get non-existent
	notFound, err := store.GetAPIKeyByHash(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash() error = %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for non-existent key")
	}

	// Update last used
	now := time.Now()
	err = store.UpdateAPIKeyLastUsed(ctx, key.ID, now)
	if err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed() error = %v", err)
	}

	// Update spent
	err = store.UpdateAPIKeySpent(ctx, key.ID, 10.5)
	if err != nil {
		t.Fatalf("UpdateAPIKeySpent() error = %v", err)
	}

	retrieved, _ = store.GetAPIKeyByHash(ctx, hash)
	if retrieved.SpentBudget != 10.5 {
		t.Errorf("SpentBudget = %f, want 10.5", retrieved.SpentBudget)
	}

	// List keys
	keys, err := store.ListAPIKeys(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("ListAPIKeys() returned %d keys, want 1", len(keys))
	}

	// Delete (soft)
	err = store.DeleteAPIKey(ctx, key.ID)
	if err != nil {
		t.Fatalf("DeleteAPIKey() error = %v", err)
	}

	// Should not appear in list after delete
	keys, _ = store.ListAPIKeys(ctx, nil, 10, 0)
	if len(keys) != 0 {
		t.Errorf("ListAPIKeys() after delete returned %d keys, want 0", len(keys))
	}
}

func TestMemoryStore_Team(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	team := &Team{
		ID:        "team-1",
		Name:      "Test Team",
		MaxBudget: 1000,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	err := store.CreateTeam(ctx, team)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}

	retrieved, err := store.GetTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("GetTeam() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetTeam() returned nil")
	}
	if retrieved.Name != team.Name {
		t.Errorf("Name = %q, want %q", retrieved.Name, team.Name)
	}

	// Update spent
	err = store.UpdateTeamSpent(ctx, team.ID, 100)
	if err != nil {
		t.Fatalf("UpdateTeamSpent() error = %v", err)
	}

	retrieved, _ = store.GetTeam(ctx, team.ID)
	if retrieved.SpentBudget != 100 {
		t.Errorf("SpentBudget = %f, want 100", retrieved.SpentBudget)
	}

	// List teams
	teams, err := store.ListTeams(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Errorf("ListTeams() returned %d teams, want 1", len(teams))
	}

	// Delete
	err = store.DeleteTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}

	teams, _ = store.ListTeams(ctx, 10, 0)
	if len(teams) != 0 {
		t.Errorf("ListTeams() after delete returned %d teams, want 0", len(teams))
	}
}

func TestMemoryStore_User(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	user := &User{
		ID:        "user-1",
		Email:     "test@example.com",
		Name:      "Test User",
		Role:      "member",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	err := store.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Get by ID
	retrieved, err := store.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetUser() returned nil")
	}

	// Get by email
	retrieved, err = store.GetUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetUserByEmail() returned nil")
	}

	// Delete
	err = store.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	// Should not find by email after delete
	retrieved, _ = store.GetUserByEmail(ctx, user.Email)
	if retrieved != nil {
		t.Error("expected nil after delete")
	}
}

func TestMemoryStore_UsageLog(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	log := &UsageLog{
		APIKeyID:     "key-1",
		Model:        "gpt-4",
		Provider:     "openai",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		Cost:         0.01,
		LatencyMs:    200,
		StatusCode:   200,
		CreatedAt:    time.Now(),
	}

	err := store.LogUsage(ctx, log)
	if err != nil {
		t.Fatalf("LogUsage() error = %v", err)
	}

	// Get stats
	filter := UsageFilter{
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
	}
	stats, err := store.GetUsageStats(ctx, filter)
	if err != nil {
		t.Fatalf("GetUsageStats() error = %v", err)
	}

	if stats.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1", stats.TotalRequests)
	}
	if stats.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", stats.TotalTokens)
	}
	if stats.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", stats.SuccessRate)
	}
}

func TestMemoryStore_Pagination(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create 5 keys
	for i := 0; i < 5; i++ {
		_, hash, _ := GenerateAPIKey()
		key := &APIKey{
			ID:        string(rune('a' + i)),
			KeyHash:   hash,
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		store.CreateAPIKey(ctx, key)
	}

	// Test pagination
	keys, _ := store.ListAPIKeys(ctx, nil, 2, 0)
	if len(keys) != 2 {
		t.Errorf("ListAPIKeys(limit=2, offset=0) returned %d, want 2", len(keys))
	}

	keys, _ = store.ListAPIKeys(ctx, nil, 2, 3)
	if len(keys) != 2 {
		t.Errorf("ListAPIKeys(limit=2, offset=3) returned %d, want 2", len(keys))
	}

	keys, _ = store.ListAPIKeys(ctx, nil, 10, 10)
	if len(keys) != 0 {
		t.Errorf("ListAPIKeys(limit=10, offset=10) returned %d, want 0", len(keys))
	}
}

func TestMemoryStore_Ping(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
