// Package secret provides secret management interfaces and implementations.
package secret

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
)

// CachedProvider decorates a Provider with in-memory caching.
type CachedProvider struct {
	inner Provider
	cache *cache.Cache
}

// NewCachedProvider creates a new cached provider.
// defaultTTL is the expiration time for cached secrets.
func NewCachedProvider(inner Provider, defaultTTL time.Duration) *CachedProvider {
	return &CachedProvider{
		inner: inner,
		cache: cache.New(defaultTTL, defaultTTL*2),
	}
}

// Get retrieves a secret from the cache or delegates to the inner provider.
func (p *CachedProvider) Get(ctx context.Context, path string) (string, error) {
	if val, found := p.cache.Get(path); found {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}

	val, err := p.inner.Get(ctx, path)
	if err != nil {
		return "", err
	}

	p.cache.Set(path, val, cache.DefaultExpiration)
	return val, nil
}

// Close closes the inner provider.
func (p *CachedProvider) Close() error {
	return p.inner.Close()
}
