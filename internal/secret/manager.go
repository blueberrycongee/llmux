package secret

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Manager handles multiple secret providers and routes requests based on URI schemes.
type Manager struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewManager creates a new secret manager.
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider for a specific scheme (e.g., "vault", "env").
func (m *Manager) Register(scheme string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[scheme] = provider
}

// Get retrieves a secret by parsing the path scheme.
// If no scheme is present, it returns the path as-is (static secret support).
func (m *Manager) Get(ctx context.Context, path string) (string, error) {
	parts := strings.SplitN(path, "://", 2)
	if len(parts) != 2 {
		// No scheme, return as static value
		return path, nil
	}

	scheme := parts[0]
	secretPath := parts[1]

	m.mu.RLock()
	provider, ok := m.providers[scheme]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("no secret provider registered for scheme: %s", scheme)
	}

	return provider.Get(ctx, secretPath)
}

// Close closes all registered providers.
func (m *Manager) Close() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []string
	for scheme, p := range m.providers {
		if err := p.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", scheme, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close providers: %s", strings.Join(errs, "; "))
	}
	return nil
}
