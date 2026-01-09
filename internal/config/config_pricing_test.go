package config_test

import (
	"os"
	"testing"

	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadPricingFileConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  port: 8080
providers:
  - name: openai
    type: openai
    api_key: sk-test
    models: ["gpt-4"]
pricing_file: "/path/to/pricing.json"
`
	tmpfile, err := os.CreateTemp("", "config_*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(configContent))
	assert.NoError(t, err)
	tmpfile.Close()

	// Load config
	cfg, err := config.LoadFromFile(tmpfile.Name())
	assert.NoError(t, err)

	// Verify PricingFile
	assert.Equal(t, "/path/to/pricing.json", cfg.PricingFile)
}
