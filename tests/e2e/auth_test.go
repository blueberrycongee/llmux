//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/tests/testutil"
)

// TestAuth_ValidAPIKey tests that valid API key is accepted.
func TestAuth_ValidAPIKey(t *testing.T) {
	// Create a server with auth enabled
	localMock := testutil.NewMockLLMServer()
	defer localMock.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
		testutil.WithAuth(),
	)
	if err != nil {
		t.Skipf("Auth setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	err = server.Store().CreateAPIKey(context.Background(), &auth.APIKey{
		ID:       "valid-test-key-id",
		KeyHash:  auth.HashKey("valid-test-key"),
		KeyType:  auth.KeyTypeLLMAPI,
		IsActive: true,
	})
	require.NoError(t, err)

	client := testutil.NewTestClient(server.URL()).WithAPIKey("valid-test-key")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, httpResp, err := client.ChatCompletion(ctx, req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	testutil.AssertChatResponse(t, resp)
}

// TestAuth_InvalidAPIKey tests that invalid API key is rejected.
func TestAuth_InvalidAPIKey(t *testing.T) {
	localMock := testutil.NewMockLLMServer()
	defer localMock.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
		testutil.WithAuth(),
	)
	if err != nil {
		t.Skipf("Auth setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	// Client with invalid API key
	client := testutil.NewTestClient(server.URL()).WithAPIKey("invalid-key-12345")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := client.ChatCompletion(ctx, req)
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
}

// TestAuth_MissingAPIKey tests that missing API key is rejected.
func TestAuth_MissingAPIKey(t *testing.T) {
	localMock := testutil.NewMockLLMServer()
	defer localMock.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
		testutil.WithAuth(),
	)
	if err != nil {
		t.Skipf("Auth setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	// Client without API key
	client := testutil.NewTestClient(server.URL())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := client.ChatCompletion(ctx, req)
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, httpResp.StatusCode)
}

// TestAuth_HealthEndpointsSkipAuth tests that health endpoints don't require auth.
func TestAuth_HealthEndpointsSkipAuth(t *testing.T) {
	localMock := testutil.NewMockLLMServer()
	defer localMock.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
		testutil.WithAuth(),
	)
	if err != nil {
		t.Skipf("Auth setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	// Client without API key
	client := testutil.NewTestClient(server.URL())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Health endpoints should work without auth
	resp, err := client.HealthCheck(ctx, "/health/live")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "health endpoint should not require auth")
}

// TestAuth_MetricsEndpointRequiresAuth tests that metrics endpoint requires auth when auth is enabled.
func TestAuth_MetricsEndpointRequiresAuth(t *testing.T) {
	localMock := testutil.NewMockLLMServer()
	defer localMock.Close()

	server, err := testutil.NewTestServer(
		testutil.WithMockProvider(localMock.URL()),
		testutil.WithModels("gpt-4o-mock"),
		testutil.WithAuth(),
	)
	if err != nil {
		t.Skipf("Auth setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	// Client without API key
	client := testutil.NewTestClient(server.URL())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metricsUnauthed, err := client.GetMetrics(ctx)
	require.NoError(t, err)
	assert.NotContains(t, metricsUnauthed, "# HELP", "metrics should not be accessible without auth when auth is enabled")

	err = server.Store().CreateAPIKey(context.Background(), &auth.APIKey{
		ID:       "metrics-test-key-id",
		KeyHash:  auth.HashKey("metrics-test-key"),
		KeyType:  auth.KeyTypeLLMAPI,
		IsActive: true,
	})
	require.NoError(t, err)

	metricsAuthed, err := client.WithAPIKey("metrics-test-key").GetMetrics(ctx)
	require.NoError(t, err)
	assert.Contains(t, metricsAuthed, "# HELP", "metrics should be accessible with auth")
}

// TestAuth_BearerTokenFormat tests that Bearer token format is accepted.
func TestAuth_BearerTokenFormat(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with Bearer prefix
	clientWithBearer := testutil.NewTestClient(testServer.URL()).WithAPIKey("test-key")

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, err := clientWithBearer.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// Verify Authorization header was sent correctly
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests)

	authHeader := requests[0].Headers.Get("Authorization")
	assert.Contains(t, authHeader, "Bearer", "should use Bearer token format")
}
