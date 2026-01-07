package mcp

import (
	"net/http"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// MCP-specific error types.
const (
	TypeMCPConnectionError = "mcp_connection_error"
	TypeMCPToolNotFound    = "mcp_tool_not_found"
	TypeMCPExecutionError  = "mcp_tool_execution_error"
	TypeMCPClientNotFound  = "mcp_client_not_found"
)

// NewMCPConnectionError creates an MCP connection error.
func NewMCPConnectionError(clientName, message string) *llmerrors.LLMError {
	return &llmerrors.LLMError{
		StatusCode: http.StatusServiceUnavailable,
		Message:    message,
		Type:       TypeMCPConnectionError,
		Provider:   "mcp:" + clientName,
		Retryable:  true,
	}
}

// NewMCPToolNotFoundError creates a tool not found error.
func NewMCPToolNotFoundError(toolName string) *llmerrors.LLMError {
	return &llmerrors.LLMError{
		StatusCode: http.StatusNotFound,
		Message:    "tool not found: " + toolName,
		Type:       TypeMCPToolNotFound,
		Provider:   "mcp",
		Retryable:  false,
	}
}

// NewMCPExecutionError creates a tool execution error.
func NewMCPExecutionError(toolName, message string) *llmerrors.LLMError {
	return &llmerrors.LLMError{
		StatusCode: http.StatusInternalServerError,
		Message:    message,
		Type:       TypeMCPExecutionError,
		Provider:   "mcp:" + toolName,
		Retryable:  false,
	}
}

// NewMCPClientNotFoundError creates a client not found error.
func NewMCPClientNotFoundError(clientID string) *llmerrors.LLMError {
	return &llmerrors.LLMError{
		StatusCode: http.StatusNotFound,
		Message:    "MCP client not found: " + clientID,
		Type:       TypeMCPClientNotFound,
		Provider:   "mcp",
		Retryable:  false,
	}
}
