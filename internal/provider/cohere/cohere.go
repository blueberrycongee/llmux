// Package cohere implements the Cohere provider adapter.
// Cohere provides the Command series of foundation models.
// API Reference: https://docs.cohere.com/reference/chat
package cohere

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "cohere"

	// DefaultBaseURL is the default Cohere API endpoint.
	DefaultBaseURL = "https://api.cohere.ai/v2"
)

// DefaultModels lists available Cohere models.
var DefaultModels = []string{
	"command-r-plus",
	"command-r",
	"command-r-plus-08-2024",
	"command-r-08-2024",
	"command-nightly",
	"command-light",
}

// Provider implements the Cohere API adapter.
type Provider struct {
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
}

// New creates a new Cohere provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Provider{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		models:  cfg.Models,
		client:  &http.Client{},
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
	return strings.HasPrefix(model, "command-")
}

// cohereRequest represents the Cohere Chat API request format.
type cohereRequest struct {
	Model          string          `json:"model"`
	Messages       []cohereMessage `json:"messages"`
	Stream         bool            `json:"stream,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	P              *float64        `json:"p,omitempty"` // top_p
	K              int             `json:"k,omitempty"` // top_k
	StopSequences  []string        `json:"stop_sequences,omitempty"`
	Tools          []cohereTool    `json:"tools,omitempty"`
	ToolChoice     string          `json:"tool_choice,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type cohereMessage struct {
	Role        string             `json:"role"` // user, assistant, system, tool
	Content     string             `json:"content,omitempty"`
	ToolResults []cohereToolResult `json:"tool_results,omitempty"`
	ToolCalls   []cohereToolCall   `json:"tool_calls,omitempty"`
}

type cohereTool struct {
	Type     string            `json:"type"` // function
	Function cohereFunctionDef `json:"function"`
}

type cohereFunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type cohereToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type cohereToolResult struct {
	Call   cohereToolCall `json:"call"`
	Output string         `json:"output"`
}

type responseFormat struct {
	Type string `json:"type"` // text, json_object
}

// cohereResponse represents the Cohere Chat API response format.
type cohereResponse struct {
	ID           string `json:"id"`
	FinishReason string `json:"finish_reason"`
	Message      struct {
		Role      string           `json:"role"`
		Content   []cohereContent  `json:"content,omitempty"`
		ToolCalls []cohereToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Usage struct {
		BilledUnits struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"billed_units"`
		Tokens struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"tokens"`
	} `json:"usage"`
}

type cohereContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// BuildRequest creates an HTTP request for the Cohere API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	cohereReq := p.transformRequest(req)

	body, err := json.Marshal(cohereReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) *cohereRequest {
	cohereReq := &cohereRequest{
		Model:  req.Model,
		Stream: req.Stream,
	}

	if req.MaxTokens > 0 {
		cohereReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature != nil {
		cohereReq.Temperature = req.Temperature
	}

	if req.TopP != nil {
		cohereReq.P = req.TopP
	}

	if len(req.Stop) > 0 {
		cohereReq.StopSequences = req.Stop
	}

	// Transform response format
	if req.ResponseFormat != nil {
		cohereReq.ResponseFormat = &responseFormat{Type: req.ResponseFormat.Type}
	}

	// Transform messages
	cohereReq.Messages = p.transformMessages(req.Messages)

	// Transform tools
	if len(req.Tools) > 0 {
		cohereReq.Tools = p.transformTools(req.Tools)
	}

	// Transform tool_choice
	if len(req.ToolChoice) > 0 {
		var tc string
		if err := json.Unmarshal(req.ToolChoice, &tc); err == nil {
			switch tc {
			case "auto":
				cohereReq.ToolChoice = "auto"
			case "required":
				cohereReq.ToolChoice = "required"
			case "none":
				cohereReq.ToolChoice = "none"
			}
		}
	}

	return cohereReq
}

func (p *Provider) transformMessages(messages []types.ChatMessage) []cohereMessage {
	result := make([]cohereMessage, 0, len(messages))

	for _, msg := range messages {
		var content string
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			// Try as array
			var contentArr []map[string]any
			if err := json.Unmarshal(msg.Content, &contentArr); err == nil {
				for _, c := range contentArr {
					if text, ok := c["text"].(string); ok {
						content += text
					}
				}
			}
		}

		cohereMsg := cohereMessage{
			Role:    mapRole(msg.Role),
			Content: content,
		}

		// Handle tool calls in assistant message
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				cohereMsg.ToolCalls = append(cohereMsg.ToolCalls, cohereToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}

		// Handle tool results
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// Convert to Cohere format - need to find the corresponding call
			cohereMsg.Role = "tool"
			cohereMsg.ToolResults = append(cohereMsg.ToolResults, cohereToolResult{
				Call: cohereToolCall{
					ID:   msg.ToolCallID,
					Type: "function",
				},
				Output: content,
			})
			cohereMsg.Content = ""
		}

		result = append(result, cohereMsg)
	}

	return result
}

func mapRole(role string) string {
	switch role {
	case "system":
		return "system"
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	case "tool":
		return "tool"
	default:
		return role
	}
}

func (p *Provider) transformTools(tools []types.Tool) []cohereTool {
	result := make([]cohereTool, 0, len(tools))
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

		result = append(result, cohereTool{
			Type: "function",
			Function: cohereFunctionDef{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  params,
			},
		})
	}
	return result
}

// ParseResponse transforms a Cohere response into the unified format.
func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var cohereResp cohereResponse
	if err := json.Unmarshal(body, &cohereResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.transformResponse(&cohereResp), nil
}

func (p *Provider) transformResponse(resp *cohereResponse) *types.ChatResponse {
	// Extract text content
	var textContent string
	for _, content := range resp.Message.Content {
		if content.Type == "text" {
			textContent += content.Text
		}
	}

	// Transform tool calls
	toolCalls := make([]types.ToolCall, 0, len(resp.Message.ToolCalls))
	for _, tc := range resp.Message.ToolCalls {
		toolCalls = append(toolCalls, types.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	// Map finish reason
	finishReason := mapCohereFinishReason(resp.FinishReason)

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
		Model:   "",
		Choices: []types.Choice{{
			Index:        0,
			Message:      message,
			FinishReason: finishReason,
		}},
		Usage: &types.Usage{
			PromptTokens:     resp.Usage.Tokens.InputTokens,
			CompletionTokens: resp.Usage.Tokens.OutputTokens,
			TotalTokens:      resp.Usage.Tokens.InputTokens + resp.Usage.Tokens.OutputTokens,
		},
	}
}

func mapCohereFinishReason(reason string) string {
	switch reason {
	case "COMPLETE":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "STOP_SEQUENCE":
		return "stop"
	case "TOOL_CALL":
		return "tool_calls"
	default:
		return reason
	}
}

// ParseStreamChunk parses a single SSE chunk from Cohere.
func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
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
	case "content-delta":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		message, ok := delta["message"].(map[string]any)
		if !ok {
			return nil, nil
		}
		content, ok := message["content"].(map[string]any)
		if !ok {
			return nil, nil
		}
		text, ok := content["text"].(string)
		if !ok {
			return nil, nil
		}

		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{
					Content: text,
				},
			}},
		}, nil

	case "message-start":
		id, _ := event["id"].(string) //nolint:errcheck // zero value is acceptable
		return &types.StreamChunk{
			ID:     id,
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{
					Role: "assistant",
				},
			}},
		}, nil

	case "message-end":
		delta, ok := event["delta"].(map[string]any)
		if !ok {
			return nil, nil
		}
		finishReason, ok := delta["finish_reason"].(string)
		if !ok {
			finishReason = ""
		}
		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index:        0,
				FinishReason: mapCohereFinishReason(finishReason),
			}},
		}, nil
	}

	return nil, nil
}

// MapError converts a Cohere error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}

	message := "unknown error"
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		message = errResp.Message
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
