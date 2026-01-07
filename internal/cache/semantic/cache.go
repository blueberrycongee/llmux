package semantic

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/blueberrycongee/llmux/internal/cache/semantic/embedding"
	"github.com/blueberrycongee/llmux/internal/cache/semantic/vector"
)

// Cache implements semantic caching using vector similarity.
// It stores LLM responses indexed by embedding vectors of the prompts,
// allowing cache hits for semantically similar (but not identical) prompts.
type Cache struct {
	embedder            embedding.Embedder
	vectorStore         vector.Store
	similarityThreshold float64
	defaultTTL          time.Duration

	// Statistics
	hits       atomic.Int64
	misses     atomic.Int64
	sets       atomic.Int64
	errors     atomic.Int64
	embedCalls atomic.Int64
}

// New creates a new semantic cache with the given embedder and vector store.
func New(embedder embedding.Embedder, store vector.Store, cfg Config) (*Cache, error) {
	if embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if store == nil {
		return nil, fmt.Errorf("vector store is required")
	}

	if cfg.SimilarityThreshold <= 0 || cfg.SimilarityThreshold > 1 {
		cfg.SimilarityThreshold = 0.95
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = time.Hour
	}

	return &Cache{
		embedder:            embedder,
		vectorStore:         store,
		similarityThreshold: cfg.SimilarityThreshold,
		defaultTTL:          cfg.DefaultTTL,
	}, nil
}

// Get retrieves a cached response for a semantically similar prompt.
// Returns the cached response and similarity score if found, nil otherwise.
func (c *Cache) Get(ctx context.Context, prompt string) (*CacheResult, error) {
	if prompt == "" {
		c.misses.Add(1)
		return nil, nil
	}

	// Generate embedding for the prompt
	emb, err := c.embedder.Embed(ctx, prompt)
	if err != nil {
		c.errors.Add(1)
		return nil, fmt.Errorf("generate embedding: %w", err)
	}
	c.embedCalls.Add(1)

	// Search for similar vectors
	results, err := c.vectorStore.Search(ctx, emb, vector.SearchOptions{
		TopK:              1,
		DistanceThreshold: 1 - c.similarityThreshold, // Convert similarity to distance
	})
	if err != nil {
		c.errors.Add(1)
		return nil, fmt.Errorf("vector search: %w", err)
	}

	if len(results) == 0 {
		c.misses.Add(1)
		return nil, nil
	}

	// Check similarity threshold
	result := results[0]
	similarity := result.Score // Qdrant returns cosine similarity directly

	if similarity < c.similarityThreshold {
		c.misses.Add(1)
		return nil, nil
	}

	c.hits.Add(1)
	return &CacheResult{
		Response:     result.Payload.Response,
		Similarity:   similarity,
		CachedPrompt: result.Payload.Prompt,
		Model:        result.Payload.Model,
	}, nil
}

// Set stores a response in the semantic cache.
func (c *Cache) Set(ctx context.Context, prompt, response, model string, ttl time.Duration) error {
	if prompt == "" || response == "" {
		return nil
	}

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	// Generate embedding for the prompt
	emb, err := c.embedder.Embed(ctx, prompt)
	if err != nil {
		c.errors.Add(1)
		return fmt.Errorf("generate embedding: %w", err)
	}
	c.embedCalls.Add(1)

	// Store in vector database
	entry := vector.Entry{
		ID:     uuid.New().String(),
		Vector: emb,
		Payload: vector.Payload{
			Prompt:    prompt,
			Response:  response,
			Model:     model,
			CreatedAt: time.Now().Unix(),
		},
		TTL: ttl,
	}

	if err := c.vectorStore.Insert(ctx, entry); err != nil {
		c.errors.Add(1)
		return fmt.Errorf("vector insert: %w", err)
	}

	c.sets.Add(1)
	return nil
}

// SetBatch stores multiple responses in the semantic cache.
func (c *Cache) SetBatch(ctx context.Context, entries []CacheEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Extract prompts for batch embedding
	prompts := make([]string, len(entries))
	for i, e := range entries {
		prompts[i] = e.Prompt
	}

	// Generate embeddings in batch
	embeddings, err := c.embedder.EmbedBatch(ctx, prompts)
	if err != nil {
		c.errors.Add(1)
		return fmt.Errorf("generate embeddings: %w", err)
	}
	c.embedCalls.Add(int64(len(prompts)))

	// Prepare vector entries
	vectorEntries := make([]vector.Entry, len(entries))
	now := time.Now().Unix()

	for i, e := range entries {
		ttl := e.TTL
		if ttl <= 0 {
			ttl = c.defaultTTL
		}

		vectorEntries[i] = vector.Entry{
			ID:     uuid.New().String(),
			Vector: embeddings[i],
			Payload: vector.Payload{
				Prompt:    e.Prompt,
				Response:  e.Response,
				Model:     e.Model,
				CreatedAt: now,
			},
			TTL: ttl,
		}
	}

	// Insert batch
	if err := c.vectorStore.InsertBatch(ctx, vectorEntries); err != nil {
		c.errors.Add(1)
		return fmt.Errorf("vector insert batch: %w", err)
	}

	c.sets.Add(int64(len(entries)))
	return nil
}

// Delete removes a cached entry by its prompt.
// Note: This requires searching for the prompt first, which may not be exact.
func (c *Cache) Delete(ctx context.Context, prompt string) error {
	// Generate embedding to find the entry
	emb, err := c.embedder.Embed(ctx, prompt)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	// Search for exact or very similar match
	results, err := c.vectorStore.Search(ctx, emb, vector.SearchOptions{
		TopK:              1,
		DistanceThreshold: 0.01, // Very strict for deletion
	})
	if err != nil {
		return fmt.Errorf("vector search: %w", err)
	}

	if len(results) == 0 {
		return nil // Nothing to delete
	}

	return c.vectorStore.Delete(ctx, results[0].ID)
}

// Ping checks if the cache is healthy.
func (c *Cache) Ping(ctx context.Context) error {
	return c.vectorStore.Ping(ctx)
}

// Close releases resources held by the cache.
func (c *Cache) Close() error {
	return c.vectorStore.Close()
}

// Stats returns cache statistics.
func (c *Cache) Stats() Stats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return Stats{
		Hits:       hits,
		Misses:     misses,
		Sets:       c.sets.Load(),
		Errors:     c.errors.Load(),
		EmbedCalls: c.embedCalls.Load(),
		HitRate:    hitRate,
	}
}

// SimilarityThreshold returns the configured similarity threshold.
func (c *Cache) SimilarityThreshold() float64 {
	return c.similarityThreshold
}

// CacheResult represents a semantic cache hit.
type CacheResult struct {
	// Response is the cached LLM response.
	Response string

	// Similarity is the cosine similarity score (0-1).
	Similarity float64

	// CachedPrompt is the original prompt that was cached.
	CachedPrompt string

	// Model is the model that generated the cached response.
	Model string
}

// CacheEntry represents an entry to be cached.
type CacheEntry struct {
	Prompt   string
	Response string
	Model    string
	TTL      time.Duration
}

// Stats holds semantic cache statistics.
type Stats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Sets       int64   `json:"sets"`
	Errors     int64   `json:"errors"`
	EmbedCalls int64   `json:"embed_calls"`
	HitRate    float64 `json:"hit_rate"`
}
