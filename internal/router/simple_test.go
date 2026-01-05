package router

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

func TestSimpleRouter_Pick(t *testing.T) {
	t.Run("returns deployment for registered model", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)
		dep := &provider.Deployment{
			ID:           "dep-1",
			ProviderName: "openai",
			ModelName:    "gpt-4",
		}
		r.AddDeployment(dep)

		got, err := r.Pick(context.Background(), "gpt-4")
		if err != nil {
			t.Fatalf("Pick() error = %v", err)
		}
		if got.ID != "dep-1" {
			t.Errorf("Pick() = %s, want dep-1", got.ID)
		}
	})

	t.Run("returns error for unknown model", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)

		_, err := r.Pick(context.Background(), "unknown-model")
		if err != ErrNoAvailableDeployment {
			t.Errorf("Pick() error = %v, want ErrNoAvailableDeployment", err)
		}
	})

	t.Run("skips cooled down deployments", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)

		dep1 := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
		dep2 := &provider.Deployment{ID: "dep-2", ModelName: "gpt-4", ProviderName: "openai"}
		r.AddDeployment(dep1)
		r.AddDeployment(dep2)

		// Cool down dep-1
		rateLimitErr := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
		r.ReportFailure(dep1, rateLimitErr)

		// Should only return dep-2
		for i := 0; i < 10; i++ {
			got, err := r.Pick(context.Background(), "gpt-4")
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}
			if got.ID != "dep-2" {
				t.Errorf("Pick() = %s, want dep-2 (dep-1 should be cooled down)", got.ID)
			}
		}
	})

	t.Run("returns error when all deployments cooled down", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)

		dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
		r.AddDeployment(dep)

		// Cool down the only deployment
		rateLimitErr := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
		r.ReportFailure(dep, rateLimitErr)

		_, err := r.Pick(context.Background(), "gpt-4")
		if err != ErrNoAvailableDeployment {
			t.Errorf("Pick() error = %v, want ErrNoAvailableDeployment", err)
		}
	})
}

func TestSimpleRouter_ReportSuccess(t *testing.T) {
	r := NewSimpleRouter(60 * time.Second)
	dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
	r.AddDeployment(dep)

	// Report multiple successes
	r.ReportSuccess(dep, 100*time.Millisecond)
	r.ReportSuccess(dep, 200*time.Millisecond)
	r.ReportSuccess(dep, 150*time.Millisecond)

	// Verify stats updated
	r.mu.RLock()
	stats := r.stats[dep.ID]
	r.mu.RUnlock()

	if stats.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", stats.TotalRequests)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", stats.SuccessCount)
	}
	if stats.AvgLatencyMs <= 0 {
		t.Error("AvgLatencyMs should be positive")
	}
}

func TestSimpleRouter_ReportFailure(t *testing.T) {
	t.Run("cooldown on rate limit", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)
		dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
		r.AddDeployment(dep)

		err := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
		r.ReportFailure(dep, err)

		if !r.IsCircuitOpen(dep) {
			t.Error("circuit should be open after rate limit error")
		}
	})

	t.Run("no cooldown on bad request", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)
		dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
		r.AddDeployment(dep)

		err := llmerrors.NewInvalidRequestError("openai", "gpt-4", "bad request")
		r.ReportFailure(dep, err)

		if r.IsCircuitOpen(dep) {
			t.Error("circuit should NOT be open after bad request error")
		}
	})

	t.Run("cooldown on 5xx errors", func(t *testing.T) {
		r := NewSimpleRouter(60 * time.Second)
		dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
		r.AddDeployment(dep)

		err := llmerrors.NewServiceUnavailableError("openai", "gpt-4", "service down")
		r.ReportFailure(dep, err)

		if !r.IsCircuitOpen(dep) {
			t.Error("circuit should be open after 503 error")
		}
	})
}

func TestSimpleRouter_ModelAlias(t *testing.T) {
	r := NewSimpleRouter(60 * time.Second)
	dep := &provider.Deployment{
		ID:           "dep-1",
		ProviderName: "openai",
		ModelName:    "gpt-4-0613",
		ModelAlias:   "gpt-4", // User requests "gpt-4", routes to "gpt-4-0613"
	}
	r.AddDeployment(dep)

	// Should be accessible via alias
	got, err := r.Pick(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if got.ModelName != "gpt-4-0613" {
		t.Errorf("ModelName = %s, want gpt-4-0613", got.ModelName)
	}
}

func TestSimpleRouter_RemoveDeployment(t *testing.T) {
	r := NewSimpleRouter(60 * time.Second)
	dep := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
	r.AddDeployment(dep)

	// Verify it exists
	_, err := r.Pick(context.Background(), "gpt-4")
	if err != nil {
		t.Fatalf("deployment should exist: %v", err)
	}

	// Remove it
	r.RemoveDeployment("dep-1")

	// Should no longer be available
	_, err = r.Pick(context.Background(), "gpt-4")
	if err != ErrNoAvailableDeployment {
		t.Errorf("Pick() error = %v, want ErrNoAvailableDeployment", err)
	}
}

func TestSimpleRouter_GetDeployments(t *testing.T) {
	r := NewSimpleRouter(60 * time.Second)

	dep1 := &provider.Deployment{ID: "dep-1", ModelName: "gpt-4", ProviderName: "openai"}
	dep2 := &provider.Deployment{ID: "dep-2", ModelName: "gpt-4", ProviderName: "azure"}
	r.AddDeployment(dep1)
	r.AddDeployment(dep2)

	deps := r.GetDeployments("gpt-4")
	if len(deps) != 2 {
		t.Errorf("GetDeployments() count = %d, want 2", len(deps))
	}
}

func TestIsCooldownRequired_Integration(t *testing.T) {
	// Verify our error types trigger correct cooldown behavior
	tests := []struct {
		name     string
		err      *llmerrors.LLMError
		wantCool bool
	}{
		{"rate limit", llmerrors.NewRateLimitError("p", "m", "msg"), true},
		{"auth error", llmerrors.NewAuthenticationError("p", "m", "msg"), true},
		{"timeout", llmerrors.NewTimeoutError("p", "m", "msg"), true},
		{"not found", llmerrors.NewNotFoundError("p", "m", "msg"), true},
		{"service unavailable", llmerrors.NewServiceUnavailableError("p", "m", "msg"), true},
		{"internal error", llmerrors.NewInternalError("p", "m", "msg"), true},
		{"bad request", llmerrors.NewInvalidRequestError("p", "m", "msg"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llmerrors.IsCooldownRequired(tt.err.StatusCode)
			if got != tt.wantCool {
				t.Errorf("IsCooldownRequired(%d) = %v, want %v (status: %s)",
					tt.err.StatusCode, got, tt.wantCool, http.StatusText(tt.err.StatusCode))
			}
		})
	}
}
