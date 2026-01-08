package builtin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// CacheBackend defines the interface for cache storage.
type CacheBackend interface {
	Get(key string) (*types.ChatResponse, error)
	Set(key string, resp *types.ChatResponse, ttl time.Duration) error
	Delete(key string) error
	Clear() error
}

// CachePlugin implements response caching.
type CachePlugin struct {
	backend  CacheBackend
	logger   *slog.Logger
	priority int
	ttl      time.Duration

	// KeyFunc generates a cache key from the request.
	KeyFunc func(req *types.ChatRequest) (string, error)
}

// CacheOption configures the CachePlugin.
type CacheOption func(*CachePlugin)

// WithCachePriority sets the plugin priority.
func WithCachePriority(priority int) CacheOption {
	return func(p *CachePlugin) {
		p.priority = priority
	}
}

// WithCacheLogger sets the logger.
func WithCacheLogger(logger *slog.Logger) CacheOption {
	return func(p *CachePlugin) {
		p.logger = logger
	}
}

// WithCacheTTL sets the default TTL for cached items.
func WithCacheTTL(ttl time.Duration) CacheOption {
	return func(p *CachePlugin) {
		p.ttl = ttl
	}
}

// WithCacheKeyFunc sets a custom key generation function.
func WithCacheKeyFunc(fn func(req *types.ChatRequest) (string, error)) CacheOption {
	return func(p *CachePlugin) {
		p.KeyFunc = fn
	}
}

// NewCachePlugin creates a new cache plugin.
// Default priority is 10 (high, runs early to return cached response).
// Default TTL is 1 hour.
func NewCachePlugin(backend CacheBackend, opts ...CacheOption) *CachePlugin {
	p := &CachePlugin{
		backend:  backend,
		priority: 10,
		ttl:      time.Hour,
		KeyFunc:  defaultKeyFunc,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.logger == nil {
		p.logger = slog.Default()
	}

	return p
}

func (p *CachePlugin) Name() string  { return "cache" }
func (p *CachePlugin) Priority() int { return p.priority }

func (p *CachePlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	// Skip caching for streaming requests for now
	if req.Stream {
		return req, nil, nil
	}

	key, err := p.KeyFunc(req)
	if err != nil {
		p.logger.Warn("failed to generate cache key", "error", err)
		return req, nil, nil
	}

	// Check cache
	resp, err := p.backend.Get(key)
	if err != nil {
		// Cache miss or error
		return req, nil, nil
	}

	if resp != nil {
		p.logger.Debug("cache hit", "key", key)

		// Mark as cache hit in context
		ctx.Set("cache_hit", true)
		ctx.Set("cache_key", key)

		return req, &plugin.ShortCircuit{
			Response: resp,
			Metadata: map[string]any{
				"cache_hit": true,
				"cache_key": key,
			},
		}, nil
	}

	// Cache miss, store key for PostHook
	ctx.Set("cache_key", key)
	return req, nil, nil
}

func (p *CachePlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	// Only cache successful responses
	if err != nil || resp == nil {
		return resp, err, nil
	}

	// Don't cache if it was a cache hit
	if hit, ok := ctx.Get("cache_hit"); ok {
		if hitBool, ok := hit.(bool); ok && hitBool {
			return resp, err, nil
		}
	}

	// Get cache key from context
	keyVal, ok := ctx.Get("cache_key")
	if !ok {
		return resp, err, nil
	}
	key, ok := keyVal.(string)
	if !ok {
		return resp, err, nil
	}

	// Store in cache asynchronously
	go func() {
		if setErr := p.backend.Set(key, resp, p.ttl); setErr != nil {
			p.logger.Warn("failed to cache response", "key", key, "error", setErr)
		} else {
			p.logger.Debug("response cached", "key", key)
		}
	}()

	return resp, err, nil
}

func (p *CachePlugin) Cleanup() error {
	return nil
}

// defaultKeyFunc generates a SHA256 hash of the request model and messages.
func defaultKeyFunc(req *types.ChatRequest) (string, error) {
	data := struct {
		Model    string              `json:"model"`
		Messages []types.ChatMessage `json:"messages"`
		Tools    []types.Tool        `json:"tools,omitempty"`
	}{
		Model:    req.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// Ensure CachePlugin implements Plugin interface
var _ plugin.Plugin = (*CachePlugin)(nil)

// =============================================================================
// Memory Cache Backend
// =============================================================================

// MemoryCacheBackend implements an in-memory cache backend.
type MemoryCacheBackend struct {
	items           sync.Map
	maxSize         int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

type memoryCacheItem struct {
	response  *types.ChatResponse
	expiresAt time.Time
}

// MemoryCacheOption configures the MemoryCacheBackend.
type MemoryCacheOption func(*MemoryCacheBackend)

// WithMemoryCacheMaxSize sets the maximum number of items in the cache.
func WithMemoryCacheMaxSize(size int) MemoryCacheOption {
	return func(c *MemoryCacheBackend) {
		c.maxSize = size
	}
}

// WithMemoryCacheCleanupInterval sets the cleanup interval for expired items.
func WithMemoryCacheCleanupInterval(interval time.Duration) MemoryCacheOption {
	return func(c *MemoryCacheBackend) {
		c.cleanupInterval = interval
	}
}

// NewMemoryCacheBackend creates a new in-memory cache backend.
func NewMemoryCacheBackend(opts ...MemoryCacheOption) *MemoryCacheBackend {
	c := &MemoryCacheBackend{
		maxSize:         10000,
		cleanupInterval: 5 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	go c.cleanupLoop()

	return c
}

func (c *MemoryCacheBackend) Get(key string) (*types.ChatResponse, error) {
	val, ok := c.items.Load(key)
	if !ok {
		return nil, nil
	}

	item, ok := val.(memoryCacheItem)
	if !ok {
		return nil, nil
	}
	if time.Now().After(item.expiresAt) {
		c.items.Delete(key)
		return nil, nil
	}

	return item.response, nil
}

func (c *MemoryCacheBackend) Set(key string, resp *types.ChatResponse, ttl time.Duration) error {
	// Simple eviction if over capacity (not strictly LRU for simplicity)
	// In a real implementation, use an LRU cache
	if c.count() >= c.maxSize {
		// Just delete a random key (sync.Map iteration order is random)
		c.items.Range(func(k, v any) bool {
			c.items.Delete(k)
			return false // stop after one deletion
		})
	}

	c.items.Store(key, memoryCacheItem{
		response:  resp,
		expiresAt: time.Now().Add(ttl),
	})
	return nil
}

func (c *MemoryCacheBackend) Delete(key string) error {
	c.items.Delete(key)
	return nil
}

func (c *MemoryCacheBackend) Clear() error {
	c.items.Range(func(key, value any) bool {
		c.items.Delete(key)
		return true
	})
	return nil
}

func (c *MemoryCacheBackend) count() int {
	count := 0
	c.items.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (c *MemoryCacheBackend) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.items.Range(func(key, value any) bool {
				item, ok := value.(memoryCacheItem)
				if ok && now.After(item.expiresAt) {
					c.items.Delete(key)
				}
				return true
			})
		case <-c.stopCleanup:
			return
		}
	}
}

// Ensure MemoryCacheBackend implements CacheBackend interface
var _ CacheBackend = (*MemoryCacheBackend)(nil)
