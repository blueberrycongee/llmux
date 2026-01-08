package pricing

import (
	"testing"
)

func TestCalculator_Calculate(t *testing.T) {
	calc := NewCalculator(nil) // Use default pricing

	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		want         float64
	}{
		{
			name:         "gpt-4o exact match",
			model:        "gpt-4o",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0.005*1 + 0.015*1, // 0.020
		},
		{
			name:         "gpt-4-turbo wildcard match",
			model:        "gpt-4-turbo-preview",
			inputTokens:  1000,
			outputTokens: 500,
			want:         0.01*1 + 0.03*0.5, // 0.025
		},
		{
			name:         "claude-3-5-sonnet wildcard match",
			model:        "claude-3-5-sonnet-20240620",
			inputTokens:  2000,
			outputTokens: 1000,
			want:         0.003*2 + 0.015*1, // 0.021
		},
		{
			name:         "gemini-1.5-flash wildcard match",
			model:        "gemini-1.5-flash-001",
			inputTokens:  10000,
			outputTokens: 5000,
			want:         0.000075*10 + 0.0003*5, // 0.0015 + 0.0015 = 0.003
		},
		{
			name:         "unknown model returns zero",
			model:        "unknown-model",
			inputTokens:  1000,
			outputTokens: 1000,
			want:         0,
		},
		{
			name:         "zero tokens",
			model:        "gpt-4o",
			inputTokens:  0,
			outputTokens: 0,
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.Calculate(tt.model, tt.inputTokens, tt.outputTokens)
			// Allow small floating point differences
			if diff := got - tt.want; diff < -0.0001 || diff > 0.0001 {
				t.Errorf("Calculate() = %v, want %v (diff: %v)", got, tt.want, diff)
			}
		})
	}
}

func TestCalculator_FindPricing(t *testing.T) {
	calc := NewCalculator(nil)

	tests := []struct {
		name      string
		model     string
		wantFound bool
		wantModel string // The pattern that should match
	}{
		{
			name:      "exact match gpt-4o",
			model:     "gpt-4o",
			wantFound: true,
			wantModel: "gpt-4o",
		},
		{
			name:      "wildcard match gpt-4-turbo-preview",
			model:     "gpt-4-turbo-preview",
			wantFound: true,
			wantModel: "gpt-4-turbo*",
		},
		{
			name:      "wildcard match claude-3-opus-20240229",
			model:     "claude-3-opus-20240229",
			wantFound: true,
			wantModel: "claude-3-opus*",
		},
		{
			name:      "unknown model",
			model:     "completely-unknown",
			wantFound: false,
		},
		{
			name:      "case insensitive match",
			model:     "GPT-4O",
			wantFound: true,
			wantModel: "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, found := calc.GetPricing(tt.model)
			if found != tt.wantFound {
				t.Errorf("GetPricing() found = %v, want %v", found, tt.wantFound)
			}
			if found && pricing.Model != tt.wantModel {
				t.Errorf("GetPricing() model = %v, want %v", pricing.Model, tt.wantModel)
			}
		})
	}
}

func TestCalculator_AddPricing(t *testing.T) {
	calc := NewCalculator(nil)

	// Add custom pricing
	customPricing := ModelPricing{
		Model:           "custom-model",
		InputCostPer1K:  0.001,
		OutputCostPer1K: 0.002,
	}
	calc.AddPricing(customPricing)

	// Test calculation with custom model
	cost := calc.Calculate("custom-model", 1000, 1000)
	want := 0.001 + 0.002 // 0.003
	if diff := cost - want; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Calculate() with custom pricing = %v, want %v", cost, want)
	}

	// Test overwriting existing pricing
	updatedPricing := ModelPricing{
		Model:           "gpt-4o",
		InputCostPer1K:  0.999,
		OutputCostPer1K: 0.999,
	}
	calc.AddPricing(updatedPricing)

	cost = calc.Calculate("gpt-4o", 1000, 1000)
	want = 0.999 + 0.999 // 1.998
	if diff := cost - want; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Calculate() with updated pricing = %v, want %v", cost, want)
	}
}

func BenchmarkCalculate(b *testing.B) {
	calc := NewCalculator(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calc.Calculate("gpt-4o", 1000, 1000)
	}
}

func BenchmarkCalculateWildcard(b *testing.B) {
	calc := NewCalculator(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calc.Calculate("gpt-4-turbo-preview", 1000, 1000)
	}
}
