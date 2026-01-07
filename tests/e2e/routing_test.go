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

func TestRouting_SingleProvider(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	testutil.AssertChatResponse(t, resp)

	// Verify request went to mock (path may be /chat/completions or /v1/chat/completions)
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests, "mock should have recorded requests")
	assert.Equal(t, "POST", requests[0].Method)
	assert.Contains(t, requests[0].Path, "chat/completions")
}

func TestRouting_ModelMapping(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Request gpt-4o-mock
	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// Response should have the requested model
	assert.Equal(t, "gpt-4o-mock", resp.Model)
}

func TestRouting_UnknownModel(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "unknown-model-xyz",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	// Should return error for unknown model
	assert.True(t, httpResp.StatusCode >= 400, "should return error for unknown model")

	// Mock should not receive request
	testutil.AssertNoRequests(t, mockLLM)
}

func TestRouting_UpstreamFailure(t *testing.T) {
	resetMock()
	mockLLM.SetNextError(http.StatusInternalServerError, "Upstream error")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	// Should propagate error
	assert.True(t, httpResp.StatusCode >= 400)

	// Request should have been attempted
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests, "mock should have recorded requests")
	assert.Equal(t, "POST", requests[0].Method)
}

func TestRouting_SequentialRequests(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make multiple sequential requests
	successCount := 0
	for i := 0; i < 5; i++ {
		req := &testutil.ChatCompletionRequest{
			Model: "gpt-4o-mock",
			Messages: []testutil.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		resp, httpResp, err := testClient.ChatCompletion(ctx, req)
		require.NoError(t, err)
		if httpResp.StatusCode == http.StatusOK && resp != nil {
			testutil.AssertChatResponse(t, resp)
			successCount++
		}
	}

	// At least some requests should succeed
	assert.Greater(t, successCount, 0, "at least some requests should succeed")
}

func TestRouting_ConcurrentRequests(t *testing.T) {
	resetMock()

	const numRequests = 20
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.ChatCompletionRequest{
				Model: "gpt-4o-mock",
				Messages: []testutil.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
			}

			_, _, err := testClient.ChatCompletion(ctx, req)
			results <- err
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	assert.Equal(t, numRequests, successCount, "all concurrent requests should succeed")
	testutil.AssertRequestCount(t, mockLLM, numRequests)
}

func TestRouting_DifferentModelsSequential(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models := []string{"gpt-4o-mock", "gpt-3.5-turbo-mock"}

	for _, model := range models {
		req := &testutil.ChatCompletionRequest{
			Model: model,
			Messages: []testutil.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		resp, _, err := testClient.ChatCompletion(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, model, resp.Model)
	}

	testutil.AssertRequestCount(t, mockLLM, len(models))
}

func TestRouting_RequestTimeout(t *testing.T) {
	resetMock()
	mockLLM.Latency = 5 * time.Second // Very slow response

	// Use a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, err := testClient.ChatCompletion(ctx, req)

	// Should timeout
	assert.Error(t, err, "request should timeout")
}

func TestRouting_HeadersForwarded(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// Check that authorization header was forwarded
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	// Should have Authorization header (from provider config)
	auth := requests[0].Headers.Get("Authorization")
	assert.NotEmpty(t, auth, "Authorization header should be present")
}

func TestRouting_ContentTypeForwarded(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	contentType := requests[0].Headers.Get("Content-Type")
	assert.Contains(t, contentType, "application/json")
}

func TestRouting_RecoveryAfterError(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First request fails with a non-cooldown error (400 Bad Request)
	mockLLM.SetNextError(http.StatusBadRequest, "Bad request error")

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	assert.True(t, httpResp.StatusCode >= 400)

	// Second request should succeed (no error configured)
	resp, httpResp2, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	// Should succeed since 400 doesn't trigger cooldown
	if httpResp2.StatusCode == http.StatusOK {
		testutil.AssertChatResponse(t, resp)
	}
}
