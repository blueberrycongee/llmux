package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestInvitationLink_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		link     *InvitationLink
		expected bool
	}{
		{
			name: "active and no limits",
			link: &InvitationLink{
				IsActive: true,
			},
			expected: true,
		},
		{
			name: "inactive",
			link: &InvitationLink{
				IsActive: false,
			},
			expected: false,
		},
		{
			name: "expired",
			link: &InvitationLink{
				IsActive:  true,
				ExpiresAt: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "not expired yet",
			link: &InvitationLink{
				IsActive:  true,
				ExpiresAt: func() *time.Time { t := time.Now().Add(1 * time.Hour); return &t }(),
			},
			expected: true,
		},
		{
			name: "max uses reached",
			link: &InvitationLink{
				IsActive:    true,
				MaxUses:     5,
				CurrentUses: 5,
			},
			expected: false,
		},
		{
			name: "uses under limit",
			link: &InvitationLink{
				IsActive:    true,
				MaxUses:     5,
				CurrentUses: 3,
			},
			expected: true,
		},
		{
			name: "unlimited uses",
			link: &InvitationLink{
				IsActive:    true,
				MaxUses:     0, // 0 = unlimited
				CurrentUses: 100,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.link.IsValid()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMemoryInvitationLinkStore_CRUD(t *testing.T) {
	store := NewMemoryInvitationLinkStore()
	ctx := context.Background()

	teamID := "team-123"
	link := &InvitationLink{
		ID:       "invite-1",
		Token:    "hashed-token-123",
		TeamID:   &teamID,
		Role:     "member",
		IsActive: true,
	}

	// Create
	err := store.CreateInvitationLink(ctx, link)
	if err != nil {
		t.Fatalf("CreateInvitationLink failed: %v", err)
	}

	// Get by ID
	retrieved, err := store.GetInvitationLink(ctx, "invite-1")
	if err != nil {
		t.Fatalf("GetInvitationLink failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected invitation link to exist")
	}
	if *retrieved.TeamID != teamID {
		t.Errorf("TeamID mismatch: got %s", *retrieved.TeamID)
	}

	// Get by token
	byToken, err := store.GetInvitationLinkByToken(ctx, "hashed-token-123")
	if err != nil {
		t.Fatalf("GetInvitationLinkByToken failed: %v", err)
	}
	if byToken == nil {
		t.Fatal("Expected to find invitation by token")
	}

	// Update
	retrieved.Role = "admin"
	err = store.UpdateInvitationLink(ctx, retrieved)
	if err != nil {
		t.Fatalf("UpdateInvitationLink failed: %v", err)
	}

	updated, _ := store.GetInvitationLink(ctx, "invite-1")
	if updated.Role != "admin" {
		t.Errorf("Expected role 'admin', got %s", updated.Role)
	}

	// Delete
	err = store.DeleteInvitationLink(ctx, "invite-1")
	if err != nil {
		t.Fatalf("DeleteInvitationLink failed: %v", err)
	}

	deleted, _ := store.GetInvitationLink(ctx, "invite-1")
	if deleted != nil {
		t.Error("Expected invitation to be deleted")
	}
}

func TestMemoryInvitationLinkStore_List(t *testing.T) {
	store := NewMemoryInvitationLinkStore()
	ctx := context.Background()

	team1 := "team-1"
	team2 := "team-2"

	links := []*InvitationLink{
		{ID: "1", Token: "t1", TeamID: &team1, IsActive: true, CreatedBy: "user-a"},
		{ID: "2", Token: "t2", TeamID: &team1, IsActive: false, CreatedBy: "user-a"},
		{ID: "3", Token: "t3", TeamID: &team2, IsActive: true, CreatedBy: "user-b"},
	}

	for _, link := range links {
		_ = store.CreateInvitationLink(ctx, link)
	}

	// List all
	all, _ := store.ListInvitationLinks(ctx, InvitationLinkFilter{})
	if len(all) != 3 {
		t.Errorf("Expected 3 links, got %d", len(all))
	}

	// Filter by team
	team1Links, _ := store.ListInvitationLinks(ctx, InvitationLinkFilter{TeamID: &team1})
	if len(team1Links) != 2 {
		t.Errorf("Expected 2 links for team-1, got %d", len(team1Links))
	}

	// Filter by active status
	active := true
	activeLinks, _ := store.ListInvitationLinks(ctx, InvitationLinkFilter{IsActive: &active})
	if len(activeLinks) != 2 {
		t.Errorf("Expected 2 active links, got %d", len(activeLinks))
	}

	// Filter by creator
	creatorA := "user-a"
	userALinks, _ := store.ListInvitationLinks(ctx, InvitationLinkFilter{CreatedBy: &creatorA})
	if len(userALinks) != 2 {
		t.Errorf("Expected 2 links by user-a, got %d", len(userALinks))
	}
}

func TestInvitationService_CreateAndAccept(t *testing.T) {
	invStore := NewMemoryInvitationLinkStore()
	authStore := NewMemoryStore()
	service := NewInvitationService(invStore, authStore, slog.Default())
	ctx := context.Background()

	// Create team
	team := &Team{ID: "team-abc", IsActive: true}
	_ = authStore.CreateTeam(ctx, team)

	// Create user
	user := &User{ID: "user-xyz", Role: "internal_user", IsActive: true}
	_ = authStore.CreateUser(ctx, user)

	// Create invitation
	teamID := "team-abc"
	link, token, err := service.CreateInvitationLink(ctx, &CreateInvitationRequest{
		TeamID:    &teamID,
		Role:      "member",
		MaxUses:   10,
		ExpiresIn: 24, // 24 hours
		CreatedBy: "admin-user",
	})

	if err != nil {
		t.Fatalf("CreateInvitationLink failed: %v", err)
	}

	if link == nil || token == "" {
		t.Fatal("Expected link and token to be returned")
	}

	if !link.IsValid() {
		t.Error("Expected new link to be valid")
	}

	// Accept invitation
	result, err := service.AcceptInvitation(ctx, &AcceptInvitationRequest{
		Token:  token,
		UserID: "user-xyz",
	})

	if err != nil {
		t.Fatalf("AcceptInvitation failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got: %s", result.Message)
	}

	// Verify team membership was created
	membership, _ := authStore.GetTeamMembership(ctx, "user-xyz", "team-abc")
	if membership == nil {
		t.Error("Expected team membership to be created")
	}

	// Verify use count was incremented
	updatedLink, _ := invStore.GetInvitationLink(ctx, link.ID)
	if updatedLink.CurrentUses != 1 {
		t.Errorf("Expected CurrentUses to be 1, got %d", updatedLink.CurrentUses)
	}
}

func TestInvitationService_AcceptInvalidToken(t *testing.T) {
	invStore := NewMemoryInvitationLinkStore()
	authStore := NewMemoryStore()
	service := NewInvitationService(invStore, authStore, slog.Default())
	ctx := context.Background()

	result, err := service.AcceptInvitation(ctx, &AcceptInvitationRequest{
		Token:  "invalid-token",
		UserID: "user-123",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for invalid token")
	}

	if result.Message != "invitation not found" {
		t.Errorf("Expected 'invitation not found', got: %s", result.Message)
	}
}

func TestInvitationService_AcceptExpired(t *testing.T) {
	invStore := NewMemoryInvitationLinkStore()
	authStore := NewMemoryStore()
	service := NewInvitationService(invStore, authStore, slog.Default())
	ctx := context.Background()

	// Create expired invitation directly
	expiredTime := time.Now().Add(-1 * time.Hour)
	link := &InvitationLink{
		ID:        "expired-link",
		Token:     hashInvitationToken("expired-token"),
		IsActive:  true,
		ExpiresAt: &expiredTime,
	}
	_ = invStore.CreateInvitationLink(ctx, link)

	result, err := service.AcceptInvitation(ctx, &AcceptInvitationRequest{
		Token:  "expired-token",
		UserID: "user-123",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for expired invitation")
	}
}

func TestInvitationService_DeactivateInvitation(t *testing.T) {
	invStore := NewMemoryInvitationLinkStore()
	authStore := NewMemoryStore()
	service := NewInvitationService(invStore, authStore, slog.Default())
	ctx := context.Background()

	// Create invitation
	teamID := "team-1"
	link, _, _ := service.CreateInvitationLink(ctx, &CreateInvitationRequest{
		TeamID:    &teamID,
		CreatedBy: "admin",
	})

	// Verify active
	if !link.IsActive {
		t.Error("Expected link to be active initially")
	}

	// Deactivate
	err := service.DeactivateInvitation(ctx, link.ID)
	if err != nil {
		t.Fatalf("DeactivateInvitation failed: %v", err)
	}

	// Verify deactivated
	updated, _ := invStore.GetInvitationLink(ctx, link.ID)
	if updated.IsActive {
		t.Error("Expected link to be deactivated")
	}
}

func TestIncrementInvitationLinkUses(t *testing.T) {
	store := NewMemoryInvitationLinkStore()
	ctx := context.Background()

	link := &InvitationLink{
		ID:          "use-test",
		Token:       "token",
		IsActive:    true,
		CurrentUses: 0,
	}
	_ = store.CreateInvitationLink(ctx, link)

	// Increment
	_ = store.IncrementInvitationLinkUses(ctx, "use-test")
	_ = store.IncrementInvitationLinkUses(ctx, "use-test")
	_ = store.IncrementInvitationLinkUses(ctx, "use-test")

	updated, _ := store.GetInvitationLink(ctx, "use-test")
	if updated.CurrentUses != 3 {
		t.Errorf("Expected 3 uses, got %d", updated.CurrentUses)
	}
}
