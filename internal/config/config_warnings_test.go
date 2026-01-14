package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWarnings_CacheEnabledWithoutAuth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Cache.Enabled = true
	cfg.Auth.Enabled = false

	warnings := cfg.Warnings()
	require.NotEmpty(t, warnings)

	var found bool
	for _, w := range warnings {
		if w.Code == WarningCacheWithoutAuth {
			found = true
			break
		}
	}
	require.True(t, found, "expected %q warning", WarningCacheWithoutAuth)
}

func TestWarnings_NoCacheOrAuthEnabled(t *testing.T) {
	t.Run("cache disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Cache.Enabled = false
		cfg.Auth.Enabled = false
		require.Empty(t, cfg.Warnings())
	})

	t.Run("auth enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Cache.Enabled = true
		cfg.Auth.Enabled = true
		require.Empty(t, cfg.Warnings())
	})
}
