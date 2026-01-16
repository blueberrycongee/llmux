package auth

import (
	"context"
	"testing"
)

func TestMemoryStore_APIKey_DeepCopy(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	key := &APIKey{
		ID:       "key-1",
		KeyHash:  "hash-1",
		IsActive: true,
		ModelSpend: map[string]float64{
			"gpt-4": 10.0,
		},
		AllowedModels: []string{"gpt-4", "gpt-3.5-turbo"},
		Metadata: Metadata{
			"foo": "bar",
		},
	}

	err := store.CreateAPIKey(ctx, key)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	// Get the key
	retrieved, err := store.GetAPIKeyByID(ctx, key.ID)
	if err != nil {
		t.Fatalf("GetAPIKeyByID() error = %v", err)
	}

	// Modify the retrieved key's maps/slices
	retrieved.ModelSpend["gpt-4"] = 20.0
	retrieved.AllowedModels[0] = "claude-3"
	retrieved.Metadata["foo"] = "baz"

	// Get the key again from store
	again, _ := store.GetAPIKeyByID(ctx, key.ID)

	// Verify original values in store are unchanged
	if again.ModelSpend["gpt-4"] != 10.0 {
		t.Errorf("ModelSpend was mutated: got %v, want 10.0", again.ModelSpend["gpt-4"])
	}
	if again.AllowedModels[0] != "gpt-4" {
		t.Errorf("AllowedModels was mutated: got %v, want gpt-4", again.AllowedModels[0])
	}
	if again.Metadata["foo"] != "bar" {
		t.Errorf("Metadata was mutated: got %v, want bar", again.Metadata["foo"])
	}
}

func TestMemoryStore_Team_DeepCopy(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	team := &Team{
		ID:       "team-1",
		IsActive: true,
		ModelSpend: map[string]float64{
			"gpt-4": 10.0,
		},
		Members: []string{"user-1"},
	}

	err := store.CreateTeam(ctx, team)
	if err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}

	retrieved, _ := store.GetTeam(ctx, team.ID)
	retrieved.ModelSpend["gpt-4"] = 20.0
	retrieved.Members[0] = "user-2"

	again, _ := store.GetTeam(ctx, team.ID)
	if again.ModelSpend["gpt-4"] != 10.0 {
		t.Errorf("Team ModelSpend was mutated")
	}
	if again.Members[0] != "user-1" {
		t.Errorf("Team Members was mutated")
	}
}
