package vertexai

import (
	"context"
	"net/http"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// SupportEmbedding checks if the provider supports embedding requests.
func (p *Provider) SupportEmbedding() bool {
	// Vertex AI supports embeddings, but we need to implement it properly.
	// For now, return false to satisfy the interface.
	return false
}

// BuildEmbeddingRequest creates an HTTP request for the Vertex AI Embedding API.
func (p *Provider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	return nil, errors.NewInvalidRequestError(ProviderName, "", "embedding not yet implemented for vertexai")
}

// ParseEmbeddingResponse transforms a Vertex AI response into the unified format.
func (p *Provider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	return nil, errors.NewInvalidRequestError(ProviderName, "", "embedding not yet implemented for vertexai")
}
