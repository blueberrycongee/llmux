// Package bedrock implements the AWS Bedrock provider adapter.
// AWS Bedrock provides access to various foundation models including Claude, Llama, and Titan.
// API Reference: https://docs.aws.amazon.com/bedrock/latest/APIReference/
package bedrock

import (
	"bytes"
	"context"
	"encoding/json"
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
	ProviderName = "bedrock"

	// DefaultRegion is the default AWS region.
	DefaultRegion = "us-east-1"
)

// DefaultModels lists commonly available Bedrock models.
var DefaultModels = []string{
	"anthropic.claude-3-5-sonnet-20241022-v2:0",
	"anthropic.claude-3-5-sonnet-20240620-v1:0",
	"anthropic.claude-3-opus-20240229-v1:0",
	"anthropic.claude-3-sonnet-20240229-v1:0",
	"anthropic.claude-3-haiku-20240307-v1:0",
	"meta.llama3-1-70b-instruct-v1:0",
	"meta.llama3-1-8b-instruct-v1:0",
	"amazon.titan-text-express-v1",
	"mistral.mistral-large-2402-v1:0",
}

// Provider implements the AWS Bedrock API adapter.
type Provider struct {
	accessKey    string
	secretKey    string
	region       string
	baseURL      string
	models       []string
	client       *http.Client
}

// New creates a new AWS Bedrock provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	region := DefaultRegion
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Parse API key as "access_key:secret_key"
	parts := strings.SplitN(cfg.APIKey, ":", 2)
	accessKey := ""
	secretKey := ""
	if len(parts) == 2 {
		accessKey = parts[0]
		secretKey = parts[1]
	}

	return &Provider{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		baseURL:   baseURL,
		models:    cfg.Models,
		client:    &http.Client{},
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
	// Check common prefixes
	prefixes := []string{"anthropic.", "meta.", "amazon.", "mistral.", "cohere.", "ai21."}
	for _, prefix := range prefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// bedrockRequest represents the Bedrock Converse API request format.
type bedrockRequest struct {
	ModelID             string                 `json:"-"` // Used in URL, not body
	Messages            []bedrockMessage       `json:"messages"`
	System              []bedrockSystemContent `json:"system,omitempty"`
	InferenceConfig     *inferenceConfig       `json:"inferenceConfig,omitempty"`
	ToolConfig          *bedrockToolConfig     `json:"toolConfig,omitempty"`
}

type bedrockMessage struct {
	Role    string                `json:"role"`
	Content []bedrockContentBlock `json:"content"`
}

type bedrockSystemContent struct {
	Text string `json:"text"`
}

type bedrockContentBlock struct {
	Text       string             `json:"text,omitempty"`
	ToolUse    *bedrockToolUse    `json:"toolUse,omitempty"`
	ToolResult *bedrockToolResult `json:"toolResult,omitempty"`
}

type bedrockToolUse struct {
	ToolUseID string         `json:"toolUseId"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
}

type bedrockToolResult struct {
	ToolUseID string                   `json:"toolUseId"`
	Content   []bedrockToolResultContent `json:"content"`
}

type bedrockToolResultContent struct {
	Text string `json:"text,omitempty"`
}

type inferenceConfig struct {
	MaxTokens     int      `json:"maxTokens,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	TopP          *float64 `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

type bedrockToolConfig struct {
	Tools      []bedrockTool   `json:"tools,omitempty"`
	ToolChoice *bedrockToolChoice `json:"toolChoice,omitempty"`
}

type bedrockTool struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type bedrockToolChoice struct {
	Auto *struct{} `json:"auto,omitempty"`
	Any  *struct{} `json:"any,omitempty"`
	Tool *struct {
		Name string `json:"name"`
	} `json:"tool,omitempty"`
}

// bedrockResponse represents the Bedrock Converse API response format.
type bedrockResponse struct {
	Output struct {
		Message bedrockMessage `json:"message"`
	} `json:"output"`
	StopReason string         `json:"stopReason"`
	Usage      bedrockUsage   `json:"usage"`
}

type bedrockUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// BuildRequest creates an HTTP request for the Bedrock API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	bedrockReq, err := p.transformRequest(req)
	if err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}

	body, err := json.Marshal(bedrockReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL with model ID
	url := fmt.Sprintf("%s/model/%s/converse", p.baseURL, req.Model)
	if req.Stream {
		url = fmt.Sprintf("%s/model/%s/converse-stream", p.baseURL, req.Model)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Note: Real implementation would use AWS SigV4 signing
	// For now, we'll use a simplified auth header
	httpReq.Header.Set("X-Access-Key", p.accessKey)

	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) (*bedrockRequest, error) {
	bedrockReq := &bedrockRequest{
		ModelID: req.Model,
		InferenceConfig: &inferenceConfig{},
	}

	if req.MaxTokens > 0 {
		bedrockReq.InferenceConfig.MaxTokens = req.MaxTokens
	}
	if req.Temperature != nil {
		bedrockReq.InferenceConfig.Temperature = req.Temperature
	}
	if req.TopP != nil {
		bedrockReq.InferenceConfig.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		bedrockReq.InferenceConfig.StopSequences = req.Stop
	}

	// Transform messages
	messages, system, err := p.transformMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	bedrockReq.Messages = messages
	if len(system) > 0 {
		bedrockReq.System = system
	}

	// Transform tools
	if len(req.Tools) > 0 {
		bedrockReq.ToolConfig = p.transformTools(req.Tools, req.ToolChoice)
	}

	return bedrockReq, nil
}

func (p *Provider) transformMessages(messages []types.ChatMessage) ([]bedrockMessage, []bedrockSystemContent, error) {
	var result []bedrockMessage
	var system []bedrockSystemContent

	for _, msg := range messages {
		// Handle system message
		if msg.Role == "system" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err == nil {
				system = append(system, bedrockSystemContent{Text: content})
			}
			continue
		}

		bedrockMsg := bedrockMessage{Role: msg.Role}

		// Handle tool calls in assistant message
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				var input map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = make(map[string]any)
				}
				bedrockMsg.Content = append(bedrockMsg.Content, bedrockContentBlock{
					ToolUse: &bedrockToolUse{
						ToolUseID: tc.ID,
						Name:      tc.Function.Name,
						Input:     input,
					},
				})
			}
			result = append(result, bedrockMsg)
			continue
		}

		// Handle tool result
		if msg.Role == "tool" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				content = string(msg.Content)
			}
			bedrockMsg.Role = "user"
			bedrockMsg.Content = append(bedrockMsg.Content, bedrockContentBlock{
				ToolResult: &bedrockToolResult{
					ToolUseID: msg.ToolCallID,
					Content:   []bedrockToolResultContent{{Text: content}},
				},
			})
			result = append(result, bedrockMsg)
			continue
		}

		// Regular text message
		var content string
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			// Try as array
			var contentArr []map[string]any
			if err := json.Unmarshal(msg.Content, &contentArr); err == nil {
				for _, c := range contentArr {
					if text, ok := c["text"].(string); ok {
						bedrockMsg.Content = append(bedrockMsg.Content, bedrockContentBlock{Text: text})
					}
				}
			}
		} else {
			bedrockMsg.Content = append(bedrockMsg.Content, bedrockContentBlock{Text: content})
		}

		result = append(result, bedrockMsg)
	}

	return result, system, nil
}

func (p *Provider) transformTools(tools []types.Tool, toolChoice json.RawMessage) *bedrockToolConfig {
	config := &bedrockToolConfig{}

	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}

		var params map[string]any
		if len(tool.Function.Parameters) > 0 {
			json.Unmarshal(tool.Function.Parameters, &params)
		}

		config.Tools = append(config.Tools, bedrockTool{
			ToolSpec: bedrockToolSpec{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: map[string]any{"json": params},
			},
		})
	}

	// Handle tool choice
	if len(toolChoice) > 0 {
		var tc string
		if err := json.Unmarshal(toolChoice, &tc); err == nil {
			switch tc {
			case "auto":
				config.ToolChoice = &bedrockToolChoice{Auto: &struct{}{}}
			case "required":
				config.ToolChoice = &bedrockToolChoice{Any: &struct{}{}}
			}
		}
	}

	return config
}

// ParseResponse transforms a Bedrock response into the unified format.
func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var bedrockResp bedrockResponse
	if err := json.Unmarshal(body, &bedrockResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.transformResponse(&bedrockResp), nil
}

func (p *Provider) transformResponse(resp *bedrockResponse) *types.ChatResponse {
	var textContent string
	var toolCalls []types.ToolCall

	for _, block := range resp.Output.Message.Content {
		if block.Text != "" {
			textContent += block.Text
		}
		if block.ToolUse != nil {
			inputJSON, _ := json.Marshal(block.ToolUse.Input)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   block.ToolUse.ToolUseID,
				Type: "function",
				Function: types.ToolCallFunction{
					Name:      block.ToolUse.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	message := types.ChatMessage{
		Role:    "assistant",
		Content: json.RawMessage(fmt.Sprintf("%q", textContent)),
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return &types.ChatResponse{
		Object: "chat.completion",
		Choices: []types.Choice{{
			Index:        0,
			Message:      message,
			FinishReason: mapBedrockStopReason(resp.StopReason),
		}},
		Usage: &types.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func mapBedrockStopReason(reason string) string {
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

// ParseStreamChunk parses a single event from Bedrock streaming response.
func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Bedrock uses a specific event stream format
	var event map[string]any
	if err := json.Unmarshal(trimmed, &event); err != nil {
		return nil, nil
	}

	// Handle content block delta
	if delta, ok := event["contentBlockDelta"].(map[string]any); ok {
		if d, ok := delta["delta"].(map[string]any); ok {
			if text, ok := d["text"].(string); ok {
				return &types.StreamChunk{
					Object: "chat.completion.chunk",
					Choices: []types.StreamChoice{{
						Index: 0,
						Delta: types.StreamDelta{Content: text},
					}},
				}, nil
			}
		}
	}

	// Handle message stop
	if stop, ok := event["messageStop"].(map[string]any); ok {
		stopReason, _ := stop["stopReason"].(string)
		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index:        0,
				FinishReason: mapBedrockStopReason(stopReason),
			}},
		}, nil
	}

	return nil, nil
}

// MapError converts a Bedrock error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}

	message := "unknown error"
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		message = errResp.Message
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
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
