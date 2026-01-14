// Package gemini provides the Google Gemini provider for LLMux library mode.
// It handles request/response transformation between OpenAI format and Gemini's generateContent API.
package gemini

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	ProviderName      = "gemini"
	DefaultBaseURL    = "https://generativelanguage.googleapis.com"
	DefaultAPIVersion = "v1beta"
)

type Provider struct {
	apiKey      string
	tokenSource provider.TokenSource
	baseURL     string
	apiVersion  string
	models      []string
	headers     map[string]string
}

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

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	opts := []Option{
		WithAPIKey(cfg.APIKey),
		WithBaseURL(cfg.BaseURL),
		WithModels(cfg.Models...),
	}
	if cfg.TokenSource != nil {
		opts = append(opts, WithTokenSource(cfg.TokenSource))
	}
	p := New(opts...)
	for k, v := range cfg.Headers {
		p.headers[k] = v
	}
	return p, nil
}

func (p *Provider) Name() string              { return ProviderName }
func (p *Provider) SupportedModels() []string { return p.models }
func (p *Provider) SupportsModel(model string) bool {
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	return strings.HasPrefix(model, "gemini-")
}

type geminiRequest struct {
	Contents          []geminiContent   `json:"contents"`
	SystemInstruction *geminiContent    `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiResponse struct {
	Candidates    []candidate    `json:"candidates"`
	UsageMetadata *usageMetadata `json:"usageMetadata,omitempty"`
}

type candidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	geminiReq := p.transformRequest(req)
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	action := "generateContent"
	if req.Stream {
		action = "streamGenerateContent"
	}
	// Get token from TokenSource or fallback to apiKey
	token, err := provider.GetToken(p.tokenSource, p.apiKey)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	base, err := url.Parse(strings.TrimSuffix(p.baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse base_url: %w", err)
	}
	base.Path = base.Path + "/" + p.apiVersion + "/models/" + url.PathEscape(req.Model) + ":" + action
	q := base.Query()
	q.Set("key", token)
	base.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}

func (p *Provider) transformRequest(req *types.ChatRequest) *geminiRequest {
	geminiReq := &geminiRequest{GenerationConfig: &generationConfig{}}
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

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			var content string
			if err := json.Unmarshal(msg.Content, &content); err == nil {
				geminiReq.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: content}}}
			}
			continue
		}
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		var content string
		if err := json.Unmarshal(msg.Content, &content); err == nil {
			geminiReq.Contents = append(geminiReq.Contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: content}},
			})
		}
	}
	return geminiReq
}

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
	for i, c := range resp.Candidates {
		var text string
		for _, part := range c.Content.Parts {
			text += part.Text
		}
		choices = append(choices, types.Choice{
			Index:        i,
			Message:      types.ChatMessage{Role: "assistant", Content: json.RawMessage(fmt.Sprintf("%q", text))},
			FinishReason: mapFinishReason(c.FinishReason),
		})
	}
	chatResp := &types.ChatResponse{Object: "chat.completion", Choices: choices}
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
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		return reason
	}
}

func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var resp geminiResponse
	if err := json.Unmarshal(trimmed, &resp); err != nil {
		return nil, nil
	}
	if len(resp.Candidates) == 0 {
		return nil, nil
	}
	c := resp.Candidates[0]
	var text string
	for _, part := range c.Content.Parts {
		text += part.Text
	}
	chunk := &types.StreamChunk{
		Object:  "chat.completion.chunk",
		Choices: []types.StreamChoice{{Index: 0, Delta: types.StreamDelta{Content: text}}},
	}
	if c.FinishReason != "" {
		chunk.Choices[0].FinishReason = mapFinishReason(c.FinishReason)
	}
	return chunk, nil
}

func (p *Provider) MapError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
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
