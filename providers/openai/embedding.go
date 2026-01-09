package openai

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
	return true
}

// BuildEmbeddingRequest creates an HTTP request for the OpenAI Embedding API.
func (p *Provider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	// Validate input before sending to API
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid embedding request: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimSuffix(p.baseURL, "/") + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Get token from TokenSource or fallback to apiKey
	token, err := provider.GetToken(p.tokenSource, p.apiKey)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

// ParseEmbeddingResponse transforms an OpenAI response into the unified format.
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
