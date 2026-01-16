package llmux

import (
	"context"
	"log/slog"
	"time"

	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// RateLimitKeyStrategy defines how to derive the rate limit key.
type RateLimitKeyStrategy string

const (
	// RateLimitKeyByAPIKey uses the API key as the rate limit key.
	RateLimitKeyByAPIKey RateLimitKeyStrategy = "api_key" // #nosec G101 -- identifier value, not a credential.
	// RateLimitKeyByUser uses the user ID as the rate limit key.
	RateLimitKeyByUser RateLimitKeyStrategy = "user"
	// RateLimitKeyByModel uses the model name as the rate limit key.
	RateLimitKeyByModel RateLimitKeyStrategy = "model"
	// RateLimitKeyByAPIKeyAndModel uses both API key and model as the rate limit key.
	RateLimitKeyByAPIKeyAndModel RateLimitKeyStrategy = "api_key_model" // #nosec G101 -- identifier value, not a credential.
)

// RateLimiterConfig holds configuration for rate limiting.
type RateLimiterConfig struct {
	// Enabled indicates whether rate limiting is enabled.
	Enabled bool
	// RPMLimit is the requests per minute limit.
	RPMLimit int64
	// TPMLimit is the tokens per minute limit.
	TPMLimit int64
	// WindowSize is the sliding window duration (default: 1 minute).
	WindowSize time.Duration
	// KeyStrategy defines how to derive the rate limit key.
	KeyStrategy RateLimitKeyStrategy
	// FailOpen allows requests when the rate limiter backend fails.
	FailOpen bool
}

// FallbackReporter receives fallback outcomes for observability.
type FallbackReporter func(ctx context.Context, originalModel, fallbackModel string, err error, success bool)

// ClientConfig holds all configuration for the LLMux client.
type ClientConfig struct {
	// Providers configuration
	Providers []ProviderConfig

	// Custom provider instances (for advanced use)
	ProviderInstances []providerInstance

	// Routing
	RouterStrategy   Strategy
	Router           Router // Custom router (overrides RouterStrategy)
	FallbackEnabled  bool
	RetryCount       int
	RetryBackoff     time.Duration
	RetryMaxBackoff  time.Duration
	RetryJitter      float64
	CooldownPeriod   time.Duration
	EWMAAlpha        float64
	DefaultProvider  string
	FallbackReporter FallbackReporter

	// Distributed Routing Stats (for multi-instance deployments)
	StatsStore router.StatsStore

	// Distributed Round Robin counters (for multi-instance deployments)
	RoundRobinStore router.RoundRobinStore

	// Caching
	CacheEnabled   bool
	Cache          Cache // Custom cache implementation
	CacheTTL       time.Duration
	CacheTypeLabel string

	// HTTP
	Timeout time.Duration

	// Logging
	Logger *slog.Logger

	// Plugins
	Plugins      []plugin.Plugin
	PluginConfig *plugin.PipelineConfig

	// Pricing
	PricingFile string

	// Stream recovery
	StreamRecoveryMode StreamRecoveryMode
	// StreamRecoveryMaxAccumulatedBytes caps the in-memory accumulated stream content used for recovery.
	// Set to 0 to disable the cap (not recommended).
	StreamRecoveryMaxAccumulatedBytes int

	// Observability
	OTelMetricsConfig observability.OTelMetricsConfig

	// Rate Limiting (Distributed)
	RateLimiter       resilience.DistributedLimiter
	RateLimiterConfig RateLimiterConfig
}

// providerInstance holds a pre-configured provider with its models.
type providerInstance struct {
	Name     string
	Provider Provider
	Models   []string
}

// Option is a function that configures the Client.
type Option func(*ClientConfig)

// defaultConfig returns sensible defaults.
func defaultConfig() *ClientConfig {
	return &ClientConfig{
		RouterStrategy:                    StrategySimpleShuffle,
		FallbackEnabled:                   true,
		RetryCount:                        3,
		RetryBackoff:                      time.Second,
		RetryMaxBackoff:                   5 * time.Second,
		RetryJitter:                       0.2,
		CooldownPeriod:                    60 * time.Second,
		EWMAAlpha:                         0.1,
		CacheEnabled:                      false,
		CacheTTL:                          time.Hour,
		CacheTypeLabel:                    "unknown",
		Timeout:                           30 * time.Second,
		Logger:                            slog.Default(),
		StreamRecoveryMode:                StreamRecoveryRetry,
		StreamRecoveryMaxAccumulatedBytes: 1 << 20, // 1MiB
	}
}

// StreamRecoveryMode controls how stream recovery behaves after a mid-stream failure.
type StreamRecoveryMode string

const (
	StreamRecoveryOff    StreamRecoveryMode = "off"
	StreamRecoveryAppend StreamRecoveryMode = "append"
	StreamRecoveryRetry  StreamRecoveryMode = "retry"
)

// WithProvider adds a provider configuration.
// The provider will be created automatically based on the Type field.
//
// Example:
//
//	llmux.WithProvider(llmux.ProviderConfig{
//	    Name:   "openai",
//	    Type:   "openai",
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	    Models: []string{"gpt-4o", "gpt-4o-mini"},
//	})
func WithProvider(cfg ProviderConfig) Option {
	return func(c *ClientConfig) {
		c.Providers = append(c.Providers, cfg)
	}
}

// WithProviderInstance adds a pre-configured provider instance.
// Use this when you need custom provider configuration beyond what ProviderConfig offers.
//
// Example:
//
//	openaiProvider := openai.New(
//	    openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
//	    openai.WithBaseURL("https://custom-endpoint.com/v1"),
//	)
//	llmux.WithProviderInstance("my-openai", openaiProvider, []string{"gpt-4o"})
func WithProviderInstance(name string, prov Provider, models []string) Option {
	return func(c *ClientConfig) {
		c.ProviderInstances = append(c.ProviderInstances, providerInstance{
			Name:     name,
			Provider: prov,
			Models:   models,
		})
	}
}

// WithRouter sets a custom router implementation.
// This overrides the RouterStrategy option.
//
// Example:
//
//	customRouter := myrouter.New(...)
//	llmux.WithRouter(customRouter)
func WithRouter(r Router) Option {
	return func(c *ClientConfig) {
		c.Router = r
	}
}

// WithRouterStrategy sets the routing strategy.
// Available strategies:
//   - StrategySimpleShuffle: Random selection with optional weights
//   - StrategyLowestLatency: Select deployment with lowest latency
//   - StrategyLeastBusy: Select deployment with fewest active requests
//   - StrategyLowestTPMRPM: Select deployment with lowest token/request usage
//   - StrategyLowestCost: Select deployment with lowest cost
//   - StrategyTagBased: Filter deployments by tags
func WithRouterStrategy(strategy Strategy) Option {
	return func(c *ClientConfig) {
		c.RouterStrategy = strategy
	}
}

// WithDefaultProvider prefers a provider when multiple deployments are available.
func WithDefaultProvider(provider string) Option {
	return func(c *ClientConfig) {
		c.DefaultProvider = provider
	}
}

// WithFallback enables/disables fallback on failure.
// When enabled, failed requests will be retried on different deployments.
func WithFallback(enabled bool) Option {
	return func(c *ClientConfig) {
		c.FallbackEnabled = enabled
	}
}

// WithRetry configures retry behavior.
// count: number of retry attempts (0 = no retries)
// backoff: initial backoff duration (exponential backoff is applied)
func WithRetry(count int, backoff time.Duration) Option {
	return func(c *ClientConfig) {
		c.RetryCount = count
		c.RetryBackoff = backoff
	}
}

// WithRetryMaxBackoff sets the maximum backoff duration for retries.
// Use 0 to disable the cap.
func WithRetryMaxBackoff(d time.Duration) Option {
	return func(c *ClientConfig) {
		c.RetryMaxBackoff = d
	}
}

// WithRetryJitter sets the jitter ratio for retries (0.0 - 1.0).
func WithRetryJitter(jitter float64) Option {
	return func(c *ClientConfig) {
		c.RetryJitter = jitter
	}
}

// WithFallbackReporter records fallback outcomes.
func WithFallbackReporter(reporter FallbackReporter) Option {
	return func(c *ClientConfig) {
		c.FallbackReporter = reporter
	}
}

// WithCooldown sets the cooldown period for failed deployments.
// Deployments that fail will be excluded from routing for this duration.
func WithCooldown(d time.Duration) Option {
	return func(c *ClientConfig) {
		c.CooldownPeriod = d
	}
}

// WithEWMAAlpha sets the smoothing factor for EWMA calculations.
// alpha should be between 0 and 1. A higher alpha discounts older observations faster.
func WithEWMAAlpha(alpha float64) Option {
	return func(c *ClientConfig) {
		c.EWMAAlpha = alpha
	}
}

// WithCache enables caching with the given implementation.
//
// Example:
//
//	redisCache, _ := redis.New(redis.WithAddr("localhost:6379"))
//	llmux.WithCache(redisCache)
func WithCache(cache Cache) Option {
	return func(c *ClientConfig) {
		c.CacheEnabled = true
		c.Cache = cache
	}
}

// WithCacheTTL sets the default cache TTL.
// This is used when no TTL is specified in the cache control.
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *ClientConfig) {
		c.CacheTTL = ttl
	}
}

// WithCacheTypeLabel sets the cache type label for metrics.
func WithCacheTypeLabel(label string) Option {
	return func(c *ClientConfig) {
		c.CacheTypeLabel = label
	}
}

// WithTimeout sets the HTTP request timeout.
// This applies to all provider API calls.
func WithTimeout(d time.Duration) Option {
	return func(c *ClientConfig) {
		c.Timeout = d
	}
}

// WithLogger sets the logger for the client.
// The logger is used for debug, info, and error messages.
func WithLogger(logger *slog.Logger) Option {
	return func(c *ClientConfig) {
		c.Logger = logger
	}
}

// WithPlugin adds a plugin to the client.
// Plugins are executed in the order of their priority.
func WithPlugin(p plugin.Plugin) Option {
	return func(c *ClientConfig) {
		c.Plugins = append(c.Plugins, p)
	}
}

// WithPluginConfig sets the plugin pipeline configuration.
func WithPluginConfig(config plugin.PipelineConfig) Option {
	return func(c *ClientConfig) {
		c.PluginConfig = &config
	}
}

// WithPricingFile sets the path to the custom pricing JSON file.
func WithPricingFile(path string) Option {
	return func(c *ClientConfig) {
		c.PricingFile = path
	}
}

// WithOTelMetrics configures OpenTelemetry metrics.
func WithOTelMetrics(config observability.OTelMetricsConfig) Option {
	return func(c *ClientConfig) {
		c.OTelMetricsConfig = config
	}
}

// WithRateLimiter sets a custom distributed rate limiter implementation.
// This enables distributed rate limiting across multiple LLMux instances.
//
// Example:
//
//	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	limiter := resilience.NewRedisLimiter(redisClient)
//	llmux.WithRateLimiter(limiter)
func WithRateLimiter(limiter resilience.DistributedLimiter) Option {
	return func(c *ClientConfig) {
		c.RateLimiter = limiter
	}
}

// WithRateLimiterConfig sets the rate limiter configuration.
// This configures the rate limits (RPM/TPM) and key strategy.
//
// Example:
//
//	llmux.WithRateLimiterConfig(llmux.RateLimiterConfig{
//	    Enabled:     true,
//	    RPMLimit:    100,
//	    TPMLimit:    10000,
//	    WindowSize:  time.Minute,
//	    KeyStrategy: llmux.RateLimitKeyByAPIKey,
//	})
func WithRateLimiterConfig(config RateLimiterConfig) Option {
	return func(c *ClientConfig) {
		c.RateLimiterConfig = config
	}
}

// WithStatsStore sets a distributed stats store for routing decisions.
// This enables multi-instance deployments to share routing statistics
// (latency, active requests, cooldowns), making load-balanced strategies
// like least-busy and lowest-latency work correctly across a cluster.
//
// Example:
//
//	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	statsStore := router.NewRedisStatsStore(redisClient)
//	llmux.WithStatsStore(statsStore)
func WithStatsStore(store router.StatsStore) Option {
	return func(c *ClientConfig) {
		c.StatsStore = store
	}
}

// WithRoundRobinStore sets a distributed round-robin store for consistent RR across instances.
func WithRoundRobinStore(store router.RoundRobinStore) Option {
	return func(c *ClientConfig) {
		c.RoundRobinStore = store
	}
}

// WithStreamRecoveryMode configures how streaming recovery behaves after a mid-stream failure.
func WithStreamRecoveryMode(mode StreamRecoveryMode) Option {
	return func(c *ClientConfig) {
		c.StreamRecoveryMode = mode
	}
}

// WithStreamRecoveryMaxAccumulatedBytes caps the accumulated stream content retained in-memory for recovery.
// A value of 0 disables the cap.
func WithStreamRecoveryMaxAccumulatedBytes(maxBytes int) Option {
	return func(c *ClientConfig) {
		c.StreamRecoveryMaxAccumulatedBytes = maxBytes
	}
}
