package auth

import (
	"context"
	"testing"
	"time"
)

func TestMemorySSOConfigStore_CRUD(t *testing.T) {
	store := NewMemorySSOConfigStore()
	ctx := context.Background()

	// Initially empty
	config, err := store.GetSSOConfig(ctx)
	if err != nil {
		t.Fatalf("GetSSOConfig failed: %v", err)
	}
	if config != nil {
		t.Error("Expected nil config initially")
	}

	// Save config
	now := time.Now()
	newConfig := &SSOConfig{
		ID:        "sso_config",
		CreatedAt: now,
		UpdatedAt: now,
		SSOSettings: SSOSettings{
			DefaultTeamID: "team-default",
			RoleMappings: &RoleMappings{
				ProxyAdminRoles:   []string{"admin", "super-admin"},
				InternalUserRoles: []string{"user", "developer"},
				DefaultRole:       "internal_user",
				UseRoleHierarchy:  true,
			},
		},
	}

	err = store.SaveSSOConfig(ctx, newConfig)
	if err != nil {
		t.Fatalf("SaveSSOConfig failed: %v", err)
	}

	// Get config
	retrieved, err := store.GetSSOConfig(ctx)
	if err != nil {
		t.Fatalf("GetSSOConfig failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected config to exist")
	}
	if retrieved.SSOSettings.DefaultTeamID != "team-default" {
		t.Errorf("DefaultTeamID mismatch: got %s", retrieved.SSOSettings.DefaultTeamID)
	}
	if len(retrieved.SSOSettings.RoleMappings.ProxyAdminRoles) != 2 {
		t.Errorf("Expected 2 proxy admin roles, got %d", len(retrieved.SSOSettings.RoleMappings.ProxyAdminRoles))
	}

	// Delete config
	err = store.DeleteSSOConfig(ctx)
	if err != nil {
		t.Fatalf("DeleteSSOConfig failed: %v", err)
	}

	config, _ = store.GetSSOConfig(ctx)
	if config != nil {
		t.Error("Expected nil config after delete")
	}
}

func TestSSOConfigManager_Caching(t *testing.T) {
	store := NewMemorySSOConfigStore()
	manager := NewSSOConfigManager(store, 100*time.Millisecond)
	ctx := context.Background()

	// Save config
	config := &SSOConfig{
		SSOSettings: SSOSettings{
			DefaultTeamID: "team-1",
		},
	}
	_ = manager.UpdateConfig(ctx, config)

	// Get should use cache
	retrieved, _ := manager.GetConfig(ctx)
	if retrieved.SSOSettings.DefaultTeamID != "team-1" {
		t.Errorf("Expected team-1, got %s", retrieved.SSOSettings.DefaultTeamID)
	}

	// Modify directly in store (bypassing manager)
	_ = store.SaveSSOConfig(ctx, &SSOConfig{
		SSOSettings: SSOSettings{
			DefaultTeamID: "team-2",
		},
	})

	// Should still get cached value
	retrieved, _ = manager.GetConfig(ctx)
	if retrieved.SSOSettings.DefaultTeamID != "team-1" {
		t.Errorf("Expected cached team-1, got %s", retrieved.SSOSettings.DefaultTeamID)
	}

	// Wait for cache expiry
	time.Sleep(150 * time.Millisecond)

	// Should now get new value
	retrieved, _ = manager.GetConfig(ctx)
	if retrieved.SSOSettings.DefaultTeamID != "team-2" {
		t.Errorf("Expected team-2 after cache expiry, got %s", retrieved.SSOSettings.DefaultTeamID)
	}
}

func TestSSOConfigManager_InvalidateCache(t *testing.T) {
	store := NewMemorySSOConfigStore()
	manager := NewSSOConfigManager(store, 5*time.Minute)
	ctx := context.Background()

	// Save and cache config
	_ = manager.UpdateConfig(ctx, &SSOConfig{
		SSOSettings: SSOSettings{DefaultTeamID: "team-a"},
	})

	// Verify cached
	_, _ = manager.GetConfig(ctx)

	// Modify in store
	_ = store.SaveSSOConfig(ctx, &SSOConfig{
		SSOSettings: SSOSettings{DefaultTeamID: "team-b"},
	})

	// Invalidate cache
	manager.InvalidateCache()

	// Should get new value
	retrieved, _ := manager.GetConfig(ctx)
	if retrieved.SSOSettings.DefaultTeamID != "team-b" {
		t.Errorf("Expected team-b after invalidation, got %s", retrieved.SSOSettings.DefaultTeamID)
	}
}

func TestRoleMappings_MapRoleFromClaims(t *testing.T) {
	rm := &RoleMappings{
		ProxyAdminRoles:   []string{"admin", "super-admin"},
		OrgAdminRoles:     []string{"org-admin"},
		InternalUserRoles: []string{"user", "developer"},
		TeamRoles:         []string{"team-member"},
		DefaultRole:       "internal_user",
		UseRoleHierarchy:  true,
	}

	tests := []struct {
		name       string
		claimRoles []string
		expected   UserRole
	}{
		{
			name:       "proxy admin match",
			claimRoles: []string{"admin"},
			expected:   UserRoleProxyAdmin,
		},
		{
			name:       "org admin match",
			claimRoles: []string{"org-admin"},
			expected:   UserRoleOrgAdmin,
		},
		{
			name:       "internal user match",
			claimRoles: []string{"developer"},
			expected:   UserRoleInternalUser,
		},
		{
			name:       "hierarchy - highest wins",
			claimRoles: []string{"team-member", "admin", "developer"},
			expected:   UserRoleProxyAdmin,
		},
		{
			name:       "no match - default",
			claimRoles: []string{"unknown-role"},
			expected:   UserRoleInternalUser,
		},
		{
			name:       "empty claims - default",
			claimRoles: []string{},
			expected:   UserRoleInternalUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rm.MapRoleFromClaims(tt.claimRoles)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRoleMappings_NoHierarchy(t *testing.T) {
	rm := &RoleMappings{
		ProxyAdminRoles:   []string{"admin"},
		InternalUserRoles: []string{"user"},
		UseRoleHierarchy:  false,
	}

	// First match wins (even if lower priority)
	// In this case, the order in roleChecks determines the result
	result := rm.MapRoleFromClaims([]string{"user", "admin"})
	// admin is checked first, so it should still win
	if result != UserRoleProxyAdmin {
		t.Errorf("Expected proxy_admin, got %s", result)
	}
}

func TestSSOSettings_JSON(t *testing.T) {
	settings := SSOSettings{
		DefaultTeamID: "team-123",
		RoleMappings: &RoleMappings{
			ProxyAdminRoles: []string{"admin"},
			DefaultRole:     "internal_user",
		},
		GeneralSettings: &GeneralSSOSettings{
			UserIDUpsert: true,
			EnforceRbac:  true,
		},
	}

	// Serialize
	data, err := settings.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize
	var parsed SSOSettings
	err = parsed.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if parsed.DefaultTeamID != "team-123" {
		t.Errorf("DefaultTeamID mismatch: got %s", parsed.DefaultTeamID)
	}
	if parsed.RoleMappings == nil {
		t.Fatal("RoleMappings should not be nil")
	}
	if parsed.GeneralSettings == nil || !parsed.GeneralSettings.UserIDUpsert {
		t.Error("GeneralSettings.UserIDUpsert should be true")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		set      []string
		subset   []string
		expected bool
	}{
		{[]string{"a", "b", "c"}, []string{"b"}, true},
		{[]string{"a", "b", "c"}, []string{"d"}, false},
		{[]string{"a", "b", "c"}, []string{"d", "a"}, true},
		{[]string{}, []string{"a"}, false},
		{[]string{"a"}, []string{}, false},
	}

	for i, tt := range tests {
		result := containsAny(tt.set, tt.subset)
		if result != tt.expected {
			t.Errorf("Case %d: expected %v, got %v", i, tt.expected, result)
		}
	}
}
