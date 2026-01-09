package anthropic

import (
	"context"
	"net/http"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// SupportEmbedding checks if the provider supports embedding requests.
func (p *Provider) SupportEmbedding() bool {
	return false
}

// BuildEmbeddingRequest creates an HTTP request for the Anthropic Embedding API.
func (p *Provider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	return nil, errors.NewInvalidRequestError(ProviderName, "", "embedding not supported by anthropic")
}

// ParseEmbeddingResponse transforms an Anthropic response into the unified format.
func (p *Provider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	return nil, errors.NewInvalidRequestError(ProviderName, "", "embedding not supported by anthropic")
}
