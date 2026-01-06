// Package anthropic implements the Anthropic Claude provider adapter.
// It handles request/response transformation between OpenAI format and Anthropic's Messages API.
package anthropic

import (
	"bytes"
	"context"
	"github.com/goccy/go-json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "anthropic"

	// DefaultBaseURL is the default Anthropic API endpoint.
	DefaultBaseURL = "https://api.anthropic.com"

	// DefaultAPIVersion is the default Anthropic API version.
	DefaultAPIVersion = "2023-06-01"

	// DefaultMaxTokens is the default max tokens for Anthropic models.
	DefaultMaxTokens = 4096
)

// Provider implements the Anthropic Claude API adapter.
type Provider struct {
	apiKey     string
	baseURL    string
	apiVersion string
	models     []string
	client     *http.Client
}

// New creates a new Anthropic provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Provider{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		apiVersion: DefaultAPIVersion,
		models:     cfg.Models,
		client:     &http.Client{},
	}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return ProviderName
}

// SupportedModels returns the list of supported models.
func (p *Provider) SupportedModels() []string {
	return p.models
}

// SupportsModel checks if the provider supports the given model.
func (p *Provider) SupportsModel(model string) bool {
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	// Also support models with claude prefix
	return strings.HasPrefix(model, "claude-")
}

// anthropicRequest represents the Anthropic Messages API request format.
type anthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	System        string             `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	TopK          *int               `json:"top_k,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	Metadata      *metadata          `json:"metadata,omitempty"`
	Tools         []anthropicTool    `json:"tools,omitempty"`
	ToolChoice    *toolChoice        `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type metadata struct {
	UserID string `json:"user_id,omitempty"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema inputSchema `json:"input_schema"`
}

type inputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

type toolChoice struct {
	Type                   string `json:"type"` // auto, any, tool, none
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// anthropicResponse represents the Anthropic Messages API response format.
type anthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// BuildRequest creates an HTTP request for the Anthropic API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// Transform OpenAI format to Anthropic format
	anthropicReq, err := p.transformRequest(req)
	if err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set Anthropic-specific headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", p.apiVersion)

	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) (*anthropicRequest, error) {
	anthropicReq := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: DefaultMaxTokens,
		Stream:    req.Stream,
	}

	// Map max_tokens
	if req.MaxTokens > 0 {
		anthropicReq.MaxTokens = req.MaxTokens
	}

	// Map temperature
	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}

	// Map top_p
	if req.TopP != nil {
		anthropicReq.TopP = req.TopP
	}

	// Map stop sequences
	if len(req.Stop) > 0 {
		anthropicReq.StopSequences = req.Stop
	}

	// Map user to metadata
	if req.User != "" {
		anthropicReq.Metadata = &metadata{UserID: req.User}
	}

	// Transform messages - extract system message and convert others
	messages, systemPrompt, err := p.transformMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	anthropicReq.Messages = messages
	if systemPrompt != "" {
		anthropicReq.System = systemPrompt
	}

	// Transform tools
	if len(req.Tools) > 0 {
		anthropicReq.Tools = p.transformTools(req.Tools)
	}

	// Transform tool_choice
	if len(req.ToolChoice) > 0 {
		tc, err := p.transformToolChoice(req.ToolChoice)
		if err == nil && tc != nil {
			anthropicReq.ToolChoice = tc
		}
	}

	return anthropicReq, nil
}

func (p *Provider) transformMessages(messages []types.ChatMessage) ([]anthropicMessage, string, error) {
	var result []anthropicMessage
	var systemPrompt string

	for _, msg := range messages {
		role := msg.Role

		// Extract system message
		if role == "system" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				// Try as array
				var contentArr []map[string]any
				if err := json.Unmarshal(msg.Content, &contentArr); err == nil {
					for _, c := range contentArr {
						if text, ok := c["text"].(string); ok {
							systemPrompt += text
						}
					}
				}
			} else {
				systemPrompt = content
			}
			continue
		}

		// Map assistant role
		if role == "assistant" {
			// Check if it has tool_calls
			if len(msg.ToolCalls) > 0 {
				var blocks []contentBlock
				for _, tc := range msg.ToolCalls {
					var input any
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
						input = tc.Function.Arguments // Use raw string if unmarshal fails
					}
					blocks = append(blocks, contentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: input,
					})
				}
				result = append(result, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
				continue
			}
		}

		// Map tool role to user with tool_result
		if role == "tool" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				content = string(msg.Content) // Use raw content if unmarshal fails
			}
			result = append(result, anthropicMessage{
				Role: "user",
				Content: []contentBlock{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   content,
				}},
			})
			continue
		}

		// Regular message - try to parse content
		var content string
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			// Content might be an array of content blocks
			var contentArr []map[string]any
			if err := json.Unmarshal(msg.Content, &contentArr); err != nil {
				return nil, "", fmt.Errorf("invalid message content format")
			}
			// Convert to Anthropic content blocks
			var blocks []contentBlock
			for _, c := range contentArr {
				if c["type"] == "text" {
					blocks = append(blocks, contentBlock{
						Type: "text",
						Text: c["text"].(string),
					})
				}
				// TODO: Handle image content blocks
			}
			result = append(result, anthropicMessage{
				Role:    role,
				Content: blocks,
			})
		} else {
			result = append(result, anthropicMessage{
				Role:    role,
				Content: content,
			})
		}
	}

	return result, systemPrompt, nil
}

func (p *Provider) transformTools(tools []types.Tool) []anthropicTool {
	result := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}

		var params map[string]any
		if len(tool.Function.Parameters) > 0 {
			if err := json.Unmarshal(tool.Function.Parameters, &params); err != nil {
				params = make(map[string]any) // Use empty map if unmarshal fails
			}
		}

		schema := inputSchema{
			Type:       "object",
			Properties: make(map[string]any),
		}
		if props, ok := params["properties"].(map[string]any); ok {
			schema.Properties = props
		}
		if required, ok := params["required"].([]any); ok {
			for _, r := range required {
				if s, ok := r.(string); ok {
					schema.Required = append(schema.Required, s)
				}
			}
		}

		result = append(result, anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: schema,
		})
	}
	return result
}

func (p *Provider) transformToolChoice(raw json.RawMessage) (*toolChoice, error) {
	// Try as string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		switch str {
		case "auto":
			return &toolChoice{Type: "auto"}, nil
		case "required":
			return &toolChoice{Type: "any"}, nil
		case "none":
			return &toolChoice{Type: "none"}, nil
		}
		return nil, nil
	}

	// Try as object
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	if fn, ok := obj["function"].(map[string]any); ok {
		if name, ok := fn["name"].(string); ok {
			return &toolChoice{Type: "tool", Name: name}, nil
		}
	}

	return nil, nil
}

// ParseResponse transforms an Anthropic response into the unified format.
func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.transformResponse(&anthropicResp), nil
}

func (p *Provider) transformResponse(resp *anthropicResponse) *types.ChatResponse {
	// Build message content and tool calls
	var textContent string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	// Map stop_reason to finish_reason
	finishReason := mapStopReason(resp.StopReason)

	message := types.ChatMessage{
		Role:    "assistant",
		Content: json.RawMessage(fmt.Sprintf("%q", textContent)),
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return &types.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: 0, // Anthropic doesn't return created timestamp
		Model:   resp.Model,
		Choices: []types.Choice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: &types.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return reason
	}
}

// ParseStreamChunk parses a single SSE chunk from Anthropic.
func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// Skip empty lines
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Handle SSE format: "event: xxx" and "data: xxx"
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil, nil // Skip event lines
	}

	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
	}

	// Check for stream end
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}

	// Parse the JSON event
	var event map[string]any
	if err := json.Unmarshal(trimmed, &event); err != nil {
		return nil, nil // Skip unparseable events
	}

	eventType, _ := event["type"].(string)

	switch eventType {
	case "content_block_delta":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		if delta["type"] == "text_delta" {
			text, _ := delta["text"].(string)
			return &types.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []types.StreamChoice{{
					Index: 0,
					Delta: types.StreamDelta{
						Content: text,
					},
				}},
			}, nil
		}

	case "message_start":
		msg, ok := event["message"].(map[string]any)
		if !ok {
			return nil, nil
		}
		id, _ := msg["id"].(string)
		model, _ := msg["model"].(string)
		return &types.StreamChunk{
			ID:     id,
			Object: "chat.completion.chunk",
			Model:  model,
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{
					Role: "assistant",
				},
			}},
		}, nil

	case "message_delta":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		stopReason, _ := delta["stop_reason"].(string)
		if stopReason != "" {
			return &types.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []types.StreamChoice{{
					Index:        0,
					FinishReason: mapStopReason(stopReason),
				}},
			}, nil
		}

	case "message_stop":
		return nil, nil
	}

	return nil, nil
}

// MapError converts an Anthropic error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	// Try to parse Anthropic error format
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	message := "unknown error"
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return llmerrors.NewAuthenticationError(ProviderName, "", message)
	case http.StatusTooManyRequests:
		return llmerrors.NewRateLimitError(ProviderName, "", message)
	case http.StatusBadRequest:
		return llmerrors.NewInvalidRequestError(ProviderName, "", message)
	case http.StatusNotFound:
		return llmerrors.NewNotFoundError(ProviderName, "", message)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return llmerrors.NewTimeoutError(ProviderName, "", message)
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return llmerrors.NewServiceUnavailableError(ProviderName, "", message)
	default:
		return llmerrors.NewInternalError(ProviderName, "", message)
	}
}
