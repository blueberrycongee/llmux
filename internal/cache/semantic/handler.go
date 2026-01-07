package semantic

import (
	"context"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// Handler provides high-level semantic caching operations for LLM requests.
// It wraps the semantic cache and handles message-to-prompt conversion.
type Handler struct {
	cache   *Cache
	config  HandlerConfig
	enabled bool
}

// HandlerConfig holds configuration for the semantic cache handler.
type HandlerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	DefaultTTL       time.Duration `yaml:"default_ttl"`
	MaxCacheableSize int           `yaml:"max_cacheable_size"` // Max response size to cache (bytes)
}

// DefaultHandlerConfig returns sensible defaults.
func DefaultHandlerConfig() HandlerConfig {
	return HandlerConfig{
		Enabled:          true,
		DefaultTTL:       time.Hour,
		MaxCacheableSize: 10 * 1024 * 1024, // 10MB
	}
}

// NewHandler creates a new semantic cache handler.
func NewHandler(cache *Cache, cfg HandlerConfig) *Handler {
	return &Handler{
		cache:   cache,
		config:  cfg,
		enabled: cfg.Enabled,
	}
}

// CacheControl allows per-request cache behavior customization.
type CacheControl struct {
	TTL       time.Duration `json:"ttl,omitempty"`
	Namespace string        `json:"namespace,omitempty"`
	NoCache   bool          `json:"no-cache,omitempty"` // Skip cache read
	NoStore   bool          `json:"no-store,omitempty"` // Skip cache write
}

// CachedResponse represents a cached LLM response with metadata.
type CachedResponse struct {
	Response     []byte  `json:"response"`
	Similarity   float64 `json:"similarity"`
	CachedPrompt string  `json:"cached_prompt,omitempty"`
	Model        string  `json:"model,omitempty"`
}

// GetCachedResponse attempts to retrieve a semantically similar cached response.
func (h *Handler) GetCachedResponse(ctx context.Context, req *types.ChatRequest, ctrl *CacheControl) (*CachedResponse, error) {
	if !h.enabled || h.cache == nil {
		return nil, nil
	}

	// Check cache control
	if ctrl != nil && ctrl.NoCache {
		return nil, nil
	}

	// Convert messages to prompt
	prompt := MessagesToPrompt(req.Messages)
	if prompt == "" {
		return nil, nil
	}

	// Get from semantic cache
	result, err := h.cache.Get(ctx, prompt)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	return &CachedResponse{
		Response:     []byte(result.Response),
		Similarity:   result.Similarity,
		CachedPrompt: result.CachedPrompt,
		Model:        result.Model,
	}, nil
}

// SetCachedResponse stores a response in the semantic cache.
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

	// Convert messages to prompt
	prompt := MessagesToPrompt(req.Messages)
	if prompt == "" {
		return nil
	}

	// Determine TTL
	ttl := h.config.DefaultTTL
	if ctrl != nil && ctrl.TTL > 0 {
		ttl = ctrl.TTL
	}

	return h.cache.Set(ctx, prompt, string(resp), req.Model, ttl)
}

// InvalidateCache removes cached responses for similar prompts.
func (h *Handler) InvalidateCache(ctx context.Context, req *types.ChatRequest) error {
	if !h.enabled || h.cache == nil {
		return nil
	}

	prompt := MessagesToPrompt(req.Messages)
	if prompt == "" {
		return nil
	}

	return h.cache.Delete(ctx, prompt)
}

// Stats returns cache statistics.
func (h *Handler) Stats() Stats {
	if h.cache == nil {
		return Stats{}
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

// SimilarityThreshold returns the configured similarity threshold.
func (h *Handler) SimilarityThreshold() float64 {
	if h.cache == nil {
		return 0
	}
	return h.cache.SimilarityThreshold()
}

// ParseCacheControl extracts cache control settings from request.
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
