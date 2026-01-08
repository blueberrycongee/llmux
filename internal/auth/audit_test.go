package auth

import (
	"testing"
	"time"
)

func TestMemoryAuditLogStore_CreateAndGet(t *testing.T) {
	store := NewMemoryAuditLogStore()

	log := &AuditLog{
		ID:         "test-001",
		Timestamp:  time.Now().UTC(),
		ActorID:    "user-123",
		ActorType:  "user",
		Action:     AuditActionAPIKeyCreate,
		ObjectType: AuditObjectAPIKey,
		ObjectID:   "key-456",
		Success:    true,
	}

	// Create
	err := store.CreateAuditLog(log)
	if err != nil {
		t.Fatalf("CreateAuditLog failed: %v", err)
	}

	// Get
	retrieved, err := store.GetAuditLog("test-001")
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetAuditLog returned nil")
	}

	if retrieved.ActorID != log.ActorID {
		t.Errorf("ActorID mismatch: got %s, want %s", retrieved.ActorID, log.ActorID)
	}

	if retrieved.Action != log.Action {
		t.Errorf("Action mismatch: got %s, want %s", retrieved.Action, log.Action)
	}
}

func TestMemoryAuditLogStore_ListWithFilters(t *testing.T) {
	store := NewMemoryAuditLogStore()

	// Create test logs
	logs := []*AuditLog{
		{
			ID:         "log-1",
			Timestamp:  time.Now().Add(-2 * time.Hour),
			ActorID:    "user-1",
			ActorType:  "user",
			Action:     AuditActionAPIKeyCreate,
			ObjectType: AuditObjectAPIKey,
			ObjectID:   "key-1",
			Success:    true,
		},
		{
			ID:         "log-2",
			Timestamp:  time.Now().Add(-1 * time.Hour),
			ActorID:    "user-1",
			ActorType:  "user",
			Action:     AuditActionTeamCreate,
			ObjectType: AuditObjectTeam,
			ObjectID:   "team-1",
			Success:    true,
		},
		{
			ID:         "log-3",
			Timestamp:  time.Now(),
			ActorID:    "user-2",
			ActorType:  "user",
			Action:     AuditActionLoginFailed,
			ObjectType: AuditObjectUser,
			ObjectID:   "user-2",
			Success:    false,
		},
	}

	for _, log := range logs {
		if err := store.CreateAuditLog(log); err != nil {
			t.Fatalf("CreateAuditLog failed: %v", err)
		}
	}

	// Test: List all
	_, total, err := store.ListAuditLogs(AuditLogFilter{})
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 logs, got %d", total)
	}

	// Test: Filter by actor
	actorID := "user-1"
	_, total, err = store.ListAuditLogs(AuditLogFilter{ActorID: &actorID})
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 logs for user-1, got %d", total)
	}

	// Test: Filter by action
	action := AuditActionLoginFailed
	_, total, err = store.ListAuditLogs(AuditLogFilter{Action: &action})
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 failed login log, got %d", total)
	}

	// Test: Filter by success
	success := false
	results, total, err := store.ListAuditLogs(AuditLogFilter{Success: &success})
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 failed log, got %d", total)
	}

	// Verify ordering (most recent first)
	if len(results) > 0 && results[0].ID != "log-3" {
		t.Errorf("Expected most recent log first, got %s", results[0].ID)
	}
}

func TestMemoryAuditLogStore_Stats(t *testing.T) {
	store := NewMemoryAuditLogStore()

	// Create test logs
	logs := []*AuditLog{
		{ID: "1", ActorID: "user-1", Action: AuditActionAPIKeyCreate, ObjectType: AuditObjectAPIKey, Success: true},
		{ID: "2", ActorID: "user-1", Action: AuditActionAPIKeyCreate, ObjectType: AuditObjectAPIKey, Success: true},
		{ID: "3", ActorID: "user-2", Action: AuditActionTeamCreate, ObjectType: AuditObjectTeam, Success: true},
		{ID: "4", ActorID: "user-3", Action: AuditActionLoginFailed, ObjectType: AuditObjectUser, Success: false},
	}

	for _, log := range logs {
		log.Timestamp = time.Now()
		if err := store.CreateAuditLog(log); err != nil {
			t.Fatalf("CreateAuditLog failed: %v", err)
		}
	}

	stats, err := store.GetAuditLogStats(AuditLogFilter{})
	if err != nil {
		t.Fatalf("GetAuditLogStats failed: %v", err)
	}

	if stats.TotalEvents != 4 {
		t.Errorf("Expected 4 total events, got %d", stats.TotalEvents)
	}

	if stats.SuccessCount != 3 {
		t.Errorf("Expected 3 success events, got %d", stats.SuccessCount)
	}

	if stats.FailureCount != 1 {
		t.Errorf("Expected 1 failure event, got %d", stats.FailureCount)
	}

	if stats.UniqueActors != 3 {
		t.Errorf("Expected 3 unique actors, got %d", stats.UniqueActors)
	}

	if stats.ActionCounts[string(AuditActionAPIKeyCreate)] != 2 {
		t.Errorf("Expected 2 api_key_create actions, got %d", stats.ActionCounts[string(AuditActionAPIKeyCreate)])
	}
}

func TestMemoryAuditLogStore_DeleteOldLogs(t *testing.T) {
	store := NewMemoryAuditLogStore()

	// Create logs with different timestamps
	oldLog := &AuditLog{
		ID:        "old",
		Timestamp: time.Now().Add(-48 * time.Hour),
		ActorID:   "user-1",
		Action:    AuditActionLogin,
	}

	newLog := &AuditLog{
		ID:        "new",
		Timestamp: time.Now(),
		ActorID:   "user-1",
		Action:    AuditActionLogin,
	}

	_ = store.CreateAuditLog(oldLog)
	_ = store.CreateAuditLog(newLog)

	// Delete logs older than 24 hours
	deleted, err := store.DeleteAuditLogs(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("DeleteAuditLogs failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted log, got %d", deleted)
	}

	// Verify only new log remains
	remaining, total, _ := store.ListAuditLogs(AuditLogFilter{})
	if total != 1 {
		t.Errorf("Expected 1 remaining log, got %d", total)
	}

	if len(remaining) > 0 && remaining[0].ID != "new" {
		t.Errorf("Expected 'new' log to remain, got %s", remaining[0].ID)
	}
}

func TestAuditLogger_LogAction(t *testing.T) {
	store := NewMemoryAuditLogStore()
	logger := NewAuditLogger(store, true)

	err := logger.LogAction(
		"user-123",
		"user",
		AuditActionTeamCreate,
		AuditObjectTeam,
		"team-456",
		true,
		nil,
		map[string]any{"name": "Test Team"},
	)

	if err != nil {
		t.Fatalf("LogAction failed: %v", err)
	}

	// Verify log was created
	logs, total, _ := store.ListAuditLogs(AuditLogFilter{})
	if total != 1 {
		t.Errorf("Expected 1 log, got %d", total)
	}

	if len(logs) > 0 {
		if logs[0].Action != AuditActionTeamCreate {
			t.Errorf("Expected team_create action, got %s", logs[0].Action)
		}
	}
}

func TestAuditLogger_Disabled(t *testing.T) {
	store := NewMemoryAuditLogStore()
	logger := NewAuditLogger(store, false) // Disabled

	err := logger.LogAction(
		"user-123",
		"user",
		AuditActionTeamCreate,
		AuditObjectTeam,
		"team-456",
		true,
		nil,
		nil,
	)

	if err != nil {
		t.Fatalf("LogAction failed: %v", err)
	}

	// Verify no log was created (logger is disabled)
	_, total, _ := store.ListAuditLogs(AuditLogFilter{})
	if total != 0 {
		t.Errorf("Expected 0 logs (logger disabled), got %d", total)
	}
}

func TestCalculateDiff(t *testing.T) {
	before := map[string]any{
		"name":   "Old Name",
		"budget": 100.0,
		"active": true,
	}

	after := map[string]any{
		"name":   "New Name",
		"budget": 200.0,
		"models": []string{"gpt-4"},
	}

	diff := calculateDiff(before, after)

	// Check changed field
	if nameChange, ok := diff["name"].(map[string]any); ok {
		if nameChange["before"] != "Old Name" || nameChange["after"] != "New Name" {
			t.Error("Name change not correctly tracked")
		}
	} else {
		t.Error("Name change not found in diff")
	}

	// Check removed field
	if activeChange, ok := diff["active"].(map[string]any); ok {
		if activeChange["after"] != nil {
			t.Error("Removed field should have nil after value")
		}
	} else {
		t.Error("Removed field not found in diff")
	}

	// Check added field
	if modelsChange, ok := diff["models"].(map[string]any); ok {
		if modelsChange["before"] != nil {
			t.Error("Added field should have nil before value")
		}
	} else {
		t.Error("Added field not found in diff")
	}
}
