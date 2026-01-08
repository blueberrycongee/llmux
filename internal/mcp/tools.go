package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// convertMCPTool converts an MCP tool definition to OpenAI format.
func convertMCPTool(mcpTool *mcp.Tool) types.Tool {
	// Build parameters JSON
	params := map[string]any{
		"type": "object",
	}

	// Ensure properties is always present (required by OpenAI API)
	if len(mcpTool.InputSchema.Properties) > 0 {
		params["properties"] = mcpTool.InputSchema.Properties
	} else {
		params["properties"] = map[string]any{}
	}

	if len(mcpTool.InputSchema.Required) > 0 {
		params["required"] = mcpTool.InputSchema.Required
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		paramsJSON = []byte(`{"type":"object","properties":{}}`)
	}

	return types.Tool{
		Type: "function",
		Function: types.ToolFunction{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			Parameters:  paramsJSON,
		},
	}
}

// executeToolOnClient executes a tool call on a specific MCP client.
func executeToolOnClient(ctx context.Context, client *Client, toolCall types.ToolCall) (*ToolExecutionResult, error) {
	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Build MCP request
	callReq := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: string(mcp.MethodToolsCall),
		},
		Params: mcp.CallToolParams{
			Name:      toolCall.Function.Name,
			Arguments: args,
		},
	}

	// Execute
	resp, err := client.Conn.CallTool(ctx, callReq)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	// Extract result
	content := extractTextFromResponse(resp, toolCall.Function.Name)

	return &ToolExecutionResult{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Function.Name,
		Content:    content,
		IsError:    resp != nil && resp.IsError,
	}, nil
}

// extractTextFromResponse extracts text content from an MCP tool response.
func extractTextFromResponse(resp *mcp.CallToolResult, toolName string) string {
	if resp == nil {
		return fmt.Sprintf("Tool '%s' executed successfully", toolName)
	}

	var result strings.Builder

	for _, content := range resp.Content {
		switch c := content.(type) {
		case mcp.TextContent:
			result.WriteString(c.Text)
		case mcp.ImageContent:
			result.WriteString(fmt.Sprintf("[Image: %s]", c.MIMEType))
		case mcp.AudioContent:
			result.WriteString(fmt.Sprintf("[Audio: %s]", c.MIMEType))
		case mcp.EmbeddedResource:
			result.WriteString(fmt.Sprintf("[Resource: %s]", c.Type))
		default:
			// Fallback: try JSON serialization
			if data, err := json.Marshal(content); err == nil {
				result.Write(data)
			}
		}
	}

	if result.Len() > 0 {
		return strings.TrimSpace(result.String())
	}

	return fmt.Sprintf("Tool '%s' executed successfully", toolName)
}

// CreateToolMessage creates a tool response message from execution result.
func CreateToolMessage(result ToolExecutionResult) types.ChatMessage {
	contentJSON, err := json.Marshal(result.Content)
	if err != nil {
		contentJSON = []byte(`"error marshaling content"`)
	}

	return types.ChatMessage{
		Role:       "tool",
		Content:    contentJSON,
		ToolCallID: result.ToolCallID,
	}
}

// HasToolCalls checks if a response contains tool calls that need execution.
func HasToolCalls(resp *types.ChatResponse) bool {
	if resp == nil || len(resp.Choices) == 0 {
		return false
	}

	choice := resp.Choices[0]
	return choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0
}

// GetToolCalls extracts tool calls from a response.
func GetToolCalls(resp *types.ChatResponse) []types.ToolCall {
	if !HasToolCalls(resp) {
		return nil
	}
	return resp.Choices[0].Message.ToolCalls
}

// AppendToolResults appends tool execution results to a chat request.
func AppendToolResults(req *types.ChatRequest, assistantMsg types.ChatMessage, results []ToolExecutionResult) {
	// Add the assistant message with tool calls
	req.Messages = append(req.Messages, assistantMsg)

	// Add tool response messages
	for _, result := range results {
		req.Messages = append(req.Messages, CreateToolMessage(result))
	}
}
