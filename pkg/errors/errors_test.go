package errors

import (
	"net/http"
	"testing"
)

func TestIsCooldownRequired(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		// Should trigger cooldown
		{"rate limit 429", http.StatusTooManyRequests, true},
		{"unauthorized 401", http.StatusUnauthorized, true},
		{"timeout 408", http.StatusRequestTimeout, true},
		{"not found 404", http.StatusNotFound, true},
		{"internal error 500", http.StatusInternalServerError, true},
		{"bad gateway 502", http.StatusBadGateway, true},
		{"service unavailable 503", http.StatusServiceUnavailable, true},

		// Should NOT trigger cooldown
		{"bad request 400", http.StatusBadRequest, false},
		{"forbidden 403", http.StatusForbidden, false},
		{"conflict 409", http.StatusConflict, false},
		{"unprocessable 422", http.StatusUnprocessableEntity, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCooldownRequired(tt.statusCode)
			if got != tt.want {
				t.Errorf("IsCooldownRequired(%d) = %v, want %v", tt.statusCode, got, tt.want)
			}
		})
	}
}

func TestLLMError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := NewRateLimitError("openai", "gpt-4", "rate limit exceeded")
		msg := err.Error()

		if msg == "" {
			t.Error("error message should not be empty")
		}

		// Should contain key information
		contains := []string{"rate_limit_error", "openai", "gpt-4", "429"}
		for _, s := range contains {
			if !containsString(msg, s) {
				t.Errorf("error message should contain %q, got %q", s, msg)
			}
		}
	})

	t.Run("HTTP status codes", func(t *testing.T) {
		tests := []struct {
			name     string
			err      *LLMError
			wantCode int
		}{
			{"auth error", NewAuthenticationError("p", "m", "msg"), 401},
			{"rate limit", NewRateLimitError("p", "m", "msg"), 429},
			{"bad request", NewInvalidRequestError("p", "m", "msg"), 400},
			{"not found", NewNotFoundError("p", "m", "msg"), 404},
			{"timeout", NewTimeoutError("p", "m", "msg"), 408},
			{"unavailable", NewServiceUnavailableError("p", "m", "msg"), 503},
			{"internal", NewInternalError("p", "m", "msg"), 500},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.err.HTTPStatusCode(); got != tt.wantCode {
					t.Errorf("HTTPStatusCode() = %d, want %d", got, tt.wantCode)
				}
			})
		}
	})

	t.Run("retryable flag", func(t *testing.T) {
		retryable := []func(string, string, string) *LLMError{
			NewRateLimitError,
			NewTimeoutError,
			NewServiceUnavailableError,
		}
		for _, fn := range retryable {
			err := fn("p", "m", "msg")
			if !err.Retryable {
				t.Errorf("%s should be retryable", err.Type)
			}
		}

		notRetryable := []func(string, string, string) *LLMError{
			NewAuthenticationError,
			NewInvalidRequestError,
			NewNotFoundError,
			NewInternalError,
		}
		for _, fn := range notRetryable {
			err := fn("p", "m", "msg")
			if err.Retryable {
				t.Errorf("%s should not be retryable", err.Type)
			}
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
