package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/tests/testutil"
)

func TestOIDC_StandardLogin(t *testing.T) {
	// 1. Setup Mock OIDC Provider
	oidcProvider, err := testutil.NewMockOIDCProvider()
	require.NoError(t, err)
	defer oidcProvider.Close()

	// 2. Setup Test Server with OIDC
	oidcConfig := &config.OIDCConfig{
		IssuerURL:    oidcProvider.URL(),
		ClientID:     "llmux-client-id",
		ClientSecret: "secret",
		ClaimMapping: config.ClaimMapping{
			RoleClaim: "groups",
			Roles: map[string]string{
				"admin-group": "proxy_admin",
				"dev-group":   "internal_user",
			},
		},
		UserIDUpsert: true,
		TeamIDUpsert: true,
	}

	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithOIDC(oidcConfig),
	)
	require.NoError(t, err)
	defer server.Stop()

	require.NoError(t, server.Start())

	// 3. Create Valid Token
	token, err := oidcProvider.SignToken(map[string]interface{}{
		"sub":    "user-123",
		"email":  "test@example.com",
		"groups": []string{"dev-group"},
	})
	require.NoError(t, err)

	// 4. Make Request with Token
	client := testutil.NewTestClient(server.URL())
	client = client.WithAPIKey(token) // This sets Authorization: Bearer <token>

	resp, httpResp, err := client.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	})
	require.NoError(t, err)
	defer httpResp.Body.Close()

	// 5. Verify Success
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)
	testutil.AssertChatResponse(t, resp)
}

func TestOIDC_RoleMapping(t *testing.T) {
	// Setup
	oidcProvider, err := testutil.NewMockOIDCProvider()
	require.NoError(t, err)
	defer oidcProvider.Close()

	oidcConfig := &config.OIDCConfig{
		IssuerURL: oidcProvider.URL(),
		ClientID:  "llmux-client-id",
		ClaimMapping: config.ClaimMapping{
			RoleClaim: "groups",
			Roles: map[string]string{
				"admin-group": "proxy_admin",
			},
		},
		UserIDUpsert: true,
	}

	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithOIDC(oidcConfig),
	)
	require.NoError(t, err)
	defer server.Stop()
	require.NoError(t, server.Start())

	// Test Case: Admin Role
	token, err := oidcProvider.SignToken(map[string]interface{}{
		"sub":    "admin-user",
		"email":  "admin@example.com",
		"groups": []string{"admin-group"},
	})
	require.NoError(t, err)

	client := testutil.NewTestClient(server.URL()).WithAPIKey(token)
	_, httpResp, err := client.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	defer httpResp.Body.Close()
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)
}

func TestOIDC_InvalidToken(t *testing.T) {
	// Setup Server with Provider A
	providerA, err := testutil.NewMockOIDCProvider()
	require.NoError(t, err)
	defer providerA.Close()

	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithOIDC(&config.OIDCConfig{
			IssuerURL: providerA.URL(),
			ClientID:  "llmux-client-id",
		}),
	)
	require.NoError(t, err)
	defer server.Stop()
	require.NoError(t, server.Start())

	// Setup Provider B (Attacker)
	providerB, err := testutil.NewMockOIDCProvider()
	require.NoError(t, err)
	defer providerB.Close()

	// Sign token with Provider B
	token, err := providerB.SignToken(map[string]interface{}{
		"sub": "hacker",
	})
	require.NoError(t, err)

	client := testutil.NewTestClient(server.URL()).WithAPIKey(token)
	_, httpResp, err := client.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	defer httpResp.Body.Close()

	// Should be Unauthorized because signature won't match Provider A's keys
	assert.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
}

func TestOIDC_RoleHierarchy(t *testing.T) {
	// Setup
	oidcProvider, err := testutil.NewMockOIDCProvider()
	require.NoError(t, err)
	defer oidcProvider.Close()

	oidcConfig := &config.OIDCConfig{
		IssuerURL: oidcProvider.URL(),
		ClientID:  "llmux-client-id",
		ClaimMapping: config.ClaimMapping{
			RoleClaim:        "groups",
			UseRoleHierarchy: true,
			Roles: map[string]string{
				"admin-group": "proxy_admin",
				"dev-group":   "internal_user",
			},
		},
		UserIDUpsert: true,
	}

	mockServer := testutil.NewMockLLMServer()
	defer mockServer.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(mockServer.URL()),
		testutil.WithOIDC(oidcConfig),
	)
	require.NoError(t, err)
	defer server.Stop()
	require.NoError(t, server.Start())

	// User with both Admin and Dev groups
	token, err := oidcProvider.SignToken(map[string]interface{}{
		"sub":    "super-user",
		"email":  "super@example.com",
		"groups": []string{"dev-group", "admin-group"}, // Should pick admin (higher priority)
	})
	require.NoError(t, err)

	client := testutil.NewTestClient(server.URL()).WithAPIKey(token)

	_, httpResp, err := client.ChatCompletion(context.Background(), &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	defer httpResp.Body.Close()
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)
}
