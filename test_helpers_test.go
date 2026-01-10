package llmux

import (
	"encoding/json"
	"os"
	"testing"
)

func writePricingFile(t *testing.T, prices map[string]map[string]interface{}) string {
	t.Helper()

	file, err := os.CreateTemp("", "pricing_*.json")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}

	if err := json.NewEncoder(file).Encode(prices); err != nil {
		file.Close()
		os.Remove(file.Name())
		t.Fatalf("encode pricing file error = %v", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(file.Name())
		t.Fatalf("close pricing file error = %v", err)
	}

	t.Cleanup(func() {
		_ = os.Remove(file.Name())
	})

	return file.Name()
}

func withTestPricing(t *testing.T, models ...string) Option {
	t.Helper()

	prices := make(map[string]map[string]interface{}, len(models))
	for _, model := range models {
		prices[model] = map[string]interface{}{
			"input_cost_per_token":  0.00001,
			"output_cost_per_token": 0.00002,
		}
	}

	return WithPricingFile(writePricingFile(t, prices))
}
