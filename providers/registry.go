// Package providers provides a unified registry for all LLMux provider implementations.
// It allows automatic provider creation from configuration.
package providers

import (
	"fmt"
	"sync"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/anthropic"
	"github.com/blueberrycongee/llmux/providers/deepseek"
	"github.com/blueberrycongee/llmux/providers/fireworks"
	"github.com/blueberrycongee/llmux/providers/groq"
	"github.com/blueberrycongee/llmux/providers/mistral"
	"github.com/blueberrycongee/llmux/providers/openai"
	"github.com/blueberrycongee/llmux/providers/openrouter"
	"github.com/blueberrycongee/llmux/providers/perplexity"
	"github.com/blueberrycongee/llmux/providers/together"
)

var (
	registry     = make(map[string]provider.Factory)
	registryOnce sync.Once
	registryMu   sync.RWMutex
)

// Register registers a provider factory with the given type name.
func Register(providerType string, factory provider.Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[providerType] = factory
}

// Get returns the factory for the given provider type.
func Get(providerType string) (provider.Factory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[providerType]
	return f, ok
}

// Create creates a provider instance from configuration.
func Create(cfg provider.Config) (provider.Provider, error) {
	registryMu.RLock()
	factory, ok := registry[cfg.Type]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s (available: %v)", cfg.Type, List())
	}

	return factory(cfg)
}

// List returns all registered provider type names.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// RegisterBuiltins registers all built-in provider factories.
// This is called automatically on first use.
func RegisterBuiltins() {
	registryOnce.Do(func() {
		Register("openai", openai.NewFromConfig)
		Register("anthropic", anthropic.NewFromConfig)
		Register("groq", groq.NewFromConfig)
		Register("deepseek", deepseek.NewFromConfig)
		Register("together", together.NewFromConfig)
		Register("fireworks", fireworks.NewFromConfig)
		Register("mistral", mistral.NewFromConfig)
		Register("perplexity", perplexity.NewFromConfig)
		Register("openrouter", openrouter.NewFromConfig)
	})
}

func init() {
	RegisterBuiltins()
}
