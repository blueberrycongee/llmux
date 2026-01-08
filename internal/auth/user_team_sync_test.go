package auth

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestUserTeamSyncer_Disabled(t *testing.T) {
	store := NewMemoryStore()
	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{Enabled: false}, nil)

	result, err := syncer.SyncUserTeams(context.Background(), &SyncRequest{
		UserID:  "user-123",
		TeamIDs: []string{"team-1"},
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	// When disabled, should return empty result
	if result.UserCreated || len(result.TeamsAdded) > 0 {
		t.Error("Expected no changes when syncer is disabled")
	}
}

func TestUserTeamSyncer_CreateUser(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{
		Enabled:         true,
		AutoCreateUsers: true,
		DefaultRole:     "internal_user",
	}, logger)

	email := "test@example.com"
	result, err := syncer.SyncUserTeams(context.Background(), &SyncRequest{
		UserID: "new-user",
		Email:  &email,
		Role:   "internal_user",
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	if !result.UserCreated {
		t.Error("Expected user to be created")
	}

	if result.UserID != "new-user" {
		t.Errorf("Expected user ID 'new-user', got %s", result.UserID)
	}

	// Verify user was created in store
	user, _ := store.GetUser(context.Background(), "new-user")
	if user == nil {
		t.Fatal("User not found in store")
	}
	if user.Role != "internal_user" {
		t.Errorf("Expected role 'internal_user', got %s", user.Role)
	}
}

func TestUserTeamSyncer_UserNotFoundWithoutAutoCreate(t *testing.T) {
	store := NewMemoryStore()
	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{
		Enabled:         true,
		AutoCreateUsers: false,
	}, nil)

	_, err := syncer.SyncUserTeams(context.Background(), &SyncRequest{
		UserID: "nonexistent-user",
	})

	if err == nil {
		t.Error("Expected error when user not found and auto-create disabled")
	}

	syncErr, ok := err.(*SyncError)
	if !ok {
		t.Errorf("Expected SyncError, got %T", err)
	}
	if syncErr.Code != "USER_NOT_FOUND" {
		t.Errorf("Expected USER_NOT_FOUND error, got %s", syncErr.Code)
	}
}

func TestUserTeamSyncer_SyncRole(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create existing user
	user := &User{
		ID:       "user-role-test",
		Role:     "team",
		IsActive: true,
	}
	_ = store.CreateUser(ctx, user)

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{
		Enabled:      true,
		SyncUserRole: true,
	}, nil)

	result, err := syncer.SyncUserTeams(ctx, &SyncRequest{
		UserID: "user-role-test",
		Role:   "org_admin",
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	if !result.RoleUpdated {
		t.Error("Expected role to be updated")
	}
	if result.OldRole != "team" {
		t.Errorf("Expected old role 'team', got %s", result.OldRole)
	}
	if result.NewRole != "org_admin" {
		t.Errorf("Expected new role 'org_admin', got %s", result.NewRole)
	}

	// Verify in store
	updatedUser, _ := store.GetUser(ctx, "user-role-test")
	if updatedUser.Role != "org_admin" {
		t.Errorf("User role not updated in store: %s", updatedUser.Role)
	}
}

func TestUserTeamSyncer_SyncTeams(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create user
	user := &User{ID: "user-team-test", Role: "internal_user", IsActive: true}
	_ = store.CreateUser(ctx, user)

	// Create one team
	team1 := &Team{ID: "team-1", IsActive: true}
	_ = store.CreateTeam(ctx, team1)

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{
		Enabled:         true,
		AutoCreateTeams: true,
	}, nil)

	result, err := syncer.SyncUserTeams(ctx, &SyncRequest{
		UserID:  "user-team-test",
		TeamIDs: []string{"team-1", "team-2"}, // team-2 doesn't exist yet
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	// Should add user to both teams
	if len(result.TeamsAdded) != 2 {
		t.Errorf("Expected 2 teams added, got %d: %v", len(result.TeamsAdded), result.TeamsAdded)
	}

	// team-2 should be created
	if len(result.TeamsCreated) != 1 || result.TeamsCreated[0] != "team-2" {
		t.Errorf("Expected team-2 to be created, got %v", result.TeamsCreated)
	}

	// Verify memberships
	memberships, _ := store.ListUserTeamMemberships(ctx, "user-team-test")
	if len(memberships) != 2 {
		t.Errorf("Expected 2 memberships, got %d", len(memberships))
	}
}

func TestUserTeamSyncer_RemoveFromUnlistedTeams(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create user
	user := &User{ID: "user-remove-test", Role: "internal_user", IsActive: true}
	_ = store.CreateUser(ctx, user)

	// Create teams and memberships
	team1 := &Team{ID: "team-keep", IsActive: true}
	team2 := &Team{ID: "team-remove", IsActive: true}
	_ = store.CreateTeam(ctx, team1)
	_ = store.CreateTeam(ctx, team2)

	_ = store.CreateTeamMembership(ctx, &TeamMembership{UserID: "user-remove-test", TeamID: "team-keep", Role: "member"})
	_ = store.CreateTeamMembership(ctx, &TeamMembership{UserID: "user-remove-test", TeamID: "team-remove", Role: "member"})

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{
		Enabled:                 true,
		RemoveFromUnlistedTeams: true,
	}, nil)

	result, err := syncer.SyncUserTeams(ctx, &SyncRequest{
		UserID:  "user-remove-test",
		TeamIDs: []string{"team-keep"}, // team-remove not in list
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	// Should remove from team-remove
	if len(result.TeamsRemoved) != 1 || result.TeamsRemoved[0] != "team-remove" {
		t.Errorf("Expected team-remove to be removed, got %v", result.TeamsRemoved)
	}

	// Verify only team-keep remains
	memberships, _ := store.ListUserTeamMemberships(ctx, "user-remove-test")
	if len(memberships) != 1 {
		t.Errorf("Expected 1 membership remaining, got %d", len(memberships))
	}
	if len(memberships) > 0 && memberships[0].TeamID != "team-keep" {
		t.Errorf("Expected team-keep to remain, got %s", memberships[0].TeamID)
	}
}

func TestUserTeamSyncer_ExistingMembership(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create user and team
	user := &User{ID: "user-existing", Role: "internal_user", IsActive: true}
	team := &Team{ID: "team-existing", IsActive: true}
	_ = store.CreateUser(ctx, user)
	_ = store.CreateTeam(ctx, team)

	// Create existing membership
	_ = store.CreateTeamMembership(ctx, &TeamMembership{
		UserID: "user-existing",
		TeamID: "team-existing",
		Role:   "member",
	})

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{Enabled: true}, nil)

	result, err := syncer.SyncUserTeams(ctx, &SyncRequest{
		UserID:  "user-existing",
		TeamIDs: []string{"team-existing"},
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	// Should not add anything (already a member)
	if len(result.TeamsAdded) != 0 {
		t.Errorf("Expected no teams added (already member), got %v", result.TeamsAdded)
	}
}

func TestUserTeamSyncer_OrganizationMembership(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Create user and organization
	user := &User{ID: "user-org-test", Role: "internal_user", IsActive: true}
	org := &Organization{ID: "org-1"}
	_ = store.CreateUser(ctx, user)
	_ = store.CreateOrganization(ctx, org)

	syncer := NewUserTeamSyncer(store, UserTeamSyncConfig{Enabled: true}, nil)

	orgID := "org-1"
	_, err := syncer.SyncUserTeams(ctx, &SyncRequest{
		UserID:         "user-org-test",
		OrganizationID: &orgID,
	})

	if err != nil {
		t.Fatalf("SyncUserTeams failed: %v", err)
	}

	// Verify organization membership created
	membership, _ := store.GetOrganizationMembership(ctx, "user-org-test", "org-1")
	if membership == nil {
		t.Error("Expected organization membership to be created")
	}
}
