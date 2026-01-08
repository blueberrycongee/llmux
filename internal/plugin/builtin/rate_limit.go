package builtin

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// RateLimitPlugin implements request-level rate limiting using a token bucket algorithm.
type RateLimitPlugin struct {
	limiter  *tokenBucket
	logger   *slog.Logger
	priority int

	// KeyFunc extracts a rate limit key from the context.
	// Default uses the context's Auth.APIKey if available, otherwise "global".
	KeyFunc func(ctx *plugin.Context) string

	// per-key limiters for distributed rate limiting
	perKeyLimiters sync.Map
	rate           float64
	burst          int
}

// RateLimitOption configures the RateLimitPlugin.
type RateLimitOption func(*RateLimitPlugin)

// WithRateLimitPriority sets the plugin priority.
func WithRateLimitPriority(priority int) RateLimitOption {
	return func(p *RateLimitPlugin) {
		p.priority = priority
	}
}

// WithRateLimitLogger sets the logger.
func WithRateLimitLogger(logger *slog.Logger) RateLimitOption {
	return func(p *RateLimitPlugin) {
		p.logger = logger
	}
}

// WithRateLimitKeyFunc sets a custom key extraction function.
func WithRateLimitKeyFunc(fn func(ctx *plugin.Context) string) RateLimitOption {
	return func(p *RateLimitPlugin) {
		p.KeyFunc = fn
	}
}

// WithPerKeyLimiting enables per-key rate limiting instead of global.
func WithPerKeyLimiting() RateLimitOption {
	return func(p *RateLimitPlugin) {
		// This enables per-key limiting mode
		// Each unique key gets its own rate limiter
	}
}

// NewRateLimitPlugin creates a new rate limit plugin.
// rate: requests per second allowed
// burst: maximum burst size (tokens in bucket)
// Default priority is 5 (very high, runs early to reject requests quickly).
func NewRateLimitPlugin(rate float64, burst int, opts ...RateLimitOption) *RateLimitPlugin {
	p := &RateLimitPlugin{
		limiter:  newTokenBucket(rate, burst),
		priority: 5,
		rate:     rate,
		burst:    burst,
		KeyFunc: func(ctx *plugin.Context) string {
			if ctx.Auth != nil && ctx.Auth.APIKey != nil && ctx.Auth.APIKey.ID != "" {
				return ctx.Auth.APIKey.ID
			}
			return "global"
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.logger == nil {
		p.logger = slog.Default()
	}

	return p
}

func (p *RateLimitPlugin) Name() string  { return "rate-limit" }
func (p *RateLimitPlugin) Priority() int { return p.priority }

func (p *RateLimitPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	key := p.KeyFunc(ctx)

	// Get or create per-key limiter
	limiter := p.getLimiter(key)

	if !limiter.allow() {
		p.logger.Warn("rate limit exceeded",
			"request_id", ctx.RequestID,
			"key", key,
			"model", req.Model,
		)

		return req, &plugin.ShortCircuit{
			Error:         NewRateLimitError(req.Model),
			AllowFallback: false, // Don't fallback on rate limit
			Metadata: map[string]any{
				"rate_limited": true,
				"limit_key":    key,
			},
		}, nil
	}

	p.logger.Debug("rate limit check passed",
		"request_id", ctx.RequestID,
		"key", key,
	)

	return req, nil, nil
}

func (p *RateLimitPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	// Nothing to do in PostHook for rate limiting
	return resp, err, nil
}

func (p *RateLimitPlugin) Cleanup() error {
	return nil
}

func (p *RateLimitPlugin) getLimiter(key string) *tokenBucket {
	if key == "global" {
		return p.limiter
	}

	// Per-key limiter
	if limiter, ok := p.perKeyLimiters.Load(key); ok {
		if limiterTyped, ok := limiter.(*tokenBucket); ok {
			return limiterTyped
		}
	}

	// Create new limiter for this key
	newLimiter := newTokenBucket(p.rate, p.burst)
	actual, _ := p.perKeyLimiters.LoadOrStore(key, newLimiter)
	if actualTyped, ok := actual.(*tokenBucket); ok {
		return actualTyped
	}
	return newLimiter
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	rate     float64   // tokens per second
	burst    int       // max tokens
	tokens   float64   // current tokens
	lastTime time.Time // last refill time
	mu       sync.Mutex
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		rate:     rate,
		burst:    burst,
		tokens:   float64(burst),
		lastTime: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	// Refill tokens
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	// Check if we can consume a token
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimitError represents a rate limit exceeded error.
type RateLimitError struct {
	Model   string
	Message string
}

func NewRateLimitError(model string) *RateLimitError {
	return &RateLimitError{
		Model:   model,
		Message: "rate limit exceeded",
	}
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// Is implements errors.Is support.
func (e *RateLimitError) Is(target error) bool {
	var rateLimitErr *RateLimitError
	return errors.As(target, &rateLimitErr)
}

// Ensure RateLimitPlugin implements Plugin interface
var _ plugin.Plugin = (*RateLimitPlugin)(nil)
