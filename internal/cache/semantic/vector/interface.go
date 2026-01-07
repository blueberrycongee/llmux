// Package vector provides vector storage interfaces and implementations
// for semantic caching functionality.
package vector

import (
	"context"
	"time"
)

// Store defines the interface for vector storage backends.
type Store interface {
	// Search finds similar vectors within the distance threshold.
	// Returns results sorted by similarity (most similar first).
	Search(ctx context.Context, vector []float64, opts SearchOptions) ([]SearchResult, error)

	// Insert stores a vector with associated payload.
	Insert(ctx context.Context, entry Entry) error

	// InsertBatch stores multiple vectors in a single operation.
	InsertBatch(ctx context.Context, entries []Entry) error

	// Delete removes a vector by ID.
	Delete(ctx context.Context, id string) error

	// Ping checks if the vector store is healthy.
	Ping(ctx context.Context) error

	// Close releases resources held by the store.
	Close() error
}

// SearchOptions configures vector search behavior.
type SearchOptions struct {
	// TopK is the maximum number of results to return.
	TopK int

	// DistanceThreshold is the maximum distance for a result to be included.
	// For cosine distance: 0 = identical, 2 = opposite.
	// Results with distance > DistanceThreshold are excluded.
	DistanceThreshold float64
}

// SearchResult represents a single search result.
type SearchResult struct {
	// ID is the unique identifier of the vector.
	ID string

	// Score is the similarity score (for cosine: 1 = identical, 0 = orthogonal, -1 = opposite).
	// Note: Qdrant returns score directly, while distance = 1 - score for cosine.
	Score float64

	// Distance is the vector distance (for cosine: 0 = identical, 2 = opposite).
	Distance float64

	// Payload contains the cached data associated with this vector.
	Payload Payload
}

// Entry represents a vector entry to be stored.
type Entry struct {
	// ID is the unique identifier for this entry.
	// If empty, a UUID will be generated.
	ID string

	// Vector is the embedding vector.
	Vector []float64

	// Payload contains the data to cache.
	Payload Payload

	// TTL is the time-to-live for this entry.
	// If zero, the entry does not expire.
	TTL time.Duration
}

// Payload contains the cached prompt and response.
type Payload struct {
	// Prompt is the original prompt text used to generate the embedding.
	Prompt string `json:"prompt"`

	// Response is the cached LLM response.
	Response string `json:"response"`

	// Model is the model that generated the response.
	Model string `json:"model,omitempty"`

	// CreatedAt is the timestamp when this entry was created.
	CreatedAt int64 `json:"created_at,omitempty"`
}
