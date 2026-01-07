//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

func TestSmoke_ServerStarts(t *testing.T) {
	// Server is already started in TestMain
	assert.NotEmpty(t, testServer.URL(), "server URL should not be empty")
}

func TestSmoke_HealthCheck_Live(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := testClient.HealthCheck(ctx, "/health/live")
	require.NoError(t, err)
	defer resp.Body.Close()

	testutil.RequireStatusOK(t, resp)
	testutil.AssertJSONResponse(t, resp)
}

func TestSmoke_HealthCheck_Ready(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := testClient.HealthCheck(ctx, "/health/ready")
	require.NoError(t, err)
	defer resp.Body.Close()

	testutil.RequireStatusOK(t, resp)
	testutil.AssertJSONResponse(t, resp)
}

func TestSmoke_MetricsEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics, err := testClient.GetMetrics(ctx)
	require.NoError(t, err)

	// Verify Prometheus format
	assert.Contains(t, metrics, "# HELP", "metrics should contain HELP comments")
	assert.Contains(t, metrics, "# TYPE", "metrics should contain TYPE comments")
}

func TestSmoke_ListModels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := testClient.ListModels(ctx)
	require.NoError(t, err)
	defer resp.Body.Close()

	testutil.RequireStatusOK(t, resp)
	testutil.AssertJSONResponse(t, resp)
}

func TestSmoke_InvalidEndpoint_Returns404(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", testServer.URL()+"/nonexistent", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSmoke_MockLLMServer_Responds(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Direct request to mock server
	req, err := http.NewRequestWithContext(ctx, "GET", mockLLM.URL()+"/health", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSmoke_MockLLMServer_RecordsRequests(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make a request through LLMux to mock
	chatReq := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, err := testClient.ChatCompletion(ctx, chatReq)
	require.NoError(t, err)

	// Verify mock recorded the request
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1, "mock should have recorded 1 request")
	assert.Equal(t, "POST", requests[0].Method)
	assert.Equal(t, "/chat/completions", requests[0].Path)
}

func TestSmoke_ConcurrentRequests(t *testing.T) {
	resetMock()

	const numRequests = 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			chatReq := &testutil.ChatCompletionRequest{
				Model: "gpt-4o-mock",
				Messages: []testutil.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
			}

			_, _, err := testClient.ChatCompletion(ctx, chatReq)
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < numRequests; i++ {
		err := <-results
		assert.NoError(t, err, "concurrent request %d failed", i)
	}

	// Verify all requests were recorded
	requests := mockLLM.GetRequests()
	assert.Len(t, requests, numRequests, "all requests should be recorded")
}

func TestSmoke_ServerHandlesSlowUpstream(t *testing.T) {
	resetMock()
	mockLLM.Latency = 500 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	chatReq := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, chatReq)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, elapsed, 500*time.Millisecond, "request should take at least 500ms")
}

func TestSmoke_MetricsAfterRequests(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make a request
	chatReq := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	_, _, err := testClient.ChatCompletion(ctx, chatReq)
	require.NoError(t, err)

	// Check metrics
	metrics, err := testClient.GetMetrics(ctx)
	require.NoError(t, err)

	// Should have request metrics
	assert.True(t, strings.Contains(metrics, "http_") || strings.Contains(metrics, "llmux_"),
		"metrics should contain HTTP or LLMux metrics")
}
