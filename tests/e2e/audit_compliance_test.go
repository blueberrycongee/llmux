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

func TestAudit_FullTrace(t *testing.T) {
	t.Skip("Skipping TestAudit_FullTrace: Audit logging not yet integrated in management handler")
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

	// 1. Perform Actions
	// Create Team
	teamReq := map[string]interface{}{"team_alias": "audit-team"}
	adminClient.PostJSON(context.Background(), "/team/new", teamReq)

	// Generate Key
	keyReq := map[string]interface{}{"key_name": "audit-key"}
	adminClient.PostJSON(context.Background(), "/key/generate", keyReq)

	// 2. Verify Audit Logs
	logsResp, err := adminClient.GetJSON(context.Background(), "/audit/logs")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, logsResp.StatusCode)

	var logsResult struct {
		Data []struct {
			Action     string `json:"action"`
			ObjectType string `json:"object_type"`
			ActorID    string `json:"actor_id"`
		} `json:"data"`
	}
	json.NewDecoder(logsResp.Body).Decode(&logsResult)

	// Should have at least 2 logs: create team, create key
	assert.GreaterOrEqual(t, len(logsResult.Data), 2)

	// Verify Actor ID
	for _, log := range logsResult.Data {
		assert.Equal(t, "admin-key-id", log.ActorID)
	}
}

func TestAudit_SensitiveData(t *testing.T) {
	t.Skip("Skipping TestAudit_SensitiveData: Audit logging not yet integrated")
	// Setup
	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()
	server, _ := testutil.NewTestServer(testutil.WithMockProvider(mockServer.URL()), testutil.WithAuth())
	defer server.Stop()
	server.Start()

	adminKey := &auth.APIKey{ID: "admin", KeyHash: auth.HashKey("sk-admin"), KeyType: auth.KeyTypeManagement, IsActive: true}
	server.Store().CreateAPIKey(context.Background(), adminKey)
	client := testutil.NewTestClient(server.URL()).WithAPIKey("sk-admin")

	// Create Key
	keyReq := map[string]interface{}{"key_name": "secret-key"}
	client.PostJSON(context.Background(), "/key/generate", keyReq)

	// Fetch Logs
	logsResp, _ := client.GetJSON(context.Background(), "/audit/logs")
	var logsResult struct {
		Data []struct {
			Action     string                 `json:"action"`
			ObjectType string                 `json:"object_type"`
			AfterValue map[string]interface{} `json:"after_value"`
		} `json:"data"`
	}
	json.NewDecoder(logsResp.Body).Decode(&logsResult)

	// Find key creation log
	var keyLogFound bool
	for _, l := range logsResult.Data {
		if l.Action == "create" && l.ObjectType == "api_key" {
			keyLogFound = true

			_, hasRaw := l.AfterValue["key"]
			_, hasSecret := l.AfterValue["secret"]

			assert.False(t, hasRaw, "should not have raw key")
			assert.False(t, hasSecret, "should not have secret")
		}
	}
	assert.True(t, keyLogFound, "should find key creation log")
}
