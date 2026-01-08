// Package env implements a secret provider that reads from environment variables.
package env

import (
	"context"
	"fmt"
	"os"
)

// Provider implements the secret.Provider interface for environment variables.
type Provider struct{}

// New creates a new Env provider.
func New() *Provider {
	return &Provider{}
}

// Get retrieves the value of the environment variable specified by path.
func (p *Provider) Get(ctx context.Context, path string) (string, error) {
	val, ok := os.LookupEnv(path)
	if !ok {
		return "", fmt.Errorf("environment variable %q not set", path)
	}
	return val, nil
}

// Close is a no-op for the Env provider.
func (p *Provider) Close() error {
	return nil
}
