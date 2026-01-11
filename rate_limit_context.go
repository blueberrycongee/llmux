package llmux

import "context"

type rateLimitAPIKeyContextKey struct{}

// WithRateLimitAPIKey stores an API key identifier in the context for rate limiting.
// This is only used to derive the rate limit key and is never sent to providers.
func WithRateLimitAPIKey(ctx context.Context, apiKey string) context.Context {
	if apiKey == "" {
		return ctx
	}
	return context.WithValue(ctx, rateLimitAPIKeyContextKey{}, apiKey)
}

func rateLimitAPIKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if apiKey, ok := ctx.Value(rateLimitAPIKeyContextKey{}).(string); ok {
		return apiKey
	}
	return ""
}
