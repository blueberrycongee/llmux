// Package openailike provides a base implementation for OpenAI-compatible providers.
// Most LLM providers follow OpenAI's API format with minor variations.
// This package reduces code duplication by providing a common foundation.
package openailike

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

// ProviderInfo contains provider-specific configuration.
type ProviderInfo struct {
	// Name is the provider identifier (e.g., "groq", "deepseek")
	Name string

	// DefaultBaseURL is the default API endpoint
	DefaultBaseURL string

	// APIKeyHeader is the header name for API key authentication
	// Default: "Authorization" with "Bearer " prefix
	APIKeyHeader string

	// APIKeyPrefix is the prefix for the API key value
	// Default: "Bearer "
	APIKeyPrefix string

	// ChatEndpoint is the path for chat completions
	// Default: "/chat/completions"
	ChatEndpoint string

	// SupportsStreaming indicates if the provider supports SSE streaming
	SupportsStreaming bool

	// ExtraHeaders are additional headers to include in requests
	ExtraHeaders map[string]string

	// ModelPrefixes are prefixes that identify models for this provider
	ModelPrefixes []string
}

// Provider implements a generic OpenAI-compatible LLM provider adapter.
type Provider struct {
	info    ProviderInfo
	apiKey  string
	baseURL string
	models  []string
	client  *http.Client
}

// New creates a new OpenAI-like provider instance.
func New(cfg provider.ProviderConfig, info ProviderInfo) (provider.Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = info.DefaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Provider{
		info:    info,
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		models:  cfg.Models,
		client:  &http.Client{},
	}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return p.info.Name
}

// SupportedModels returns the list of supported models.
func (p *Provider) SupportedModels() []string {
	return p.models
}

// SupportsModel checks if the provider supports the given model.
func (p *Provider) SupportsModel(model string) bool {
	// Check explicit model list
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	// Check model prefixes
	for _, prefix := range p.info.ModelPrefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// BuildRequest creates an HTTP request for the provider's API.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := p.info.ChatEndpoint
	if endpoint == "" {
		endpoint = "/chat/completions"
	}

	url := p.baseURL + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Set API key header
	apiKeyHeader := p.info.APIKeyHeader
	if apiKeyHeader == "" {
		apiKeyHeader = "Authorization"
	}
	apiKeyPrefix := p.info.APIKeyPrefix
	if apiKeyPrefix == "" && apiKeyHeader == "Authorization" {
		apiKeyPrefix = "Bearer "
	}
	httpReq.Header.Set(apiKeyHeader, apiKeyPrefix+p.apiKey)

	// Add extra headers
	for k, v := range p.info.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

// ParseResponse transforms the provider's response into the unified format.
// Since OpenAI-like providers follow the same format, this is mostly a passthrough.
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

// ParseStreamChunk parses a single SSE chunk.
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

// MapError converts a provider-specific error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	// Try to parse OpenAI-compatible error format
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

	providerName := p.info.Name

	switch statusCode {
	case http.StatusUnauthorized:
		return llmerrors.NewAuthenticationError(providerName, "", message)
	case http.StatusTooManyRequests:
		return llmerrors.NewRateLimitError(providerName, "", message)
	case http.StatusBadRequest:
		return llmerrors.NewInvalidRequestError(providerName, "", message)
	case http.StatusNotFound:
		return llmerrors.NewNotFoundError(providerName, "", message)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return llmerrors.NewTimeoutError(providerName, "", message)
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return llmerrors.NewServiceUnavailableError(providerName, "", message)
	default:
		return llmerrors.NewInternalError(providerName, "", message)
	}
}

// GetInfo returns the provider information.
func (p *Provider) GetInfo() ProviderInfo {
	return p.info
}

// GetAPIKey returns the API key.
func (p *Provider) GetAPIKey() string {
	return p.apiKey
}

// GetBaseURL returns the base URL.
func (p *Provider) GetBaseURL() string {
	return p.baseURL
}
