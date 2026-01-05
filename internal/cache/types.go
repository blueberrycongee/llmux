// Package cache provides caching functionality for LLM responses.
// It supports multiple cache backends including in-memory and Redis,
// with optional semantic caching based on embedding similarity.
package cache

import (
	"context"
	"time"
)

// CacheType represents the type of cache backend.
type CacheType string

const (
	CacheTypeLocal         CacheType = "local"          // In-memory cache
	CacheTypeRedis         CacheType = "redis"          // Redis cache
	CacheTypeRedisSemantic CacheType = "redis-semantic" // Redis with semantic similarity
	CacheTypeDual          CacheType = "dual"           // In-memory + Redis dual cache
)

// CacheControl allows per-request cache behavior customization.
// Users can pass this in the request to control caching.
type CacheControl struct {
	TTL       time.Duration `json:"ttl,omitempty"`       // Custom TTL for this request
	Namespace string        `json:"namespace,omitempty"` // Namespace isolation
	NoCache   bool          `json:"no-cache,omitempty"`  // Skip cache read (force fresh)
	NoStore   bool          `json:"no-store,omitempty"`  // Skip cache write
	MaxAge    time.Duration `json:"s-maxage,omitempty"`  // Max age for cache validity
}

// CachedResponse represents a cached LLM response with metadata.
type CachedResponse struct {
	Timestamp int64  `json:"timestamp"`          // Unix timestamp when cached
	Response  []byte `json:"response"`           // Serialized response
	Model     string `json:"model,omitempty"`    // Model used for the response
	Provider  string `json:"provider,omitempty"` // Provider that generated the response
}

// CacheStats holds cache statistics for monitoring.
type CacheStats struct {
	Hits       int64         `json:"hits"`
	Misses     int64         `json:"misses"`
	Sets       int64         `json:"sets"`
	Deletes    int64         `json:"deletes"`
	Errors     int64         `json:"errors"`
	HitRate    float64       `json:"hit_rate"`
	AvgLatency time.Duration `json:"avg_latency"`
}

// CacheEntry represents a single cache entry for pipeline operations.
type CacheEntry struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

// Cache defines the interface for all cache implementations.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns nil, nil if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with the given TTL.
	// If TTL is 0, the default TTL is used.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	Delete(ctx context.Context, key string) error

	// SetPipeline performs batch set operations for efficiency.
	SetPipeline(ctx context.Context, entries []CacheEntry) error

	// GetMulti retrieves multiple keys at once.
	// Returns a map of key -> value, missing keys are not included.
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)

	// Ping checks if the cache is healthy.
	Ping(ctx context.Context) error

	// Close releases any resources held by the cache.
	Close() error

	// Stats returns cache statistics.
	Stats() CacheStats
}

// KeyGenerator defines the interface for generating cache keys.
type KeyGenerator interface {
	// Generate creates a cache key from the request parameters.
	Generate(params KeyParams) string
}

// KeyParams contains the parameters used to generate a cache key.
type KeyParams struct {
	Model       string            `json:"model"`
	Messages    []byte            `json:"messages"`    // Serialized messages
	Temperature *float64          `json:"temperature"` // nil means not set
	MaxTokens   int               `json:"max_tokens"`
	TopP        *float64          `json:"top_p"`
	Tools       []byte            `json:"tools,omitempty"` // Serialized tools
	Namespace   string            `json:"namespace,omitempty"`
	Extra       map[string][]byte `json:"extra,omitempty"` // Provider-specific params
}
