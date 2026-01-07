// Package plugin provides a plugin system for LLMux that enables
// request interception, modification, and short-circuiting.
//
// The plugin system follows the middleware pattern with PreHook/PostHook
// lifecycle methods, allowing plugins to:
//   - Modify requests before they reach the provider
//   - Short-circuit requests (e.g., cache hits, rate limiting)
//   - Modify responses after provider returns
//   - Recover from errors or convert success to errors
package plugin

import (
	"context"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// Plugin defines the interface for LLMux plugins.
// Plugins can intercept and modify requests/responses at various lifecycle points.
type Plugin interface {
	// Name returns the plugin identifier for logging and debugging.
	Name() string

	// Priority returns the plugin execution priority.
	// Lower numbers execute first in PreHook, last in PostHook (stack order).
	Priority() int

	// PreHook is called before the request is sent to the provider.
	// Returns:
	//   - *types.ChatRequest: modified request (can be original or new)
	//   - *ShortCircuit: short-circuit decision (non-nil skips provider call)
	//   - error: plugin internal error (logged but doesn't stop execution)
	PreHook(ctx *Context, req *types.ChatRequest) (*types.ChatRequest, *ShortCircuit, error)

	// PostHook is called after receiving the provider response (or after short-circuit).
	// Plugins can:
	//   - Modify the response
	//   - Recover from errors (set err to nil and provide response)
	//   - Convert success to error (set response to nil and provide err)
	// Returns:
	//   - *types.ChatResponse: modified response
	//   - error: modified response error
	//   - error: plugin internal error (logged but doesn't stop execution)
	PostHook(ctx *Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error)

	// Cleanup is called when LLMux shuts down, for resource cleanup.
	Cleanup() error
}

// StreamPlugin extends Plugin with streaming support.
type StreamPlugin interface {
	Plugin

	// PreStreamHook is called before a streaming request starts.
	PreStreamHook(ctx *Context, req *types.ChatRequest) (*types.ChatRequest, *StreamShortCircuit, error)

	// OnStreamChunk is called for each streaming response chunk.
	// Return nil to skip the chunk, or modified chunk to pass through.
	OnStreamChunk(ctx *Context, chunk *types.StreamChunk) (*types.StreamChunk, error)

	// PostStreamHook is called after the stream completes.
	PostStreamHook(ctx *Context, err error) error
}

// ShortCircuit represents a plugin's decision to short-circuit the request.
type ShortCircuit struct {
	// Response is returned directly if non-nil (skips provider call).
	Response *types.ChatResponse

	// Error is returned directly if non-nil (skips provider call).
	Error error

	// AllowFallback controls whether fallback to other deployments is allowed.
	// Default is true (allow fallback).
	AllowFallback bool

	// Metadata is optional data passed to subsequent PostHooks.
	Metadata map[string]any
}

// StreamShortCircuit represents a streaming request short-circuit decision.
type StreamShortCircuit struct {
	// Stream is returned directly if non-nil (skips provider call).
	Stream <-chan *types.StreamChunk

	// Error is returned directly if non-nil.
	Error error

	// AllowFallback controls whether fallback is allowed.
	AllowFallback bool

	// Metadata is optional data passed to subsequent hooks.
	Metadata map[string]any
}

// Context provides execution context for plugins.
type Context struct {
	context.Context

	// RequestID is the unique identifier for this request.
	RequestID string

	// Model is the requested model name.
	Model string

	// Provider is the selected provider name.
	Provider string

	// Deployment is the selected deployment.
	Deployment *provider.Deployment

	// IsStreaming indicates if this is a streaming request.
	IsStreaming bool

	// StartTime is when the request started.
	StartTime time.Time

	// Auth contains authentication context if auth is enabled.
	Auth *auth.AuthContext

	// values stores plugin-shared key-value pairs.
	values map[string]any
	mu     sync.RWMutex
}

// NewContext creates a new plugin context.
func NewContext(ctx context.Context, requestID string) *Context {
	return &Context{
		Context:   ctx,
		RequestID: requestID,
		StartTime: time.Now(),
		values:    make(map[string]any),
	}
}

// Set stores a value in the context for sharing between plugins.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}

// Get retrieves a value from the context.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.values == nil {
		return nil, false
	}
	v, ok := c.values[key]
	return v, ok
}

// GetString retrieves a string value from the context.
func (c *Context) GetString(key string) string {
	if v, ok := c.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt retrieves an int value from the context.
func (c *Context) GetInt(key string) int {
	if v, ok := c.Get(key); ok {
		if i, ok := v.(int); ok {
			return i
		}
	}
	return 0
}

// GetBool retrieves a bool value from the context.
func (c *Context) GetBool(key string) bool {
	if v, ok := c.Get(key); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Reset clears the context for reuse.
func (c *Context) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Context = nil
	c.RequestID = ""
	c.Model = ""
	c.Provider = ""
	c.Deployment = nil
	c.IsStreaming = false
	c.StartTime = time.Time{}
	c.Auth = nil
	// Clear map but keep capacity
	for k := range c.values {
		delete(c.values, k)
	}
}
