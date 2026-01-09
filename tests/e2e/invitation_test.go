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

func TestInvitation_Flow(t *testing.T) {
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

	// Seed Admin
	adminKey := &auth.APIKey{
		ID:        "admin-key-id",
		KeyHash:   auth.HashKey("sk-admin-secret"),
		KeyPrefix: "sk-admin",
		KeyType:   auth.KeyTypeManagement,
		IsActive:  true,
	}
	server.Store().CreateAPIKey(context.Background(), adminKey)
	adminClient := testutil.NewTestClient(server.URL()).WithAPIKey("sk-admin-secret")

	// 1. Create Team
	teamReq := map[string]interface{}{"team_alias": "invite-team"}
	teamResp, _ := adminClient.PostJSON(context.Background(), "/team/new", teamReq)
	var team struct {
		TeamID string `json:"team_id"`
	}
	json.NewDecoder(teamResp.Body).Decode(&team)
	teamID := team.TeamID

	// 2. Create Invitation Link
	inviteReq := map[string]interface{}{
		"team_id":    teamID,
		"role":       "member",
		"expires_in": 24,
		"max_uses":   5,
	}
	inviteResp, err := adminClient.PostJSON(context.Background(), "/invitation/new", inviteReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, inviteResp.StatusCode)

	var invite struct {
		ID    string `json:"id"`
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	json.NewDecoder(inviteResp.Body).Decode(&invite)
	require.NotEmpty(t, invite.Token)

	// 3. Create New User (Acceptor)
	userReq := map[string]interface{}{"user_email": "newbie@test.com", "user_role": "internal_user"}
	userResp, _ := adminClient.PostJSON(context.Background(), "/user/new", userReq)
	var user struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(userResp.Body).Decode(&user)
	userID := user.UserID

	// 4. Accept Invitation
	acceptReq := map[string]interface{}{
		"token":   invite.Token,
		"user_id": userID,
	}
	acceptResp, err := adminClient.PostJSON(context.Background(), "/invitation/accept", acceptReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, acceptResp.StatusCode)

	var acceptResult struct {
		Success bool   `json:"success"`
		TeamID  string `json:"team_id"`
	}
	json.NewDecoder(acceptResp.Body).Decode(&acceptResult)
	assert.True(t, acceptResult.Success)
	assert.Equal(t, teamID, acceptResult.TeamID)

	// 5. Verify Membership in Store
	membership, err := server.Store().GetTeamMembership(context.Background(), userID, teamID)
	require.NoError(t, err)
	require.NotNil(t, membership)
	assert.Equal(t, "member", membership.Role)
}

func TestInvitation_Expired(t *testing.T) {
	// Setup
	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()
	server, _ := testutil.NewTestServer(testutil.WithMockProvider(mockServer.URL()), testutil.WithAuth())
	defer server.Stop()
	server.Start()

	adminKey := &auth.APIKey{ID: "admin", KeyHash: auth.HashKey("sk-admin"), KeyType: auth.KeyTypeManagement, IsActive: true}
	server.Store().CreateAPIKey(context.Background(), adminKey)
	client := testutil.NewTestClient(server.URL()).WithAPIKey("sk-admin")

	// Create Team
	teamResp, _ := client.PostJSON(context.Background(), "/team/new", map[string]interface{}{"team_alias": "t"})
	var team struct {
		TeamID string `json:"team_id"`
	}
	json.NewDecoder(teamResp.Body).Decode(&team)

	// Test Max Uses
	userReq := map[string]interface{}{"user_email": "u1@t.com", "user_role": "internal_user"}
	u1Resp, _ := client.PostJSON(context.Background(), "/user/new", userReq)
	var u1 struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(u1Resp.Body).Decode(&u1)

	userReq2 := map[string]interface{}{"user_email": "u2@t.com", "user_role": "internal_user"}
	u2Resp, _ := client.PostJSON(context.Background(), "/user/new", userReq2)
	var u2 struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(u2Resp.Body).Decode(&u2)

	// Create Invite with MaxUses=1
	inviteReq := map[string]interface{}{"team_id": team.TeamID, "role": "member", "max_uses": 1}
	inviteResp, _ := client.PostJSON(context.Background(), "/invitation/new", inviteReq)
	var invite struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	json.NewDecoder(inviteResp.Body).Decode(&invite)

	// Use 1 (Success)
	acceptResp1, _ := client.PostJSON(context.Background(), "/invitation/accept", map[string]interface{}{"token": invite.Token, "user_id": u1.UserID})
	assert.Equal(t, http.StatusOK, acceptResp1.StatusCode)
	var res1 struct{ Success bool }
	json.NewDecoder(acceptResp1.Body).Decode(&res1)
	assert.True(t, res1.Success)

	// Use 2 (Fail - should return BadRequest because max uses reached)
	acceptResp2, _ := client.PostJSON(context.Background(), "/invitation/accept", map[string]interface{}{"token": invite.Token, "user_id": u2.UserID})
	assert.Equal(t, http.StatusBadRequest, acceptResp2.StatusCode)

	// Verify error message
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(acceptResp2.Body).Decode(&errorResp)
	assert.Contains(t, errorResp.Error.Message, "maximum uses")
}
