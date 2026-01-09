package pricing

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)

	// Test with a known model from defaults.json
	// Note: This assumes defaults.json is correctly embedded and loaded
	price, ok := r.GetPrice("gpt-4o", "openai")
	assert.True(t, ok, "gpt-4o should be found in defaults")
	assert.Equal(t, 0.000005, price.InputCostPerToken)
	assert.Equal(t, 0.000015, price.OutputCostPerToken)
}

func TestRegistry_GetPrice(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		name       string
		model      string
		provider   string
		wantFound  bool
		wantInput  float64
		wantOutput float64
	}{
		{
			name:       "Exact match",
			model:      "gpt-4o",
			provider:   "openai",
			wantFound:  true,
			wantInput:  0.000005,
			wantOutput: 0.000015,
		},
		{
			name:       "Provider/Model match",
			model:      "gpt-4o",
			provider:   "azure",
			wantFound:  true,
			wantInput:  0.000005,
			wantOutput: 0.000015,
		},
		{
			name:      "Unknown model",
			model:     "unknown-model",
			provider:  "openai",
			wantFound: false,
		},
		{
			name:     "Model only match (fallback)",
			model:    "gpt-4o",
			provider: "some-other-provider",
			// Should fall back to "gpt-4o" key if "some-other-provider/gpt-4o" doesn't exist
			// This assumes "gpt-4o" is in the registry (which it is in defaults.json)
			wantFound:  true,
			wantInput:  0.000005,
			wantOutput: 0.000015,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, ok := r.GetPrice(tt.model, tt.provider)
			assert.Equal(t, tt.wantFound, ok)
			if tt.wantFound {
				assert.Equal(t, tt.wantInput, price.InputCostPerToken)
				assert.Equal(t, tt.wantOutput, price.OutputCostPerToken)
			}
		})
	}
}

func TestRegistry_Load(t *testing.T) {
	// Create a temporary pricing file
	content := `{
		"custom-model": {
			"litellm_provider": "custom",
			"input_cost_per_token": 0.1,
			"output_cost_per_token": 0.2
		},
		"gpt-4o": {
			"litellm_provider": "openai",
			"input_cost_per_token": 0.99,
			"output_cost_per_token": 0.99
		}
	}`
	tmpFile, err := os.CreateTemp("", "pricing_*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	r := NewRegistry()
	err = r.Load(tmpFile.Name())
	require.NoError(t, err)

	// Test new model
	price, ok := r.GetPrice("custom-model", "custom")
	assert.True(t, ok)
	assert.Equal(t, 0.1, price.InputCostPerToken)

	// Test override
	price, ok = r.GetPrice("gpt-4o", "openai")
	assert.True(t, ok)
	assert.Equal(t, 0.99, price.InputCostPerToken)
}
