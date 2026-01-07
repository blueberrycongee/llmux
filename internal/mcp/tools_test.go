package mcp

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestConvertMCPTool(t *testing.T) {
	t.Run("basic tool conversion", func(t *testing.T) {
		mcpTool := &mcp.Tool{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"param1": map[string]any{
						"type":        "string",
						"description": "First parameter",
					},
					"param2": map[string]any{
						"type":        "integer",
						"description": "Second parameter",
					},
				},
				Required: []string{"param1"},
			},
		}

		tool := convertMCPTool(mcpTool)

		if tool.Type != "function" {
			t.Errorf("Type = %q, want %q", tool.Type, "function")
		}
		if tool.Function.Name != "test_tool" {
			t.Errorf("Name = %q, want %q", tool.Function.Name, "test_tool")
		}
		if tool.Function.Description != "A test tool" {
			t.Errorf("Description = %q, want %q", tool.Function.Description, "A test tool")
		}

		// Verify parameters JSON
		var params map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &params); err != nil {
			t.Fatalf("Failed to unmarshal parameters: %v", err)
		}

		if params["type"] != "object" {
			t.Errorf("params.type = %v, want %q", params["type"], "object")
		}

		props, ok := params["properties"].(map[string]any)
		if !ok {
			t.Fatal("params.properties is not a map")
		}
		if len(props) != 2 {
			t.Errorf("len(properties) = %d, want %d", len(props), 2)
		}

		required, ok := params["required"].([]any)
		if !ok {
			t.Fatal("params.required is not a slice")
		}
		if len(required) != 1 || required[0] != "param1" {
			t.Errorf("required = %v, want [param1]", required)
		}
	})

	t.Run("tool with empty properties", func(t *testing.T) {
		mcpTool := &mcp.Tool{
			Name:        "empty_tool",
			Description: "Tool with no parameters",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: nil,
			},
		}

		tool := convertMCPTool(mcpTool)

		var params map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &params); err != nil {
			t.Fatalf("Failed to unmarshal parameters: %v", err)
		}

		// Should have empty properties object
		props, ok := params["properties"].(map[string]any)
		if !ok {
			t.Fatal("params.properties is not a map")
		}
		if len(props) != 0 {
			t.Errorf("len(properties) = %d, want %d", len(props), 0)
		}
	})
}

func TestHasToolCalls(t *testing.T) {
	tests := []struct {
		name string
		resp *types.ChatResponse
		want bool
	}{
		{
			name: "nil response",
			resp: nil,
			want: false,
		},
		{
			name: "empty choices",
			resp: &types.ChatResponse{Choices: []types.Choice{}},
			want: false,
		},
		{
			name: "no tool calls",
			resp: &types.ChatResponse{
				Choices: []types.Choice{
					{
						FinishReason: "stop",
						Message:      types.ChatMessage{Role: "assistant"},
					},
				},
			},
			want: false,
		},
		{
			name: "has tool calls",
			resp: &types.ChatResponse{
				Choices: []types.Choice{
					{
						FinishReason: "tool_calls",
						Message: types.ChatMessage{
							Role: "assistant",
							ToolCalls: []types.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: types.ToolCallFunction{
										Name:      "test_tool",
										Arguments: `{"param": "value"}`,
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "tool_calls finish reason but empty tool calls",
			resp: &types.ChatResponse{
				Choices: []types.Choice{
					{
						FinishReason: "tool_calls",
						Message: types.ChatMessage{
							Role:      "assistant",
							ToolCalls: []types.ToolCall{},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasToolCalls(tt.resp)
			if got != tt.want {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetToolCalls(t *testing.T) {
	t.Run("returns nil for no tool calls", func(t *testing.T) {
		resp := &types.ChatResponse{
			Choices: []types.Choice{
				{FinishReason: "stop"},
			},
		}

		got := GetToolCalls(resp)
		if got != nil {
			t.Errorf("GetToolCalls() = %v, want nil", got)
		}
	})

	t.Run("returns tool calls", func(t *testing.T) {
		toolCalls := []types.ToolCall{
			{ID: "call_1", Type: "function"},
			{ID: "call_2", Type: "function"},
		}

		resp := &types.ChatResponse{
			Choices: []types.Choice{
				{
					FinishReason: "tool_calls",
					Message: types.ChatMessage{
						ToolCalls: toolCalls,
					},
				},
			},
		}

		got := GetToolCalls(resp)
		if len(got) != 2 {
			t.Errorf("len(GetToolCalls()) = %d, want %d", len(got), 2)
		}
	})
}

func TestCreateToolMessage(t *testing.T) {
	result := ToolExecutionResult{
		ToolCallID: "call_123",
		ToolName:   "test_tool",
		Content:    "Tool output",
	}

	msg := CreateToolMessage(result)

	if msg.Role != "tool" {
		t.Errorf("Role = %q, want %q", msg.Role, "tool")
	}
	if msg.ToolCallID != "call_123" {
		t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "call_123")
	}

	// Content should be JSON-encoded string
	var content string
	if err := json.Unmarshal(msg.Content, &content); err != nil {
		t.Fatalf("Failed to unmarshal content: %v", err)
	}
	if content != "Tool output" {
		t.Errorf("Content = %q, want %q", content, "Tool output")
	}
}

func TestExtractTextFromResponse(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		got := extractTextFromResponse(nil, "test_tool")
		if got != "Tool 'test_tool' executed successfully" {
			t.Errorf("got %q, want success message", got)
		}
	})

	t.Run("text content", func(t *testing.T) {
		resp := &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Hello, world!",
				},
			},
		}

		got := extractTextFromResponse(resp, "test_tool")
		if got != "Hello, world!" {
			t.Errorf("got %q, want %q", got, "Hello, world!")
		}
	})

	t.Run("empty content", func(t *testing.T) {
		resp := &mcp.CallToolResult{
			Content: []mcp.Content{},
		}

		got := extractTextFromResponse(resp, "test_tool")
		if got != "Tool 'test_tool' executed successfully" {
			t.Errorf("got %q, want success message", got)
		}
	})
}

func TestAppendToolResults(t *testing.T) {
	req := &types.ChatRequest{
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	assistantMsg := types.ChatMessage{
		Role: "assistant",
		ToolCalls: []types.ToolCall{
			{ID: "call_1"},
			{ID: "call_2"},
		},
	}

	results := []ToolExecutionResult{
		{ToolCallID: "call_1", Content: "Result 1"},
		{ToolCallID: "call_2", Content: "Result 2"},
	}

	AppendToolResults(req, assistantMsg, results)

	// Should have: user + assistant + 2 tool messages = 4
	if len(req.Messages) != 4 {
		t.Errorf("len(Messages) = %d, want %d", len(req.Messages), 4)
	}

	// Check assistant message
	if req.Messages[1].Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want %q", req.Messages[1].Role, "assistant")
	}

	// Check tool messages
	if req.Messages[2].Role != "tool" {
		t.Errorf("Messages[2].Role = %q, want %q", req.Messages[2].Role, "tool")
	}
	if req.Messages[2].ToolCallID != "call_1" {
		t.Errorf("Messages[2].ToolCallID = %q, want %q", req.Messages[2].ToolCallID, "call_1")
	}
}
