package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/tests/testutil"
)

func TestOrgManagement_MemberLifecycle(t *testing.T) {
	// Setup
	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithAuth(),
	)
	require.NoError(t, err)
	defer server.Stop()
	require.NoError(t, server.Start())

	// Seed Admin Key
	adminKey := &auth.APIKey{
		ID:        "admin-key-id",
		KeyHash:   auth.HashKey("sk-admin-secret"),
		KeyPrefix: "sk-admin",
		KeyType:   auth.KeyTypeManagement,
		IsActive:  true,
	}
	err = server.Store().CreateAPIKey(context.Background(), adminKey)
	require.NoError(t, err)

	client := testutil.NewTestClient(server.URL()).WithAPIKey("sk-admin-secret")

	// 1. Create Organization
	orgReq := map[string]interface{}{
		"organization_alias": "test-org",
		"max_budget":         100.0,
	}
	orgResp, err := client.PostJSON(context.Background(), "/organization/new", orgReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, orgResp.StatusCode)

	var org struct {
		OrganizationID string `json:"organization_id"`
	}
	json.NewDecoder(orgResp.Body).Decode(&org)
	orgID := org.OrganizationID
	require.NotEmpty(t, orgID)

	// 2. Create User
	userReq := map[string]interface{}{
		"user_email": "member@test.com",
		"user_role":  "internal_user",
	}
	userResp, err := client.PostJSON(context.Background(), "/user/new", userReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, userResp.StatusCode)

	var user struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(userResp.Body).Decode(&user)
	userID := user.UserID
	require.NotEmpty(t, userID)

	// 3. Add Member to Org
	addMemberReq := map[string]interface{}{
		"organization_id": orgID,
		"members": []map[string]interface{}{
			{
				"user_id":   userID,
				"user_role": "org_member",
			},
		},
	}
	addResp, err := client.PostJSON(context.Background(), "/organization/member_add", addMemberReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, addResp.StatusCode)

	// 4. Verify Membership (List Members)
	listResp, err := client.GetJSON(context.Background(), "/organization/members?organization_id="+orgID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)

	// 5. Remove Member
	delMemberReq := map[string]interface{}{
		"organization_id": orgID,
		"user_ids":        []string{userID},
	}
	delResp, err := client.PostJSON(context.Background(), "/organization/member_delete", delMemberReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, delResp.StatusCode)
}

func TestOrgManagement_BudgetControl(t *testing.T) {
	t.Skip("Skipping TestOrgManagement_BudgetControl: Budget control not yet integrated in handler")
	// Setup
	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithAuth(),
	)
	require.NoError(t, err)
	defer server.Stop()
	require.NoError(t, server.Start())

	// Seed Admin Key
	adminKey := &auth.APIKey{
		ID:        "admin-key-id",
		KeyHash:   auth.HashKey("sk-admin-secret"),
		KeyPrefix: "sk-admin",
		KeyType:   auth.KeyTypeManagement,
		IsActive:  true,
	}
	server.Store().CreateAPIKey(context.Background(), adminKey)
	adminClient := testutil.NewTestClient(server.URL()).WithAPIKey("sk-admin-secret")

	// 1. Create Team with Budget
	teamReq := map[string]interface{}{
		"team_alias": "budget-team",
		"max_budget": 0.001, // Very small budget
	}
	teamResp, _ := adminClient.PostJSON(context.Background(), "/team/new", teamReq)
	var team struct {
		TeamID string `json:"team_id"`
	}
	json.NewDecoder(teamResp.Body).Decode(&team)
	teamID := team.TeamID

	// 2. Create Key for Team
	keyReq := map[string]interface{}{
		"team_id": teamID,
		"role":    "user",
	}
	keyResp, _ := adminClient.PostJSON(context.Background(), "/key/generate", keyReq)
	var key struct {
		Key string `json:"key"`
	}
	json.NewDecoder(keyResp.Body).Decode(&key)

	userClient := testutil.NewTestClient(server.URL()).WithAPIKey(key.Key)

	// 3. Make Request (Should succeed first)
	_, httpResp, err := userClient.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	// 4. Update Team Spend (Simulate exhaustion)
	err = server.Store().UpdateTeamSpent(context.Background(), teamID, 10.0) // Exceeds 0.001
	require.NoError(t, err)

	// 5. Make Request (Should fail)
	_, httpResp2, err := userClient.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)

	assert.NotEqual(t, http.StatusOK, httpResp2.StatusCode)
}
