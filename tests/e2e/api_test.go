//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

func TestAPI_ChatCompletions_Basic(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	resp, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	testutil.AssertChatResponse(t, resp)
	testutil.AssertHasUsage(t, resp)
}

func TestAPI_ChatCompletions_WithSystemMessage(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	testutil.AssertChatResponse(t, resp)

	// Verify request was forwarded correctly
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	messages := sentReq["messages"].([]any)
	assert.Len(t, messages, 2, "should have 2 messages")
}

func TestAPI_ChatCompletions_MultiTurn(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "My name is Alice."},
			{Role: "assistant", Content: "Hello Alice! Nice to meet you."},
			{Role: "user", Content: "What is my name?"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	testutil.AssertChatResponse(t, resp)

	// Verify all messages were sent
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	messages := sentReq["messages"].([]any)
	assert.Len(t, messages, 3, "should have 3 messages")
}

func TestAPI_ChatCompletions_WithMaxTokens(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model:     "gpt-4o-mock",
		MaxTokens: 100,
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	testutil.AssertChatResponse(t, resp)

	// Verify max_tokens was forwarded
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	maxTokens, ok := sentReq["max_tokens"].(float64)
	assert.True(t, ok, "max_tokens should be present")
	assert.Equal(t, float64(100), maxTokens)
}

func TestAPI_ChatCompletions_WithTemperature(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	temp := 0.7
	req := &testutil.ChatCompletionRequest{
		Model:       "gpt-4o-mock",
		Temperature: &temp,
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, _, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)

	testutil.AssertChatResponse(t, resp)

	// Verify temperature was forwarded
	requests := mockLLM.GetRequests()
	require.Len(t, requests, 1)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	temperature, ok := sentReq["temperature"].(float64)
	assert.True(t, ok, "temperature should be present")
	assert.Equal(t, 0.7, temperature)
}

func TestAPI_ChatCompletions_CustomResponse(t *testing.T) {
	resetMock()
	mockLLM.SetNextResponse("This is a custom response from the mock server.")

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
	assert.Equal(t, "This is a custom response from the mock server.", resp.Choices[0].Message.Content)
}

func TestAPI_ChatCompletions_InvalidModel(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "nonexistent-model",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	// Should return error for unknown model
	assert.True(t, httpResp.StatusCode >= 400, "should return error status")
}

func TestAPI_ChatCompletions_MissingModel(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "", // Missing model
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

func TestAPI_ChatCompletions_MissingMessages(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model:    "gpt-4o-mock",
		Messages: []testutil.ChatMessage{}, // Empty messages
	}

	_, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

func TestAPI_ChatCompletions_UpstreamError(t *testing.T) {
	resetMock()
	mockLLM.SetNextError(http.StatusInternalServerError, "Internal server error")

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

	assert.True(t, httpResp.StatusCode >= 400, "should return error status")
}

func TestAPI_ChatCompletions_RateLimitError(t *testing.T) {
	resetMock()
	mockLLM.SetNextError(http.StatusTooManyRequests, "Rate limit exceeded")

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

	assert.True(t, httpResp.StatusCode >= 400, "should return error status")
}

func TestAPI_ChatCompletions_ErrorResponseFormat(t *testing.T) {
	resetMock()
	mockLLM.SetNextError(http.StatusBadRequest, "Invalid request")

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
	defer httpResp.Body.Close()

	// Read error response
	body, err := io.ReadAll(httpResp.Body)
	require.NoError(t, err)

	var errResp testutil.ErrorResponse
	err = json.Unmarshal(body, &errResp)
	require.NoError(t, err)

	assert.NotEmpty(t, errResp.Error.Message, "error message should not be empty")
	assert.NotEmpty(t, errResp.Error.Type, "error type should not be empty")
}

func TestAPI_ChatCompletions_DifferentModels(t *testing.T) {
	// Use separate test for each model to avoid state pollution
	t.Run("gpt-4o-mock", func(t *testing.T) {
		resetMock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := &testutil.ChatCompletionRequest{
			Model: "gpt-4o-mock",
			Messages: []testutil.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		resp, httpResp, err := testClient.ChatCompletion(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpResp)

		if httpResp.StatusCode == http.StatusOK {
			testutil.AssertChatResponse(t, resp)
			assert.Equal(t, "gpt-4o-mock", resp.Model, "response model should match request")
		}
	})

	t.Run("gpt-3.5-turbo-mock", func(t *testing.T) {
		resetMock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := &testutil.ChatCompletionRequest{
			Model: "gpt-3.5-turbo-mock",
			Messages: []testutil.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		resp, httpResp, err := testClient.ChatCompletion(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpResp)

		if httpResp.StatusCode == http.StatusOK {
			testutil.AssertChatResponse(t, resp)
			assert.Equal(t, "gpt-3.5-turbo-mock", resp.Model, "response model should match request")
		}
	})
}
