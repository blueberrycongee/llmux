package llmux

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_DefaultProvider_PrefersProvider(t *testing.T) {
	var primaryHits atomic.Int64
	var secondaryHits atomic.Int64

	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits.Add(1)
		resp := ChatResponse{
			ID:      "primary",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: json.RawMessage(`"ok"`),
					},
					FinishReason: "stop",
				},
			},
			Usage: &Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer primaryServer.Close()

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondaryHits.Add(1)
		resp := ChatResponse{
			ID:      "secondary",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: json.RawMessage(`"ok"`),
					},
					FinishReason: "stop",
				},
			},
			Usage: &Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer secondaryServer.Close()

	pricingFile := writePricingFile(t, map[string]map[string]interface{}{
		"primary/test-model": {
			"input_cost_per_token":  0.01,
			"output_cost_per_token": 0.01,
		},
		"secondary/test-model": {
			"input_cost_per_token":  0.0001,
			"output_cost_per_token": 0.0001,
		},
	})

	primaryProvider := &httpMockProvider{
		name:    "primary",
		models:  []string{"test-model"},
		baseURL: primaryServer.URL,
	}
	secondaryProvider := &httpMockProvider{
		name:    "secondary",
		models:  []string{"test-model"},
		baseURL: secondaryServer.URL,
	}

	client, err := New(
		WithProviderInstance("primary", primaryProvider, []string{"test-model"}),
		WithProviderInstance("secondary", secondaryProvider, []string{"test-model"}),
		WithRouterStrategy(StrategyLowestCost),
		WithDefaultProvider("primary"),
		WithPricingFile(pricingFile),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	resp, err := client.ChatCompletion(context.Background(), &ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Usage == nil {
		t.Fatalf("expected usage to be set")
	}
	if resp.Usage.Provider != "primary" {
		t.Fatalf("expected provider=primary, got %q", resp.Usage.Provider)
	}
	if primaryHits.Load() != 1 {
		t.Fatalf("expected primary to be called once, got %d", primaryHits.Load())
	}
	if secondaryHits.Load() != 0 {
		t.Fatalf("expected secondary to be unused, got %d", secondaryHits.Load())
	}
}
