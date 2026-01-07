// Package builtin provides built-in plugin implementations for common use cases.
//
// Available plugins:
//   - LoggingPlugin: Request/response logging with configurable levels
//   - RateLimitPlugin: Request rate limiting per client/API key
//   - MetricsPlugin: Request metrics collection
//   - CachePlugin: Response caching with TTL
//
// Example usage:
//
//	pipeline := plugin.NewPipeline(logger, plugin.DefaultPipelineConfig())
//	pipeline.Register(builtin.NewLoggingPlugin(logger))
//	pipeline.Register(builtin.NewRateLimitPlugin(100, 50))
package builtin
