package openailike

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// SupportEmbedding checks if the provider supports embedding requests.
func (p *Provider) SupportEmbedding() bool {
	return p.info.SupportsEmbedding
}

// BuildEmbeddingRequest creates an HTTP request for the provider's embedding API.
func (p *Provider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	// Validate input before sending to API
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid embedding request: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := p.info.EmbeddingEndpoint
	if endpoint == "" {
		endpoint = "/embeddings"
	}

	url := strings.TrimSuffix(p.baseURL, "/") + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Get token from TokenSource or fallback to apiKey
	token, err := provider.GetToken(p.tokenSource, p.apiKey)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	// Set API key header
	apiKeyHeader := p.info.APIKeyHeader
	if apiKeyHeader == "" {
		apiKeyHeader = "Authorization"
	}
	apiKeyPrefix := p.info.APIKeyPrefix
	if apiKeyPrefix == "" && apiKeyHeader == "Authorization" {
		apiKeyPrefix = "Bearer "
	}
	httpReq.Header.Set(apiKeyHeader, apiKeyPrefix+token)

	// Add extra headers from info
	for k, v := range p.info.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}

	// Add custom headers
	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

// ParseEmbeddingResponse transforms the provider's response into the unified format.
func (p *Provider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var embResp types.EmbeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &embResp, nil
}
