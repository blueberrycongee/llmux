package llmux

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/resilience"
)

// TestClient_CheckRateLimit tests the checkRateLimit method.
func TestClient_CheckRateLimit(t *testing.T) {
	t.Run("returns nil when rate limiter is disabled", func(t *testing.T) {
		client, err := New(
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled: false,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		err = client.checkRateLimit(context.Background(), "test-key", "gpt-4", 0)
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("returns nil when rate limiter is nil", func(t *testing.T) {
		client, err := New(
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled:  true,
				RPMLimit: 100,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Limiter is nil, so should skip
		err = client.checkRateLimit(context.Background(), "test-key", "gpt-4", 0)
		if err != nil {
			t.Errorf("expected nil error when limiter is nil, got: %v", err)
		}
	})

	t.Run("allows request when under limit", func(t *testing.T) {
		allowLimiter := &mockDistributedLimiter{
			checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
				results := make([]resilience.LimitResult, len(descriptors))
				for i := range descriptors {
					results[i] = resilience.LimitResult{
						Allowed:   true,
						Current:   1,
						Remaining: 99,
						ResetAt:   time.Now().Add(time.Minute).Unix(),
					}
				}
				return results, nil
			},
		}

		client, err := New(
			WithRateLimiter(allowLimiter),
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled:     true,
				RPMLimit:    100,
				TPMLimit:    10000,
				WindowSize:  time.Minute,
				KeyStrategy: RateLimitKeyByAPIKey,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		err = client.checkRateLimit(context.Background(), "test-key", "gpt-4", 100)
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("rejects request when RPM limit exceeded", func(t *testing.T) {
		denyLimiter := &mockDistributedLimiter{
			checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
				results := make([]resilience.LimitResult, len(descriptors))
				for i, desc := range descriptors {
					if desc.Type == resilience.LimitTypeRequests {
						results[i] = resilience.LimitResult{
							Allowed:   false,
							Current:   100,
							Remaining: 0,
							ResetAt:   time.Now().Add(30 * time.Second).Unix(),
						}
					} else {
						results[i] = resilience.LimitResult{
							Allowed:   true,
							Current:   500,
							Remaining: 9500,
							ResetAt:   time.Now().Add(time.Minute).Unix(),
						}
					}
				}
				return results, nil
			},
		}

		client, err := New(
			WithRateLimiter(denyLimiter),
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled:     true,
				RPMLimit:    100,
				TPMLimit:    10000,
				WindowSize:  time.Minute,
				KeyStrategy: RateLimitKeyByAPIKey,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		err = client.checkRateLimit(context.Background(), "test-key", "gpt-4", 100)
		if err == nil {
			t.Error("expected rate limit error, got nil")
		}
		if !strings.Contains(err.Error(), "rate limit") {
			t.Errorf("expected rate limit error message, got: %v", err)
		}
	})

	t.Run("rejects request when TPM limit exceeded", func(t *testing.T) {
		denyLimiter := &mockDistributedLimiter{
			checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
				results := make([]resilience.LimitResult, len(descriptors))
				for i, desc := range descriptors {
					if desc.Type == resilience.LimitTypeTokens {
						results[i] = resilience.LimitResult{
							Allowed:   false,
							Current:   10000,
							Remaining: 0,
							ResetAt:   time.Now().Add(30 * time.Second).Unix(),
						}
					} else {
						results[i] = resilience.LimitResult{
							Allowed:   true,
							Current:   50,
							Remaining: 50,
							ResetAt:   time.Now().Add(time.Minute).Unix(),
						}
					}
				}
				return results, nil
			},
		}

		client, err := New(
			WithRateLimiter(denyLimiter),
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled:     true,
				RPMLimit:    100,
				TPMLimit:    10000,
				WindowSize:  time.Minute,
				KeyStrategy: RateLimitKeyByAPIKey,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		err = client.checkRateLimit(context.Background(), "test-key", "gpt-4", 1000)
		if err == nil {
			t.Error("expected rate limit error, got nil")
		}
		if !strings.Contains(err.Error(), "rate limit") {
			t.Errorf("expected rate limit error message, got: %v", err)
		}
	})

	t.Run("uses different keys based on strategy", func(t *testing.T) {
		var capturedKey string
		captureLimiter := &mockDistributedLimiter{
			checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
				if len(descriptors) > 0 {
					capturedKey = descriptors[0].Key
				}
				results := make([]resilience.LimitResult, len(descriptors))
				for i := range descriptors {
					results[i] = resilience.LimitResult{
						Allowed:   true,
						Current:   1,
						Remaining: 99,
					}
				}
				return results, nil
			},
		}

		client, err := New(
			WithRateLimiter(captureLimiter),
			WithRateLimiterConfig(RateLimiterConfig{
				Enabled:     true,
				RPMLimit:    100,
				WindowSize:  time.Minute,
				KeyStrategy: RateLimitKeyByAPIKey,
			}),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Test with different keys
		_ = client.checkRateLimit(context.Background(), "api-key-123", "gpt-4", 100)
		if capturedKey != "api-key-123" {
			t.Errorf("expected key 'api-key-123', got '%s'", capturedKey)
		}

		_ = client.checkRateLimit(context.Background(), "api-key-456", "claude-3", 100)
		if capturedKey != "api-key-456" {
			t.Errorf("expected key 'api-key-456', got '%s'", capturedKey)
		}
	})
}
func TestClient_RateLimitKeyStrategyByModel(t *testing.T) {
	var capturedKeys []string
	captureLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			if len(descriptors) > 0 {
				capturedKeys = append(capturedKeys, descriptors[0].Key)
			}
			results := make([]resilience.LimitResult, len(descriptors))
			for i, desc := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Current:   1,
					Remaining: desc.Limit - 1,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:      "test-id",
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
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"model-a", "model-b"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"model-a", "model-b"}),
		withTestPricing(t, "model-a", "model-b"),
		WithRateLimiter(captureLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByModel,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model:    "model-a",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion(model-a) error = %v", err)
	}

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model:    "model-b",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion(model-b) error = %v", err)
	}

	if len(capturedKeys) != 2 {
		t.Fatalf("expected 2 rate limit checks, got %d", len(capturedKeys))
	}
	if capturedKeys[0] != "model-a" || capturedKeys[1] != "model-b" {
		t.Fatalf("expected keys [model-a model-b], got %v", capturedKeys)
	}
}

func TestClient_RateLimitKeyStrategyByAPIKey_NoKeyFallbacksToDefault(t *testing.T) {
	client, err := New(
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	key := client.buildRateLimitKey("test-model", "user-1", "")
	if key != "default" {
		t.Fatalf("expected default key, got %q", key)
	}
}

func TestClient_RateLimitKeyStrategyByAPIKeyFromContext(t *testing.T) {
	var capturedKey string
	captureLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			if len(descriptors) > 0 {
				capturedKey = descriptors[0].Key
			}
			results := make([]resilience.LimitResult, len(descriptors))
			for i, desc := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Current:   1,
					Remaining: desc.Limit - 1,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:      "test-id",
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
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRateLimiter(captureLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx := WithRateLimitAPIKey(context.Background(), "api-key-123")
	_, err = client.ChatCompletion(ctx, &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}},
	)
	if err != nil {
		t.Fatalf("ChatCompletion error = %v", err)
	}

	if capturedKey != "api-key-123" {
		t.Fatalf("expected key 'api-key-123', got %q", capturedKey)
	}
}

func TestClient_RateLimitKeyStrategyByAPIKeyFromAuthContext(t *testing.T) {
	var capturedKey string
	captureLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			if len(descriptors) > 0 {
				capturedKey = descriptors[0].Key
			}
			results := make([]resilience.LimitResult, len(descriptors))
			for i, desc := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Current:   1,
					Remaining: desc.Limit - 1,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:      "test-id",
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
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRateLimiter(captureLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	authCtx := &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-abc"},
	}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, authCtx)
	_, err = client.ChatCompletion(ctx, &ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}},
	)
	if err != nil {
		t.Fatalf("ChatCompletion error = %v", err)
	}

	if capturedKey != "key-abc" {
		t.Fatalf("expected key 'key-abc', got %q", capturedKey)
	}
}

func TestClient_RateLimitKeyStrategyByUser(t *testing.T) {
	var capturedKey string
	captureLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			if len(descriptors) > 0 {
				capturedKey = descriptors[0].Key
			}
			results := make([]resilience.LimitResult, len(descriptors))
			for i, desc := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Current:   1,
					Remaining: desc.Limit - 1,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			ID:      "test-id",
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
	defer server.Close()

	mock := &httpMockProvider{
		name:    "mock",
		models:  []string{"test-model"},
		baseURL: server.URL,
	}

	client, err := New(
		WithProviderInstance("mock", mock, []string{"test-model"}),
		withTestPricing(t, "test-model"),
		WithRateLimiter(captureLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByUser,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	_, err = client.ChatCompletion(context.Background(), &ChatRequest{
		Model:    "test-model",
		User:     "user-123",
		Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}},
	)
	if err != nil {
		t.Fatalf("ChatCompletion error = %v", err)
	}

	if capturedKey != "user-123" {
		t.Fatalf("expected key 'user-123', got %q", capturedKey)
	}
}
