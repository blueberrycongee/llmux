// Package openai implements the OpenAI provider adapter.
// It serves as the reference implementation for other provider adapters.
package openai

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
	ProviderName = "openai"

	// DefaultBaseURL is the default OpenAI API endpoint.
	DefaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the OpenAI API adapter.
type Provider struct {
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
}

// New creates a new OpenAI provider instance.
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
	// Also support models with openai/ prefix
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-")
}

// BuildRequest creates an HTTP request for the OpenAI API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	return httpReq, nil
}

// ParseResponse transforms an OpenAI response into the unified format.
// Since our format is OpenAI-compatible, this is mostly a passthrough.
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
	// Skip empty lines and [DONE] marker
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}

	// Remove "data: " prefix if present
	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
	}

	// Skip [DONE] after prefix removal
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
	// Try to parse OpenAI error format
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

// init registers the OpenAI provider factory.
func init() {
	// Provider factory will be registered in the main package
}
