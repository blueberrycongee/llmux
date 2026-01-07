package cache

import (
	"context"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// Handler provides high-level caching operations for LLM requests.
// It wraps the underlying cache implementation and handles serialization,
// key generation, and cache control logic.
type Handler struct {
	cache   Cache
	keyGen  KeyGenerator
	config  HandlerConfig
	enabled bool
}

// HandlerConfig holds configuration for the cache handler.
type HandlerConfig struct {
	Enabled            bool          `yaml:"enabled"`
	DefaultTTL         time.Duration `yaml:"default_ttl"`
	SupportedCallTypes []string      `yaml:"supported_call_types"` // e.g., ["completion", "embedding"]
	MaxCacheableSize   int           `yaml:"max_cacheable_size"`   // Max response size to cache (bytes)
}

// DefaultHandlerConfig returns sensible defaults.
func DefaultHandlerConfig() HandlerConfig {
	return HandlerConfig{
		Enabled:            true,
		DefaultTTL:         time.Hour,
		SupportedCallTypes: []string{"completion", "acompletion"},
		MaxCacheableSize:   10 * 1024 * 1024, // 10MB
	}
}

// NewHandler creates a new cache handler.
func NewHandler(cache Cache, keyGen KeyGenerator, cfg HandlerConfig) *Handler {
	if keyGen == nil {
		keyGen = NewKeyGenerator("llmux")
	}
	return &Handler{
		cache:   cache,
		keyGen:  keyGen,
		config:  cfg,
		enabled: cfg.Enabled,
	}
}

// GetCachedResponse attempts to retrieve a cached response for the given request.
// Returns nil if cache is disabled, cache miss, or cache control says no-cache.
func (h *Handler) GetCachedResponse(ctx context.Context, req *types.ChatRequest, ctrl *CacheControl) (*CachedResponse, error) {
	if !h.enabled || h.cache == nil {
		return nil, nil
	}

	// Check cache control
	if ctrl != nil && ctrl.NoCache {
		return nil, nil
	}

	// Generate cache key
	key, err := h.generateKey(req, ctrl)
	if err != nil {
		return nil, err
	}

	// Get from cache
	data, err := h.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	// Deserialize cached response
	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		// Invalid cache entry, treat as miss
		return nil, nil
	}

	// Check max-age if specified
	if ctrl != nil && ctrl.MaxAge > 0 {
		age := time.Since(time.Unix(cached.Timestamp, 0))
		if age > ctrl.MaxAge {
			return nil, nil // Cache entry too old
		}
	}

	return &cached, nil
}

// SetCachedResponse stores a response in the cache.
// Returns immediately if cache is disabled or cache control says no-store.
func (h *Handler) SetCachedResponse(ctx context.Context, req *types.ChatRequest, resp []byte, ctrl *CacheControl) error {
	if !h.enabled || h.cache == nil {
		return nil
	}

	// Check cache control
	if ctrl != nil && ctrl.NoStore {
		return nil
	}

	// Check response size
	if len(resp) > h.config.MaxCacheableSize {
		return nil // Too large to cache
	}

	// Generate cache key
	key, err := h.generateKey(req, ctrl)
	if err != nil {
		return err
	}

	// Create cached response with metadata
	cached := CachedResponse{
		Timestamp: time.Now().Unix(),
		Response:  resp,
		Model:     req.Model,
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	// Determine TTL
	ttl := h.config.DefaultTTL
	if ctrl != nil && ctrl.TTL > 0 {
		ttl = ctrl.TTL
	}

	return h.cache.Set(ctx, key, data, ttl)
}

// generateKey creates a cache key from the request.
func (h *Handler) generateKey(req *types.ChatRequest, ctrl *CacheControl) (string, error) {
	// Serialize messages
	messages, err := json.Marshal(req.Messages)
	if err != nil {
		return "", err
	}

	// Serialize tools if present
	var tools []byte
	if len(req.Tools) > 0 {
		tools, _ = json.Marshal(req.Tools)
	}

	// Build key params
	params := KeyParams{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Tools:       tools,
	}

	// Add namespace from cache control
	if ctrl != nil && ctrl.Namespace != "" {
		params.Namespace = ctrl.Namespace
	}

	return h.keyGen.Generate(params), nil
}

// InvalidateCache removes a cached response for the given request.
func (h *Handler) InvalidateCache(ctx context.Context, req *types.ChatRequest, ctrl *CacheControl) error {
	if !h.enabled || h.cache == nil {
		return nil
	}

	key, err := h.generateKey(req, ctrl)
	if err != nil {
		return err
	}

	return h.cache.Delete(ctx, key)
}

// Stats returns cache statistics.
func (h *Handler) Stats() CacheStats {
	if h.cache == nil {
		return CacheStats{}
	}
	return h.cache.Stats()
}

// IsEnabled returns whether caching is enabled.
func (h *Handler) IsEnabled() bool {
	return h.enabled
}

// SetEnabled enables or disables caching at runtime.
func (h *Handler) SetEnabled(enabled bool) {
	h.enabled = enabled
}

// Ping checks cache health.
func (h *Handler) Ping(ctx context.Context) error {
	if h.cache == nil {
		return nil
	}
	return h.cache.Ping(ctx)
}

// Close releases cache resources.
func (h *Handler) Close() error {
	if h.cache == nil {
		return nil
	}
	return h.cache.Close()
}

// ParseCacheControl extracts cache control settings from request.
// This can be called by the API handler to get cache control from the request body.
func ParseCacheControl(raw json.RawMessage) *CacheControl {
	if len(raw) == 0 {
		return nil
	}

	var ctrl CacheControl
	if err := json.Unmarshal(raw, &ctrl); err != nil {
		return nil
	}

	return &ctrl
}
