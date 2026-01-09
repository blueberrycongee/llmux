package llmux

import (
	"log/slog"
	"time"

	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/internal/resilience"
)

// ClientConfig holds all configuration for the LLMux client.
type ClientConfig struct {
	// Providers configuration
	Providers []ProviderConfig

	// Custom provider instances (for advanced use)
	ProviderInstances []providerInstance

	// Routing
	RouterStrategy  Strategy
	Router          Router // Custom router (overrides RouterStrategy)
	FallbackEnabled bool
	RetryCount      int
	RetryBackoff    time.Duration
	CooldownPeriod  time.Duration

	// Caching
	CacheEnabled bool
	Cache        Cache // Custom cache implementation
	CacheTTL     time.Duration

	// HTTP
	Timeout time.Duration

	// Logging
	Logger *slog.Logger

	// Plugins
	Plugins      []plugin.Plugin
	PluginConfig *plugin.PipelineConfig

	// Pricing
	PricingFile string

	// Observability
	OTelMetricsConfig observability.OTelMetricsConfig

	// Rate Limiting
	RateLimiter resilience.DistributedLimiter
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
		RouterStrategy:  StrategySimpleShuffle,
		FallbackEnabled: true,
		RetryCount:      3,
		RetryBackoff:    time.Second,
		CooldownPeriod:  60 * time.Second,
		CacheEnabled:    false,
		CacheTTL:        time.Hour,
		Timeout:         30 * time.Second,
		Logger:          slog.Default(),
	}
}

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

// WithCooldown sets the cooldown period for failed deployments.
// Deployments that fail will be excluded from routing for this duration.
func WithCooldown(d time.Duration) Option {
	return func(c *ClientConfig) {
		c.CooldownPeriod = d
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

// WithRateLimiter sets the distributed rate limiter.
func WithRateLimiter(limiter resilience.DistributedLimiter) Option {
	return func(c *ClientConfig) {
		c.RateLimiter = limiter
	}
}
