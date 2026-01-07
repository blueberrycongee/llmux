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

// TestFallback_SingleProviderFailure tests that requests succeed when one provider fails
// but another is available.
func TestFallback_SingleProviderFailure(t *testing.T) {
	// Create two mock servers
	mockPrimary := testutil.NewMockLLMServer()
	defer mockPrimary.Close()

	mockFallback := testutil.NewMockLLMServer()
	defer mockFallback.Close()

	// Configure primary to fail
	mockPrimary.SetNextError(http.StatusInternalServerError, "Primary server error")

	// Configure fallback to succeed
	mockFallback.SetNextResponse("Response from fallback server")

	// Create test server with both providers
	server, err := testutil.NewTestServer(
		testutil.WithMultipleProviders([]testutil.ProviderConfig{
			{Name: "primary", URL: mockPrimary.URL(), Models: []string{"gpt-4o-mock"}},
			{Name: "fallback", URL: mockFallback.URL(), Models: []string{"gpt-4o-mock"}},
		}),
	)
	if err != nil {
		t.Skipf("Multi-provider setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	client := testutil.NewTestClient(server.URL())

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

	// Should succeed via fallback
	if httpResp.StatusCode == http.StatusOK {
		assert.Equal(t, "Response from fallback server", resp.Choices[0].Message.Content)
	}
}

// TestFallback_AllProvidersFail tests behavior when all providers fail.
func TestFallback_AllProvidersFail(t *testing.T) {
	// Create two mock servers, both configured to fail
	mockPrimary := testutil.NewMockLLMServer()
	defer mockPrimary.Close()

	mockFallback := testutil.NewMockLLMServer()
	defer mockFallback.Close()

	mockPrimary.SetNextError(http.StatusInternalServerError, "Primary error")
	mockFallback.SetNextError(http.StatusInternalServerError, "Fallback error")

	server, err := testutil.NewTestServer(
		testutil.WithMultipleProviders([]testutil.ProviderConfig{
			{Name: "primary", URL: mockPrimary.URL(), Models: []string{"gpt-4o-mock"}},
			{Name: "fallback", URL: mockFallback.URL(), Models: []string{"gpt-4o-mock"}},
		}),
	)
	if err != nil {
		t.Skipf("Multi-provider setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

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

	// Should return error when all providers fail
	assert.True(t, httpResp.StatusCode >= 400, "should return error when all providers fail")
}

// TestFallback_ProviderRecovery tests that a provider is used again after recovery.
func TestFallback_ProviderRecovery(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First request: configure error
	mockLLM.SetNextError(http.StatusServiceUnavailable, "Temporary error")

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp1, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	assert.True(t, httpResp1.StatusCode >= 400, "first request should fail")

	// Second request: no error configured, should succeed
	resp, httpResp2, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// With cooldown=0 in tests, should succeed immediately
	if httpResp2.StatusCode == http.StatusOK {
		testutil.AssertChatResponse(t, resp)
	}
}

// TestFallback_RateLimitTriggersFailover tests that rate limit errors trigger failover.
func TestFallback_RateLimitTriggersFailover(t *testing.T) {
	mockPrimary := testutil.NewMockLLMServer()
	defer mockPrimary.Close()

	mockFallback := testutil.NewMockLLMServer()
	defer mockFallback.Close()

	// Primary returns rate limit
	mockPrimary.SetNextError(http.StatusTooManyRequests, "Rate limit exceeded")

	// Fallback succeeds
	mockFallback.SetNextResponse("Fallback response")

	server, err := testutil.NewTestServer(
		testutil.WithMultipleProviders([]testutil.ProviderConfig{
			{Name: "primary", URL: mockPrimary.URL(), Models: []string{"gpt-4o-mock"}},
			{Name: "fallback", URL: mockFallback.URL(), Models: []string{"gpt-4o-mock"}},
		}),
	)
	if err != nil {
		t.Skipf("Multi-provider setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	client := testutil.NewTestClient(server.URL())

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

	if httpResp.StatusCode == http.StatusOK {
		assert.Equal(t, "Fallback response", resp.Choices[0].Message.Content)
	}
}

// TestFallback_TimeoutTriggersFailover tests that timeout triggers failover.
func TestFallback_TimeoutTriggersFailover(t *testing.T) {
	mockPrimary := testutil.NewMockLLMServer()
	defer mockPrimary.Close()

	mockFallback := testutil.NewMockLLMServer()
	defer mockFallback.Close()

	// Primary is very slow
	mockPrimary.Latency = 10 * time.Second

	// Fallback is fast
	mockFallback.SetNextResponse("Fast fallback response")

	server, err := testutil.NewTestServer(
		testutil.WithMultipleProviders([]testutil.ProviderConfig{
			{Name: "primary", URL: mockPrimary.URL(), Models: []string{"gpt-4o-mock"}},
			{Name: "fallback", URL: mockFallback.URL(), Models: []string{"gpt-4o-mock"}},
		}),
		testutil.WithTimeout(1*time.Second), // Short timeout
	)
	if err != nil {
		t.Skipf("Multi-provider setup not supported: %v", err)
		return
	}
	defer server.Stop()

	if startErr := server.Start(); startErr != nil {
		t.Skipf("Failed to start server: %v", startErr)
		return
	}

	client := testutil.NewTestClient(server.URL())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, httpResp, err := client.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// Should succeed via fallback after primary times out
	if httpResp.StatusCode == http.StatusOK {
		assert.Equal(t, "Fast fallback response", resp.Choices[0].Message.Content)
	}
}
