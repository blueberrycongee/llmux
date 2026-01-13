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
