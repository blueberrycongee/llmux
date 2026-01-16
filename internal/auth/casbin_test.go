package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCasbinEnforcer(t *testing.T) {
	ce, err := NewCasbinEnforcer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, ce)

	err = ce.AddDefaultPolicies()
	assert.NoError(t, err)

	tests := []struct {
		sub      string
		obj      string
		act      string
		expected bool
	}{
		// Proxy Admin
		{RoleSub(string(UserRoleProxyAdmin)), "/any/path", "GET", true},
		{RoleSub(string(UserRoleProxyAdmin)), "/any/path", "POST", true},
		{RoleSub(string(UserRoleProxyAdmin)), ModelObj("gpt-4"), ActionUse(), true},

		// Proxy Admin Viewer
		{RoleSub(string(UserRoleProxyAdminViewer)), "/any/path", "GET", true},
		{RoleSub(string(UserRoleProxyAdminViewer)), "/any/path", "POST", false},
		{RoleSub(string(UserRoleProxyAdminViewer)), ModelObj("gpt-4"), ActionUse(), false},

		// Management Key
		{RoleSub(string(KeyTypeManagement)), "/key/list", "GET", true},
		{RoleSub(string(KeyTypeManagement)), "/key/create", "POST", true},

		// ReadOnly Key
		{RoleSub(string(KeyTypeReadOnly)), "/v1/models", "GET", true},
		{RoleSub(string(KeyTypeReadOnly)), "/v1/chat/completions", "POST", false},

		// LLM API Key
		{RoleSub(string(KeyTypeLLMAPI)), "/v1/chat/completions", "POST", true},
		{RoleSub(string(KeyTypeLLMAPI)), "/v1/models", "GET", false},

		// Custom Policy
		{"key:custom", "model:gpt-3.5-turbo", "use", true},
		{"key:custom", "model:gpt-4", "use", false},
	}

	for _, tt := range tests {
		t.Run(tt.sub+"_"+tt.obj+"_"+tt.act, func(t *testing.T) {
			if tt.sub == "key:custom" && tt.expected {
				_, _ = ce.AddPolicy(tt.sub, tt.obj, tt.act)
				defer func() { _, _ = ce.enforcer.RemovePolicy(tt.sub, tt.obj, tt.act) }()
			}
			allowed, err := ce.Enforce(tt.sub, tt.obj, tt.act)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestCasbinRBAC(t *testing.T) {
	ce, err := NewCasbinEnforcer(nil)
	assert.NoError(t, err)

	// User -> Role
	_, _ = ce.AddRoleForUser("user:alice", RoleSub(string(UserRoleProxyAdmin)))
	// Role -> Permissions
	_, _ = ce.AddPolicy(RoleSub(string(UserRoleProxyAdmin)), "/*", "*")

	allowed, err := ce.Enforce("user:alice", "/any", "POST")
	assert.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = ce.Enforce("user:bob", "/any", "POST")
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestCasbinModelAccess(t *testing.T) {
	ce, err := NewCasbinEnforcer(nil)
	assert.NoError(t, err)

	keySub := KeySub("test-key")
	teamSub := TeamSub("test-team")

	// Key belongs to Team
	_, _ = ce.enforcer.AddGroupingPolicy(keySub, teamSub)

	// Team has access to a model
	_, _ = ce.AddPolicy(teamSub, ModelObj("claude-3"), ActionUse())

	// Check if key can access it
	allowed, err := ce.Enforce(keySub, ModelObj("claude-3"), ActionUse())
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Check if key can access another model
	allowed, err = ce.Enforce(keySub, ModelObj("gpt-4"), ActionUse())
	assert.NoError(t, err)
	assert.False(t, allowed)
}
