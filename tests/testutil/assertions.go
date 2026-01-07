package testutil

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertStatusCode asserts the HTTP response status code.
func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	assert.Equal(t, expected, resp.StatusCode, "unexpected status code")
}

// AssertContentType asserts the Content-Type header.
func AssertContentType(t *testing.T, resp *http.Response, expected string) {
	t.Helper()
	contentType := resp.Header.Get("Content-Type")
	assert.True(t, strings.HasPrefix(contentType, expected),
		"expected Content-Type to start with %q, got %q", expected, contentType)
}

// AssertJSONResponse asserts the response is JSON.
func AssertJSONResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertContentType(t, resp, "application/json")
}

// AssertSSEResponse asserts the response is SSE.
func AssertSSEResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertContentType(t, resp, "text/event-stream")
}

// RequireStatusOK requires the response status to be 200 OK.
func RequireStatusOK(t *testing.T, resp *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK")
}

// AssertChatResponse validates a chat completion response.
func AssertChatResponse(t *testing.T, resp *ChatCompletionResponse) {
	t.Helper()
	require.NotNil(t, resp, "response should not be nil")
	assert.NotEmpty(t, resp.ID, "response ID should not be empty")
	assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
	assert.NotZero(t, resp.Created, "created timestamp should not be zero")
	assert.NotEmpty(t, resp.Model, "model should not be empty")
	assert.NotEmpty(t, resp.Choices, "choices should not be empty")

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		assert.Equal(t, "assistant", choice.Message.Role, "message role should be assistant")
		assert.NotEmpty(t, choice.FinishReason, "finish_reason should not be empty")
	}
}

// AssertHasUsage validates that usage information is present.
func AssertHasUsage(t *testing.T, resp *ChatCompletionResponse) {
	t.Helper()
	require.NotNil(t, resp.Usage, "usage should not be nil")
	assert.Greater(t, resp.Usage.PromptTokens, 0, "prompt_tokens should be > 0")
	assert.Greater(t, resp.Usage.CompletionTokens, 0, "completion_tokens should be > 0")
	assert.Equal(t, resp.Usage.PromptTokens+resp.Usage.CompletionTokens, resp.Usage.TotalTokens,
		"total_tokens should equal prompt_tokens + completion_tokens")
}

// AssertRequestRecorded checks that a request was recorded by the mock server.
// Path matching is flexible - matches if the recorded path contains the expected path.
func AssertRequestRecorded(t *testing.T, mock *MockLLMServer, method, path string) {
	t.Helper()
	requests := mock.GetRequests()
	for _, req := range requests {
		if req.Method == method && (req.Path == path || strings.HasSuffix(req.Path, path)) {
			return
		}
	}
	t.Errorf("expected request %s %s to be recorded, got %d requests", method, path, len(requests))
}

// AssertNoRequests checks that no requests were recorded.
func AssertNoRequests(t *testing.T, mock *MockLLMServer) {
	t.Helper()
	requests := mock.GetRequests()
	assert.Empty(t, requests, "expected no requests, got %d", len(requests))
}

// AssertRequestCount checks the number of recorded requests.
func AssertRequestCount(t *testing.T, mock *MockLLMServer, expected int) {
	t.Helper()
	requests := mock.GetRequests()
	assert.Len(t, requests, expected, "unexpected request count")
}
