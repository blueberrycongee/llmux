// Package azure implements the Azure OpenAI provider adapter.
// Azure OpenAI uses the same API format as OpenAI but with different authentication and endpoints.
package azure

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
	ProviderName = "azure"

	// DefaultAPIVersion is the default Azure OpenAI API version.
	DefaultAPIVersion = "2024-02-15-preview"
)

// Provider implements the Azure OpenAI API adapter.
type Provider struct {
	apiKey     string
	baseURL    string
	apiVersion string
	models     []string
	client     *http.Client
}

// New creates a new Azure OpenAI provider instance.
func New(cfg provider.ProviderConfig) (provider.Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		return nil, fmt.Errorf("azure provider requires base_url (e.g., https://your-resource.openai.azure.com)")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiVersion := DefaultAPIVersion
	if v, ok := cfg.Headers["api-version"]; ok {
		apiVersion = v
	}

	return &Provider{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		apiVersion: apiVersion,
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
	return false
}

// BuildRequest creates an HTTP request for the Azure OpenAI API.
// Azure uses deployment names in the URL path instead of model in the body.
func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// Azure uses deployment name in URL, which is typically the model name
	deploymentName := req.Model

	// Build URL with deployment name and API version
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.baseURL, deploymentName, p.apiVersion)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Azure uses api-key header instead of Authorization Bearer
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.apiKey)

	return httpReq, nil
}

// ParseResponse transforms an Azure OpenAI response into the unified format.
// Azure responses are already in OpenAI format, so this is mostly a passthrough.
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

// ParseStreamChunk parses a single SSE chunk from Azure OpenAI.
// Azure uses the same SSE format as OpenAI.
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

// MapError converts an Azure OpenAI error response to a standardized error.
func (p *Provider) MapError(statusCode int, body []byte) error {
	// Try to parse Azure/OpenAI error format
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
