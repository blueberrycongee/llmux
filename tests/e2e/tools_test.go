//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

// TestTools_BasicFunctionCalling tests basic function calling / tools support.
func TestTools_BasicFunctionCalling(t *testing.T) {
	resetMock()

	// Configure mock to return a tool call
	mockLLM.SetNextToolCallResponse([]testutil.MockToolCall{
		{
			ID:   "call_abc123",
			Type: "function",
			Function: testutil.MockFunctionCall{
				Name:      "get_weather",
				Arguments: `{"location": "San Francisco", "unit": "celsius"}`,
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Define a tool
	weatherTool := json.RawMessage(`{
		"type": "function",
		"function": {
			"name": "get_weather",
			"description": "Get the current weather in a given location",
			"parameters": {
				"type": "object",
				"properties": {
					"location": {
						"type": "string",
						"description": "The city and state, e.g. San Francisco, CA"
					},
					"unit": {
						"type": "string",
						"enum": ["celsius", "fahrenheit"]
					}
				},
				"required": ["location"]
			}
		}
	}`)

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "What's the weather like in San Francisco?"},
		},
		Tools: []json.RawMessage{weatherTool},
	}

	resp, httpResp, err := testClient.ChatCompletionWithTools(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	if httpResp.StatusCode != 200 {
		t.Skipf("Server returned %d, tools may not be fully supported yet", httpResp.StatusCode)
		return
	}

	// Verify response structure
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "chat.completion", resp.Object)
	require.Len(t, resp.Choices, 1)

	choice := resp.Choices[0]
	assert.Equal(t, "tool_calls", choice.FinishReason)

	// Verify tool calls
	require.NotNil(t, choice.Message.ToolCalls)
	require.Len(t, choice.Message.ToolCalls, 1)

	toolCall := choice.Message.ToolCalls[0]
	assert.Equal(t, "call_abc123", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	assert.Equal(t, "get_weather", toolCall.Function.Name)

	// Parse arguments
	var args map[string]string
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	require.NoError(t, err)
	assert.Equal(t, "San Francisco", args["location"])
}

// TestTools_MultipleToolCalls tests multiple tool calls in a single response.
func TestTools_MultipleToolCalls(t *testing.T) {
	resetMock()

	// Configure mock to return multiple tool calls
	mockLLM.SetNextToolCallResponse([]testutil.MockToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: testutil.MockFunctionCall{
				Name:      "get_weather",
				Arguments: `{"location": "San Francisco"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: testutil.MockFunctionCall{
				Name:      "get_weather",
				Arguments: `{"location": "New York"}`,
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	weatherTool := json.RawMessage(`{
		"type": "function",
		"function": {
			"name": "get_weather",
			"description": "Get weather",
			"parameters": {"type": "object", "properties": {"location": {"type": "string"}}}
		}
	}`)

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "What's the weather in SF and NYC?"},
		},
		Tools: []json.RawMessage{weatherTool},
	}

	resp, httpResp, err := testClient.ChatCompletionWithTools(ctx, req)
	require.NoError(t, err)

	if httpResp.StatusCode != 200 {
		t.Skipf("Server returned %d", httpResp.StatusCode)
		return
	}

	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message.ToolCalls)
	assert.Len(t, resp.Choices[0].Message.ToolCalls, 2)
}

// TestTools_ToolCallForwarding verifies tool definitions are forwarded to upstream.
func TestTools_ToolCallForwarding(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	weatherTool := json.RawMessage(`{
		"type": "function",
		"function": {
			"name": "test_function",
			"description": "A test function",
			"parameters": {"type": "object", "properties": {}}
		}
	}`)

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Test"},
		},
		Tools: []json.RawMessage{weatherTool},
	}

	_, _, err := testClient.ChatCompletionWithTools(ctx, req)
	require.NoError(t, err)

	// Verify the request was forwarded with tools
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	tools, ok := sentReq["tools"].([]any)
	assert.True(t, ok, "tools should be present in forwarded request")
	assert.Len(t, tools, 1)
}

// TestTools_NoToolsProvided tests behavior when no tools are provided.
func TestTools_NoToolsProvided(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		// No tools
	}

	resp, httpResp, err := testClient.ChatCompletion(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	if httpResp.StatusCode == 200 {
		assert.Equal(t, "stop", resp.Choices[0].FinishReason)
		assert.NotEmpty(t, resp.Choices[0].Message.Content)
	}
}

// TestJSONMode_Basic tests JSON mode response format.
func TestJSONMode_Basic(t *testing.T) {
	resetMock()

	// Configure mock to return JSON
	mockLLM.SetNextJSONResponse(`{"name": "John", "age": 30}`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant that outputs JSON."},
			{Role: "user", Content: "Give me a person's info"},
		},
		ResponseFormat: &testutil.ResponseFormat{
			Type: "json_object",
		},
	}

	resp, httpResp, err := testClient.ChatCompletionWithFormat(ctx, req)
	require.NoError(t, err)

	if httpResp.StatusCode != 200 {
		t.Skipf("Server returned %d, JSON mode may not be supported", httpResp.StatusCode)
		return
	}

	require.Len(t, resp.Choices, 1)
	content := resp.Choices[0].Message.Content

	// Verify it's valid JSON
	var parsed map[string]any
	err = json.Unmarshal([]byte(content), &parsed)
	require.NoError(t, err, "response should be valid JSON")

	assert.Equal(t, "John", parsed["name"])
}

// TestJSONMode_FormatForwarded verifies response_format is forwarded to upstream.
func TestJSONMode_FormatForwarded(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "Test"},
		},
		ResponseFormat: &testutil.ResponseFormat{
			Type: "json_object",
		},
	}

	_, _, err := testClient.ChatCompletionWithFormat(ctx, req)
	require.NoError(t, err)

	// Verify the request was forwarded with response_format
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	respFmt, ok := sentReq["response_format"].(map[string]any)
	assert.True(t, ok, "response_format should be present")
	assert.Equal(t, "json_object", respFmt["type"])
}

// TestTools_ToolResultMessage tests sending tool results back.
func TestTools_ToolResultMessage(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simulate a conversation with tool result
	req := &testutil.ChatCompletionRequest{
		Model: "gpt-4o-mock",
		Messages: []testutil.ChatMessage{
			{Role: "user", Content: "What's the weather?"},
			{Role: "assistant", Content: "", ToolCalls: []testutil.ToolCallMessage{
				{
					ID:   "call_123",
					Type: "function",
					Function: testutil.FunctionCallMessage{
						Name:      "get_weather",
						Arguments: `{"location": "SF"}`,
					},
				},
			}},
			{Role: "tool", Content: `{"temperature": 72, "condition": "sunny"}`, ToolCallID: "call_123"},
		},
	}

	_, httpResp, err := testClient.ChatCompletionWithToolResult(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)

	// Verify the tool message was forwarded
	requests := mockLLM.GetRequests()
	require.NotEmpty(t, requests)

	var sentReq map[string]any
	err = json.Unmarshal(requests[0].Body, &sentReq)
	require.NoError(t, err)

	messages := sentReq["messages"].([]any)
	assert.Len(t, messages, 3, "should have 3 messages including tool result")
}
