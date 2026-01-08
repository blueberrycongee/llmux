// Package gemini implements the Google Gemini provider adapter.
// It handles request/response transformation between OpenAI format and Gemini's generateContent API.
package gemini

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
	ProviderName = "gemini"

	// DefaultBaseURL is the default Google AI Studio API endpoint.
	DefaultBaseURL = "https://generativelanguage.googleapis.com"

	// DefaultAPIVersion is the default Gemini API version.
	DefaultAPIVersion = "v1beta"
)

// Provider implements the Google Gemini API adapter.
type Provider struct {
	apiKey     string
	baseURL    string
	apiVersion string
	models     []string
	client     *http.Client
}

// New creates a new Gemini provider instance.
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
	// Also support models with gemini prefix
	return strings.HasPrefix(model, "gemini-")
}

// geminiRequest represents the Gemini generateContent API request format.
type geminiRequest struct {
	Contents          []geminiContent   `json:"contents"`
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
	Tools             []geminiTool      `json:"tools,omitempty"`
	ToolConfig        *toolConfig       `json:"toolConfig,omitempty"`
	SafetySettings    []safetySetting   `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string            `json:"text,omitempty"`
	InlineData   *inlineData       `json:"inlineData,omitempty"`
	FunctionCall *functionCall     `json:"functionCall,omitempty"`
	FunctionResp *functionResponse `json:"functionResponse,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type functionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type functionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type generationConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
	MaxOutputTokens  int      `json:"maxOutputTokens,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	CandidateCount   int      `json:"candidateCount,omitempty"`
	ResponseMimeType string   `json:"responseMimeType,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitempty"`
}

type functionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type toolConfig struct {
	FunctionCallingConfig *functionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type functionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // AUTO, ANY, NONE
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// geminiResponse represents the Gemini generateContent API response format.
type geminiResponse struct {
	Candidates     []candidate     `json:"candidates"`
	UsageMetadata  *usageMetadata  `json:"usageMetadata,omitempty"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
}

type candidate struct {
	Content       geminiContent  `json:"content"`
	FinishReason  string         `json:"finishReason"`
	Index         int            `json:"index"`
	SafetyRatings []safetyRating `json:"safetyRatings,omitempty"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type promptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []safetyRating `json:"safetyRatings,omitempty"`
}

type safetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// BuildRequest creates an HTTP request for the Gemini API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// Transform OpenAI format to Gemini format
	geminiReq, err := p.transformRequest(req)
	if err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL with model name
	model := req.Model
	action := "generateContent"
	if req.Stream {
		action = "streamGenerateContent"
	}
	url := fmt.Sprintf("%s/%s/models/%s:%s?key=%s",
		p.baseURL, p.apiVersion, model, action, p.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) (*geminiRequest, error) {
	geminiReq := &geminiRequest{
		GenerationConfig: &generationConfig{},
	}

	// Map generation config
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}
	if req.Temperature != nil {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}
	if req.TopP != nil {
		geminiReq.GenerationConfig.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		geminiReq.GenerationConfig.StopSequences = req.Stop
	}

	// Transform messages
	contents, systemInstruction, err := p.transformMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	geminiReq.Contents = contents
	if systemInstruction != nil {
		geminiReq.SystemInstruction = systemInstruction
	}

	// Transform tools
	if len(req.Tools) > 0 {
		geminiReq.Tools = p.transformTools(req.Tools)
	}

	// Transform tool_choice
	if len(req.ToolChoice) > 0 {
		tc, err := p.transformToolChoice(req.ToolChoice)
		if err == nil && tc != nil {
			geminiReq.ToolConfig = tc
		}
	}

	return geminiReq, nil
}

func (p *Provider) transformMessages(messages []types.ChatMessage) ([]geminiContent, *geminiContent, error) {
	var contents []geminiContent
	var systemInstruction *geminiContent

	for _, msg := range messages {
		role := msg.Role

		// Extract system message
		if role == "system" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err == nil {
				systemInstruction = &geminiContent{
					Parts: []geminiPart{{Text: content}},
				}
			}
			continue
		}

		// Map roles: assistant -> model, user -> user
		geminiRole := role
		if role == "assistant" {
			geminiRole = "model"
		}

		// Check for tool calls in assistant message
		if role == "assistant" && len(msg.ToolCalls) > 0 {
			var parts []geminiPart
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = make(map[string]any) // Use empty map if unmarshal fails
				}
				parts = append(parts, geminiPart{
					FunctionCall: &functionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
			contents = append(contents, geminiContent{
				Role:  geminiRole,
				Parts: parts,
			})
			continue
		}

		// Handle tool response
		if role == "tool" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				content = string(msg.Content) // Use raw content if unmarshal fails
			}
			// Find the function name from tool_call_id (simplified - in real impl would need to track)
			contents = append(contents, geminiContent{
				Role: "function",
				Parts: []geminiPart{{
					FunctionResp: &functionResponse{
						Name:     msg.ToolCallID, // Simplified
						Response: map[string]any{"result": content},
					},
				}},
			})
			continue
		}

		// Regular message
		var content string
		if err := json.Unmarshal(msg.Content, &content); err != nil {
			// Try as array
			var contentArr []map[string]any
			if err := json.Unmarshal(msg.Content, &contentArr); err != nil {
				return nil, nil, fmt.Errorf("invalid message content format")
			}
			var parts []geminiPart
			for _, c := range contentArr {
				if c["type"] == "text" {
					if text, ok := c["text"].(string); ok {
						parts = append(parts, geminiPart{Text: text})
					}
				}
				// TODO: Handle image content
			}
			contents = append(contents, geminiContent{
				Role:  geminiRole,
				Parts: parts,
			})
		} else {
			contents = append(contents, geminiContent{
				Role:  geminiRole,
				Parts: []geminiPart{{Text: content}},
			})
		}
	}

	return contents, systemInstruction, nil
}

func (p *Provider) transformTools(tools []types.Tool) []geminiTool {
	declarations := make([]functionDeclaration, 0, len(tools))
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

		declarations = append(declarations, functionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  params,
		})
	}

	if len(declarations) == 0 {
		return nil
	}

	return []geminiTool{{FunctionDeclarations: declarations}}
}

func (p *Provider) transformToolChoice(raw json.RawMessage) (*toolConfig, error) {
	// Try as string first
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		switch str {
		case "auto":
			return &toolConfig{FunctionCallingConfig: &functionCallingConfig{Mode: "AUTO"}}, nil
		case "required":
			return &toolConfig{FunctionCallingConfig: &functionCallingConfig{Mode: "ANY"}}, nil
		case "none":
			return &toolConfig{FunctionCallingConfig: &functionCallingConfig{Mode: "NONE"}}, nil
		}
		return nil, nil
	}

	// Try as object with specific function
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	if fn, ok := obj["function"].(map[string]any); ok {
		if name, ok := fn["name"].(string); ok {
			return &toolConfig{
				FunctionCallingConfig: &functionCallingConfig{
					Mode:                 "ANY",
					AllowedFunctionNames: []string{name},
				},
			}, nil
		}
	}

	return nil, nil
}

// ParseResponse transforms a Gemini response into the unified format.
func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.transformResponse(&geminiResp), nil
}

func (p *Provider) transformResponse(resp *geminiResponse) *types.ChatResponse {
	choices := make([]types.Choice, 0, len(resp.Candidates))

	for i, candidate := range resp.Candidates {
		var textContent string
		var toolCalls []types.ToolCall

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textContent += part.Text
			}
			if part.FunctionCall != nil {
				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					argsJSON = []byte("{}")
				}
				toolCalls = append(toolCalls, types.ToolCall{
					ID:   fmt.Sprintf("call_%d", len(toolCalls)),
					Type: "function",
					Function: types.ToolCallFunction{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
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

		choices = append(choices, types.Choice{
			Index:        i,
			Message:      message,
			FinishReason: mapFinishReason(candidate.FinishReason),
		})
	}

	chatResp := &types.ChatResponse{
		Object:  "chat.completion",
		Choices: choices,
	}

	if resp.UsageMetadata != nil {
		chatResp.Usage = &types.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return chatResp
}

func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	default:
		return reason
	}
}

// ParseStreamChunk parses a single SSE chunk from Gemini.
func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// Skip empty lines
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Gemini streaming returns JSON objects directly (not SSE format)
	var resp geminiResponse
	if err := json.Unmarshal(trimmed, &resp); err != nil {
		return nil, nil // Skip unparseable chunks
	}

	if len(resp.Candidates) == 0 {
		return nil, nil
	}

	candidate := resp.Candidates[0]
	var textContent string
	for _, part := range candidate.Content.Parts {
		textContent += part.Text
	}

	chunk := &types.StreamChunk{
		Object: "chat.completion.chunk",
		Choices: []types.StreamChoice{{
			Index: 0,
			Delta: types.StreamDelta{
				Content: textContent,
			},
		}},
	}

	if candidate.FinishReason != "" {
		chunk.Choices[0].FinishReason = mapFinishReason(candidate.FinishReason)
	}

	return chunk, nil
}

// MapError converts a Gemini error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	// Try to parse Gemini error format
	var errResp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
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
