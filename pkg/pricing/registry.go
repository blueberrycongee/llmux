// Package pricing provides functionality for managing and retrieving model pricing information.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

//go:embed data/defaults.json
var defaultPrices []byte

type ModelPrice struct {
	Provider               string  `json:"litellm_provider"`
	InputCostPerToken      float64 `json:"input_cost_per_token"`
	OutputCostPerToken     float64 `json:"output_cost_per_token"`
	CacheReadCostPerToken  float64 `json:"cache_read_input_token_cost,omitempty"`
	CacheWriteCostPerToken float64 `json:"cache_creation_input_token_cost,omitempty"`
	Mode                   string  `json:"mode"`
}

type Registry struct {
	prices map[string]ModelPrice
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	r := &Registry{
		prices: make(map[string]ModelPrice),
	}
	// Load defaults
	if err := r.loadBytes(defaultPrices); err != nil {
		// This should not happen in production if defaults.json is correct
		// But we can log it or panic since it's embedded data
		panic(fmt.Sprintf("failed to load default prices: %v", err))
	}
	return r
}

func (r *Registry) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.loadBytes(data)
}

func (r *Registry) loadBytes(data []byte) error {
	var prices map[string]ModelPrice
	if err := json.Unmarshal(data, &prices); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for k, v := range prices {
		r.prices[k] = v
	}
	return nil
}

func (r *Registry) GetPrice(model, provider string) (ModelPrice, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Try "provider/model"
	// Some keys in the registry might be stored as "provider/model"
	key := fmt.Sprintf("%s/%s", provider, model)
	if p, ok := r.prices[key]; ok {
		return p, true
	}

	// 2. Try "model"
	// Most keys are just "model"
	if p, ok := r.prices[model]; ok {
		return p, true
	}

	return ModelPrice{}, false
}
