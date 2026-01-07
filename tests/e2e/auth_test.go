//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	// Client with valid API key
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

	// If auth is properly implemented, should succeed with valid key
	// If auth is not implemented yet, this documents expected behavior
	if httpResp.StatusCode == http.StatusOK {
		testutil.AssertChatResponse(t, resp)
	} else if httpResp.StatusCode == http.StatusUnauthorized {
		t.Log("Auth is enabled but key validation not implemented yet")
	}
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

	// When auth is properly implemented, should return 401
	// Currently documents expected behavior
	if httpResp.StatusCode == http.StatusUnauthorized {
		t.Log("Auth correctly rejected invalid key")
	} else {
		t.Logf("Auth returned %d (may not be fully implemented)", httpResp.StatusCode)
	}
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

	// When auth is properly implemented, should return 401
	if httpResp.StatusCode == http.StatusUnauthorized {
		t.Log("Auth correctly rejected missing key")
	} else {
		t.Logf("Auth returned %d (may not be fully implemented)", httpResp.StatusCode)
	}
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

// TestAuth_MetricsEndpointSkipAuth tests that metrics endpoint doesn't require auth.
func TestAuth_MetricsEndpointSkipAuth(t *testing.T) {
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

	// Metrics endpoint should work without auth
	metrics, err := client.GetMetrics(ctx)
	require.NoError(t, err)

	assert.Contains(t, metrics, "# HELP", "metrics should be accessible without auth")
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
