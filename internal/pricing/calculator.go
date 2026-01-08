package pricing

import (
	"strings"
)

// ModelPricing defines the pricing for a model.
type ModelPricing struct {
	Model           string  // 模型名称 (支持通配符，如 "gpt-4*")
	InputCostPer1K  float64 // 每 1000 个输入 token 的价格 (美元)
	OutputCostPer1K float64 // 每 1000 个输出 token 的价格 (美元)
}

// DefaultPricing contains default pricing for common models.
// Prices are in USD per 1000 tokens, as of 2024.
var DefaultPricing = []ModelPricing{
	// OpenAI GPT-4 models
	{Model: "gpt-4o", InputCostPer1K: 0.005, OutputCostPer1K: 0.015},
	{Model: "gpt-4o-mini", InputCostPer1K: 0.00015, OutputCostPer1K: 0.0006},
	{Model: "gpt-4-turbo*", InputCostPer1K: 0.01, OutputCostPer1K: 0.03},
	{Model: "gpt-4*", InputCostPer1K: 0.03, OutputCostPer1K: 0.06}, // Fallback for gpt-4
	{Model: "gpt-3.5-turbo", InputCostPer1K: 0.0005, OutputCostPer1K: 0.0015},

	// Anthropic Claude models
	{Model: "claude-3-5-sonnet*", InputCostPer1K: 0.003, OutputCostPer1K: 0.015},
	{Model: "claude-3-opus*", InputCostPer1K: 0.015, OutputCostPer1K: 0.075},
	{Model: "claude-3-sonnet*", InputCostPer1K: 0.003, OutputCostPer1K: 0.015},
	{Model: "claude-3-haiku*", InputCostPer1K: 0.00025, OutputCostPer1K: 0.00125},
	{Model: "claude-2*", InputCostPer1K: 0.008, OutputCostPer1K: 0.024},

	// Google Gemini models
	{Model: "gemini-1.5-pro*", InputCostPer1K: 0.00125, OutputCostPer1K: 0.005},
	{Model: "gemini-1.5-flash*", InputCostPer1K: 0.000075, OutputCostPer1K: 0.0003},
	{Model: "gemini-pro*", InputCostPer1K: 0.0005, OutputCostPer1K: 0.0015},

	// DeepSeek models
	{Model: "deepseek-chat", InputCostPer1K: 0.00014, OutputCostPer1K: 0.00028},
	{Model: "deepseek-coder", InputCostPer1K: 0.00014, OutputCostPer1K: 0.00028},

	// Meta Llama models (via providers)
	{Model: "llama-3*", InputCostPer1K: 0.0002, OutputCostPer1K: 0.0002},
	{Model: "llama-2*", InputCostPer1K: 0.0002, OutputCostPer1K: 0.0002},

	// Mistral models
	{Model: "mistral-large*", InputCostPer1K: 0.004, OutputCostPer1K: 0.012},
	{Model: "mistral-medium*", InputCostPer1K: 0.0027, OutputCostPer1K: 0.0081},
	{Model: "mistral-small*", InputCostPer1K: 0.001, OutputCostPer1K: 0.003},
	{Model: "mixtral-8x7b*", InputCostPer1K: 0.0007, OutputCostPer1K: 0.0007},

	// Cohere models
	{Model: "command-r-plus*", InputCostPer1K: 0.003, OutputCostPer1K: 0.015},
	{Model: "command-r*", InputCostPer1K: 0.0005, OutputCostPer1K: 0.0015},
	{Model: "command*", InputCostPer1K: 0.001, OutputCostPer1K: 0.002},
}

// Calculator calculates the cost of API usage.
type Calculator struct {
	pricing map[string]ModelPricing
}

// NewCalculator creates a new pricing calculator.
// If no pricing is provided, uses DefaultPricing.
func NewCalculator(pricing []ModelPricing) *Calculator {
	if pricing == nil {
		pricing = DefaultPricing
	}

	c := &Calculator{
		pricing: make(map[string]ModelPricing),
	}

	for _, p := range pricing {
		c.pricing[p.Model] = p
	}

	return c
}

// Calculate returns the cost for the given model and token counts.
// Returns 0 if the model is not found in the pricing data.
func (c *Calculator) Calculate(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := c.findPricing(model)
	if !ok {
		return 0 // Unknown model, return 0
	}

	inputCost := float64(inputTokens) / 1000.0 * pricing.InputCostPer1K
	outputCost := float64(outputTokens) / 1000.0 * pricing.OutputCostPer1K

	return inputCost + outputCost
}

// findPricing finds the pricing for a model, supporting wildcards.
// Tries exact match first, then wildcard matching.
func (c *Calculator) findPricing(model string) (ModelPricing, bool) {
	// Normalize model name to lowercase for comparison
	modelLower := strings.ToLower(model)

	// 1. Try exact match first
	for pattern, p := range c.pricing {
		if strings.EqualFold(pattern, model) {
			return p, true
		}
	}

	// 2. Try wildcard matching (prefix matching)
	// Sort by pattern length descending to match most specific patterns first
	var bestMatch *ModelPricing
	var bestMatchLen int

	for pattern, p := range c.pricing {
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.ToLower(strings.TrimSuffix(pattern, "*"))
			if strings.HasPrefix(modelLower, prefix) {
				// Keep the longest matching prefix
				if len(prefix) > bestMatchLen {
					pCopy := p
					bestMatch = &pCopy
					bestMatchLen = len(prefix)
				}
			}
		}
	}

	if bestMatch != nil {
		return *bestMatch, true
	}

	return ModelPricing{}, false
}

// AddPricing adds or updates pricing for a specific model.
func (c *Calculator) AddPricing(pricing ModelPricing) {
	c.pricing[pricing.Model] = pricing
}

// GetPricing retrieves the pricing for a model.
// Returns the pricing and true if found, zero pricing and false otherwise.
func (c *Calculator) GetPricing(model string) (ModelPricing, bool) {
	return c.findPricing(model)
}
