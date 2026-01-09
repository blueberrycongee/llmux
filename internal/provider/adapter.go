// Package provider defines the interface for LLM provider adapters.
// This file provides adapters to bridge internal types with pkg/provider.
package provider

import (
	"time"

	pkgprovider "github.com/blueberrycongee/llmux/pkg/provider"
)

// ToPkgConfig converts internal ProviderConfig to pkg/provider.Config.
// This enables internal code to use the unified providers/ package implementations.
func ToPkgConfig(cfg ProviderConfig) pkgprovider.Config {
	var ts pkgprovider.TokenSource
	if cfg.TokenSource != nil {
		ts = &tokenSourceAdapter{cfg.TokenSource}
	}
	return pkgprovider.Config{
		Name:          cfg.Name,
		Type:          cfg.Type,
		APIKey:        cfg.APIKey,
		TokenSource:   ts,
		BaseURL:       cfg.BaseURL,
		Models:        cfg.Models,
		MaxConcurrent: cfg.MaxConcurrent,
		Timeout:       time.Duration(cfg.TimeoutSec) * time.Second,
		Headers:       cfg.Headers,
	}
}

// tokenSourceAdapter wraps internal TokenSource to implement pkg/provider.TokenSource
type tokenSourceAdapter struct {
	ts TokenSource
}

func (a *tokenSourceAdapter) Token() (string, error) {
	return a.ts.Token()
}

// FromPkgProvider wraps a pkg/provider.Provider to internal Provider interface.
// Since both interfaces are identical, this is a simple type assertion.
func FromPkgProvider(p pkgprovider.Provider) Provider {
	return p
}

// AdaptFactory wraps a pkg/provider.Factory to return an internal ProviderFactory.
// This allows using providers/ package factories with internal/provider types.
func AdaptFactory(factory pkgprovider.Factory) ProviderFactory {
	return func(cfg ProviderConfig) (Provider, error) {
		pkgCfg := ToPkgConfig(cfg)
		p, err := factory(pkgCfg)
		if err != nil {
			return nil, err
		}
		return FromPkgProvider(p), nil
	}
}
