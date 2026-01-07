// Package openai provides the OpenAI provider for LLMux library mode.
// It serves as the reference implementation for other provider adapters.
package openai

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
	ProviderName = "openai"

	// DefaultBaseURL is the default OpenAI API endpoint.
	DefaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the OpenAI API adapter.
type Provider struct {
	apiKey  string
	baseURL string
	models  []string
	headers map[string]string
}

// New creates a new OpenAI provider with the given options.
func New(opts ...Option) *Provider {
	p := &Provider{
		baseURL: DefaultBaseURL,
		headers: make(map[string]string),
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
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-")
}

// BuildRequest creates an HTTP request for the OpenAI API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

// ParseResponse transforms an OpenAI response into the unified format.
func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var chatResp types.ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// ParseStreamChunk parses a single SSE chunk from OpenAI.
func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}

	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
	}

	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}

	var chunk types.StreamChunk
	if err := json.Unmarshal(trimmed, &chunk); err != nil {
		return nil, fmt.Errorf("unmarshal chunk: %w", err)
	}

	return &chunk, nil
}

// MapError converts an OpenAI error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
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
