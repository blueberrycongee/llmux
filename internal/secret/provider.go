package secret

import "context"

// Provider defines the interface for retrieving secrets from various sources.
type Provider interface {
	// Get retrieves the secret value for the given path.
	// path examples: "env://OPENAI_API_KEY", "vault://secret/data/openai"
	Get(ctx context.Context, path string) (string, error)

	// Close releases any resources held by the provider.
	Close() error
}
