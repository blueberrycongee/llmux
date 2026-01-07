// Package embedding provides interfaces and implementations for generating
// text embeddings used in semantic caching.
package embedding

import "context"

// Embedder defines the interface for generating text embeddings.
type Embedder interface {
	// Embed generates an embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch generates embeddings for multiple texts in a single request.
	// This is more efficient than calling Embed multiple times.
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

	// Model returns the name of the embedding model being used.
	Model() string

	// Dimension returns the dimension of the embedding vectors.
	Dimension() int
}
