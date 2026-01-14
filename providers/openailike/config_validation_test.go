package openailike

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
)

func TestNewFromConfig_RequiresBaseURL(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "",
	}

	_, err := NewFromConfig(info, provider.Config{
		APIKey:  "test",
		BaseURL: "",
	})
	require.Error(t, err)
}

func TestNewFromConfig_RejectsPrivateBaseURLByDefault(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "http://127.0.0.1:1234/v1",
	}

	_, err := NewFromConfig(info, provider.Config{
		APIKey:  "test",
		BaseURL: "",
	})
	require.Error(t, err)
}

func TestNewFromConfig_AllowsPrivateBaseURLWhenExplicit(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "http://127.0.0.1:1234/v1",
	}

	_, err := NewFromConfig(info, provider.Config{
		APIKey:              "test",
		AllowPrivateBaseURL: true,
	})
	require.NoError(t, err)
}

func TestNewFromConfig_RejectsBaseURLWithUserInfo(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "",
	}

	_, err := NewFromConfig(info, provider.Config{
		APIKey:  "test",
		BaseURL: "https://user:pass@example.com/v1",
	})
	require.Error(t, err)
}

func TestNewFromConfig_RejectsBaseURLWithQuery(t *testing.T) {
	info := Info{
		Name:           "test-provider",
		DefaultBaseURL: "",
	}

	_, err := NewFromConfig(info, provider.Config{
		APIKey:  "test",
		BaseURL: "https://example.com/v1?x=y",
	})
	require.Error(t, err)
}
