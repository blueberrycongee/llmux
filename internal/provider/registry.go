package provider

import (
	"fmt"
	"sync"
)

// Registry manages provider factories and instances.
// It allows dynamic registration of new provider types.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]ProviderFactory
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
		providers: make(map[string]Provider),
	}
}

// RegisterFactory registers a factory function for a provider type.
// This should be called during initialization to register all supported providers.
func (r *Registry) RegisterFactory(providerType string, factory ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[providerType] = factory
}

// CreateProvider creates a new provider instance using the registered factory.
func (r *Registry) CreateProvider(cfg ProviderConfig) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[cfg.Type]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	provider, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("create provider %s: %w", cfg.Name, err)
	}

	r.mu.Lock()
	r.providers[cfg.Name] = provider
	r.mu.Unlock()

	return provider, nil
}

// GetProvider returns a provider by name.
func (r *Registry) GetProvider(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetProviderForModel finds a provider that supports the given model.
func (r *Registry) GetProviderForModel(model string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if p.SupportsModel(model) {
			return p, true
		}
	}
	return nil, false
}

// ListProviders returns all registered provider names.
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
