// Package anthropic provides the Anthropic Claude provider for LLMux library mode.
// It handles request/response transformation between OpenAI format and Anthropic's Messages API.
package anthropic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
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

// DefaultModels lists the available Anthropic models.
var DefaultModels = []string{
	"claude-3-5-sonnet-20241022",
	"claude-3-5-haiku-20241022",
	"claude-3-opus-20240229",
	"claude-3-sonnet-20240229",
	"claude-3-haiku-20240307",
}

// Provider implements the Anthropic Claude API adapter.
type Provider struct {
	apiKey     string
	baseURL    string
	apiVersion string
	models     []string
	headers    map[string]string
}

// New creates a new Anthropic provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		headers:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// NewFromConfig creates a provider from a Config struct.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	p := New(
		WithAPIKey(cfg.APIKey),
		WithBaseURL(cfg.BaseURL),
		WithModels(cfg.Models...),
	)
	for k, v := range cfg.Headers {
		p.headers[k] = v
	}
	return p, nil
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
	Content any    `json:"content"`
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
	Type                   string `json:"type"`
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
	anthropicReq, err := p.transformRequest(req)
	if err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(p.baseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", p.apiVersion)

	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) (*anthropicRequest, error) {
	anthropicReq := &anthropicRequest{
		Model:     req.Model,
		MaxTokens: DefaultMaxTokens,
		Stream:    req.Stream,
	}

	if req.MaxTokens > 0 {
		anthropicReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		anthropicReq.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		anthropicReq.StopSequences = req.Stop
	}
	if req.User != "" {
		anthropicReq.Metadata = &metadata{UserID: req.User}
	}

	messages, systemPrompt, err := p.transformMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	anthropicReq.Messages = messages
	if systemPrompt != "" {
		anthropicReq.System = systemPrompt
	}

	if len(req.Tools) > 0 {
		anthropicReq.Tools = p.transformTools(req.Tools)
	}

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

		if role == "system" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
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

		if role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []contentBlock
			for _, tc := range msg.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = tc.Function.Arguments
				}
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			result = append(result, anthropicMessage{Role: "assistant", Content: blocks})
			continue
		}

		if role == "tool" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				content = string(msg.Content)
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

		var content string
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			var contentArr []map[string]any
			if err := json.Unmarshal(msg.Content, &contentArr); err != nil {
				return nil, "", fmt.Errorf("invalid message content format")
			}
			var blocks []contentBlock
			for _, c := range contentArr {
				if c["type"] == "text" {
					if text, ok := c["text"].(string); ok {
						blocks = append(blocks, contentBlock{Type: "text", Text: text})
					}
				}
			}
			result = append(result, anthropicMessage{Role: role, Content: blocks})
		} else {
			result = append(result, anthropicMessage{Role: role, Content: content})
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
				params = make(map[string]any)
			}
		}

		schema := inputSchema{Type: "object", Properties: make(map[string]any)}
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
	var textContent string
	var toolCalls []types.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			inputJSON, err := json.Marshal(block.Input)
			if err != nil {
				inputJSON = []byte("{}")
			}
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
		Created: 0,
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
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil, nil
	}

	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
	}

	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}

	var event map[string]any
	if err := json.Unmarshal(trimmed, &event); err != nil {
		return nil, nil
	}

	eventType, ok := event["type"].(string)
	if !ok {
		return nil, nil
	}

	switch eventType {
	case "content_block_delta":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		if delta["type"] == "text_delta" {
			text, ok := delta["text"].(string)
			if !ok {
				return nil, nil
			}
			return &types.StreamChunk{
				Object: "chat.completion.chunk",
				Choices: []types.StreamChoice{{
					Index: 0,
					Delta: types.StreamDelta{Content: text},
				}},
			}, nil
		}

	case "message_start":
		msg, ok := event["message"].(map[string]any)
		if !ok {
			return nil, nil
		}
		var id, model string
		if v, ok := msg["id"].(string); ok {
			id = v
		}
		if v, ok := msg["model"].(string); ok {
			model = v
		}
		return &types.StreamChunk{
			ID:     id,
			Object: "chat.completion.chunk",
			Model:  model,
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{Role: "assistant"},
			}},
		}, nil

	case "message_delta":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		stopReason, ok := delta["stop_reason"].(string)
		if ok && stopReason != "" {
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
		return errors.NewAuthenticationError(ProviderName, "", message)
	case http.StatusTooManyRequests:
		return errors.NewRateLimitError(ProviderName, "", message)
	case http.StatusBadRequest:
		return errors.NewInvalidRequestError(ProviderName, "", message)
	case http.StatusNotFound:
		return errors.NewNotFoundError(ProviderName, "", message)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return errors.NewTimeoutError(ProviderName, "", message)
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return errors.NewServiceUnavailableError(ProviderName, "", message)
	default:
		return errors.NewInternalError(ProviderName, "", message)
	}
}
