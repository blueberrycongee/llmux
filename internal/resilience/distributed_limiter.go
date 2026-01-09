package resilience

import (
	"context"
	"time"
)

// LimitType defines what we are limiting (Requests, Tokens, etc.)
type LimitType string

const (
	LimitTypeRequests LimitType = "requests" // RPM
	LimitTypeTokens   LimitType = "tokens"   // TPM
)

// Descriptor defines a specific limit rule
type Descriptor struct {
	Key    string        // e.g., "api-key-123"
	Value  string        // e.g., "model-gpt4"
	Limit  int64         // The limit threshold (e.g., 100)
	Type   LimitType     // RPM or TPM
	Window time.Duration // Window size (default 1m)
}

// LimitResult contains the result of a check
type LimitResult struct {
	Allowed   bool
	Current   int64
	Remaining int64
	ResetAt   int64 // Timestamp when window resets
	Error     error
}

// DistributedLimiter interface supports batch checking, mirroring litellm's capability to check RPM and TPM simultaneously.
type DistributedLimiter interface {
	// CheckAllow atomically checks and increments limits for multiple descriptors.
	// Returns a list of results corresponding to the input descriptors.
	CheckAllow(ctx context.Context, descriptors []Descriptor) ([]LimitResult, error)
}
