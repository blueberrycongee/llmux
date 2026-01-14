package openai

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
)

func TestNewFromConfig_RejectsPrivateBaseURLByDefault(t *testing.T) {
	_, err := NewFromConfig(provider.Config{
		APIKey:  "test",
		BaseURL: "http://127.0.0.1:1234/v1",
	})
	require.Error(t, err)
}

func TestNewFromConfig_AllowsPrivateBaseURLWhenExplicit(t *testing.T) {
	_, err := NewFromConfig(provider.Config{
		APIKey:              "test",
		BaseURL:             "http://127.0.0.1:1234/v1",
		AllowPrivateBaseURL: true,
	})
	require.NoError(t, err)
}

func TestNewFromConfig_RejectsBaseURLWithUserInfo(t *testing.T) {
	_, err := NewFromConfig(provider.Config{
		APIKey:  "test",
		BaseURL: "https://user:pass@example.com/v1",
	})
	require.Error(t, err)
}

func TestNewFromConfig_RejectsBaseURLWithQuery(t *testing.T) {
	_, err := NewFromConfig(provider.Config{
		APIKey:  "test",
		BaseURL: "https://example.com/v1?x=y",
	})
	require.Error(t, err)
}
