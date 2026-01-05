// Package errors defines unified error types for LLM gateway operations.
// All provider-specific errors are mapped to these standard error types.
package errors

import (
	"fmt"
	"net/http"
)

// LLMError represents a standardized error from an LLM provider.
// It contains all necessary information for error handling, logging, and client response.
type LLMError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Retryable  bool   `json:"-"`
}

// Error implements the error interface.
func (e *LLMError) Error() string {
	return fmt.Sprintf("[%s] %s (provider=%s, model=%s, code=%d)",
		e.Type, e.Message, e.Provider, e.Model, e.StatusCode)
}

// HTTPStatusCode returns the appropriate HTTP status code for the error.
func (e *LLMError) HTTPStatusCode() int {
	if e.StatusCode > 0 {
		return e.StatusCode
	}
	return http.StatusInternalServerError
}

// Common error types as constants for consistency.
const (
	TypeAuthentication     = "authentication_error"
	TypeRateLimit          = "rate_limit_error"
	TypeInvalidRequest     = "invalid_request_error"
	TypeNotFound           = "not_found_error"
	TypeTimeout            = "timeout_error"
	TypeServiceUnavailable = "service_unavailable_error"
	TypeInternalError      = "internal_error"
	TypeContextLength      = "context_length_exceeded"
	TypeContentPolicy      = "content_policy_violation"
)

// NewAuthenticationError creates an authentication error (401).
func NewAuthenticationError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusUnauthorized,
		Message:    message,
		Type:       TypeAuthentication,
		Provider:   provider,
		Model:      model,
		Retryable:  false,
	}
}

// NewRateLimitError creates a rate limit error (429).
func NewRateLimitError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusTooManyRequests,
		Message:    message,
		Type:       TypeRateLimit,
		Provider:   provider,
		Model:      model,
		Retryable:  true,
	}
}

// NewInvalidRequestError creates an invalid request error (400).
func NewInvalidRequestError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusBadRequest,
		Message:    message,
		Type:       TypeInvalidRequest,
		Provider:   provider,
		Model:      model,
		Retryable:  false,
	}
}

// NewNotFoundError creates a not found error (404).
func NewNotFoundError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusNotFound,
		Message:    message,
		Type:       TypeNotFound,
		Provider:   provider,
		Model:      model,
		Retryable:  false,
	}
}

// NewTimeoutError creates a timeout error (408).
func NewTimeoutError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusRequestTimeout,
		Message:    message,
		Type:       TypeTimeout,
		Provider:   provider,
		Model:      model,
		Retryable:  true,
	}
}

// NewServiceUnavailableError creates a service unavailable error (503).
func NewServiceUnavailableError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusServiceUnavailable,
		Message:    message,
		Type:       TypeServiceUnavailable,
		Provider:   provider,
		Model:      model,
		Retryable:  true,
	}
}

// NewInternalError creates an internal server error (500).
func NewInternalError(provider, model, message string) *LLMError {
	return &LLMError{
		StatusCode: http.StatusInternalServerError,
		Message:    message,
		Type:       TypeInternalError,
		Provider:   provider,
		Model:      model,
		Retryable:  false,
	}
}

// IsCooldownRequired determines if a deployment should be cooled down based on error.
// Rate limits, auth errors, timeouts, and not found errors trigger cooldown.
// Other 4xx errors do not trigger cooldown as they are likely client errors.
func IsCooldownRequired(statusCode int) bool {
	if statusCode >= 400 && statusCode < 500 {
		switch statusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusUnauthorized,   // 401
			http.StatusRequestTimeout, // 408
			http.StatusNotFound:       // 404
			return true
		default:
			return false
		}
	}
	// All 5xx errors trigger cooldown
	return statusCode >= 500
}
