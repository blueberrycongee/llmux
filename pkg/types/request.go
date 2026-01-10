// Package types defines core data structures for LLM API requests and responses.
// All types are designed to be compatible with OpenAI's Chat Completion API format.
package types //nolint:revive // package name is intentional

import "github.com/goccy/go-json"

// ChatRequest represents an OpenAI-compatible chat completion request.
// It serves as the unified input format for all LLM providers.
type ChatRequest struct {
	Model            string          `json:"model"`
	Messages         []ChatMessage   `json:"messages"`
	Stream           bool            `json:"stream,omitempty"`
	MaxTokens        int             `json:"max_tokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	N                int             `json:"n,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	User             string          `json:"user,omitempty"`
	Tools            []Tool          `json:"tools,omitempty"`
	ToolChoice       json.RawMessage `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	StreamOptions    *StreamOptions  `json:"stream_options,omitempty"`
	// Tags are request-level tags for routing decisions.
	Tags []string `json:"tags,omitempty"`

	// Extra holds provider-specific parameters that are passed through unchanged.
	// This enables zero-copy forwarding of unknown fields.
	Extra map[string]json.RawMessage `json:"-"`
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Name       string          `json:"name,omitempty"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

// Tool represents a function that the model can call.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall represents a function call made by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction contains the function name and arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseFormat specifies the output format for the model.
type ResponseFormat struct {
	Type string `json:"type"`
}

// Reset clears the ChatRequest for reuse.
func (r *ChatRequest) Reset() {
	r.Model = ""
	r.Messages = r.Messages[:0] // Keep capacity
	r.Stream = false
	r.MaxTokens = 0
	r.Temperature = nil
	r.TopP = nil
	r.N = 0
	r.Stop = r.Stop[:0]
	r.PresencePenalty = nil
	r.FrequencyPenalty = nil
	r.User = ""
	r.Tools = r.Tools[:0]
	r.ToolChoice = nil
	r.ResponseFormat = nil
	r.Tags = nil
	// Clear map but keep it if possible, or just nil it.
	// For simplicity and safety, nil it.
	r.Extra = nil
}
