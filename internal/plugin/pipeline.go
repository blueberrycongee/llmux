// Package plugin provides a plugin system for LLMux.
package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// PipelineConfig holds configuration for the plugin pipeline.
type PipelineConfig struct {
	// PreHookTimeout is the timeout for each PreHook execution (default: 10s).
	PreHookTimeout time.Duration

	// PostHookTimeout is the timeout for each PostHook execution (default: 10s).
	PostHookTimeout time.Duration

	// PropagateErrors determines whether plugin errors are propagated to the caller.
	// When false (default), plugin errors are logged but don't affect the request.
	PropagateErrors bool

	// MaxPlugins is the maximum number of plugins allowed (default: 100).
	MaxPlugins int
}

// DefaultPipelineConfig returns a PipelineConfig with sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		PreHookTimeout:  10 * time.Second,
		PostHookTimeout: 10 * time.Second,
		PropagateErrors: false,
		MaxPlugins:      100,
	}
}

// Pipeline manages the execution of plugins in order.
// Plugins are executed in priority order for PreHooks (ascending),
// and in reverse priority order for PostHooks (descending).
type Pipeline struct {
	plugins []Plugin
	logger  *slog.Logger
	config  PipelineConfig
	closed  atomic.Bool

	// Context pool for reuse
	ctxPool sync.Pool

	mu sync.RWMutex
}

// NewPipeline creates a new plugin pipeline.
func NewPipeline(logger *slog.Logger, config PipelineConfig) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}

	// Apply defaults for zero values
	if config.PreHookTimeout == 0 {
		config.PreHookTimeout = 10 * time.Second
	}
	if config.PostHookTimeout == 0 {
		config.PostHookTimeout = 10 * time.Second
	}
	if config.MaxPlugins == 0 {
		config.MaxPlugins = 100
	}

	return &Pipeline{
		plugins: make([]Plugin, 0),
		logger:  logger,
		config:  config,
		ctxPool: sync.Pool{
			New: func() any {
				return &Context{
					values: make(map[string]any),
				}
			},
		},
	}
}

// Register adds a plugin to the pipeline.
// Plugins are sorted by priority (lower priority numbers execute first in PreHook).
func (p *Pipeline) Register(plugin Plugin) error {
	if p.closed.Load() {
		return ErrPipelineClosed
	}
	if plugin == nil {
		return ErrNilPlugin
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for duplicate names
	for _, existing := range p.plugins {
		if existing.Name() == plugin.Name() {
			return ErrDuplicatePlugin
		}
	}

	// Check max plugins
	if len(p.plugins) >= p.config.MaxPlugins {
		return ErrTooManyPlugins
	}

	// Insert in priority order (binary search)
	idx := sort.Search(len(p.plugins), func(i int) bool {
		return p.plugins[i].Priority() > plugin.Priority()
	})

	// Insert at position idx
	p.plugins = append(p.plugins, nil)
	copy(p.plugins[idx+1:], p.plugins[idx:])
	p.plugins[idx] = plugin

	p.logger.Info("plugin registered",
		"name", plugin.Name(),
		"priority", plugin.Priority(),
		"total_plugins", len(p.plugins),
	)

	return nil
}

// Unregister removes a plugin by name.
func (p *Pipeline) Unregister(name string) error {
	if p.closed.Load() {
		return ErrPipelineClosed
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for i, plugin := range p.plugins {
		if plugin.Name() == name {
			p.plugins = append(p.plugins[:i], p.plugins[i+1:]...)
			p.logger.Info("plugin unregistered", "name", name)
			return nil
		}
	}

	return ErrPluginNotFound
}

// Plugins returns a copy of the registered plugins.
func (p *Pipeline) Plugins() []Plugin {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Plugin, len(p.plugins))
	copy(result, p.plugins)
	return result
}

// PluginCount returns the number of registered plugins.
func (p *Pipeline) PluginCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.plugins)
}

// GetContext acquires a context from the pool.
func (p *Pipeline) GetContext(ctx context.Context, requestID string) *Context {
	pluginCtx := p.ctxPool.Get().(*Context)
	pluginCtx.Context = ctx
	pluginCtx.RequestID = requestID
	pluginCtx.StartTime = time.Now()
	return pluginCtx
}

// PutContext returns a context to the pool.
func (p *Pipeline) PutContext(ctx *Context) {
	ctx.Reset()
	p.ctxPool.Put(ctx)
}

// RunPreHooks executes all PreHooks in priority order (ascending).
// Returns the (potentially modified) request, any short-circuit decision,
// and the count of executed hooks (for PostHook reverse execution).
func (p *Pipeline) RunPreHooks(
	ctx *Context,
	req *types.ChatRequest,
) (*types.ChatRequest, *ShortCircuit, int) {
	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	if len(plugins) == 0 {
		return req, nil, 0
	}

	var shortCircuit *ShortCircuit
	executedCount := 0

	for i, plugin := range plugins {
		// Create timeout context for this hook
		hookCtx, cancel := context.WithTimeout(ctx.Context, p.config.PreHookTimeout)
		originalCtx := ctx.Context
		ctx.Context = hookCtx

		p.logger.Debug("running PreHook",
			"plugin", plugin.Name(),
			"priority", plugin.Priority(),
			"request_id", ctx.RequestID,
		)

		startTime := time.Now()
		var err error
		req, shortCircuit, err = plugin.PreHook(ctx, req)
		duration := time.Since(startTime)

		// Restore original context
		ctx.Context = originalCtx
		cancel()

		if err != nil {
			p.logger.Warn("PreHook error",
				"plugin", plugin.Name(),
				"error", err,
				"duration", duration,
				"request_id", ctx.RequestID,
			)
			// Continue to next plugin unless PropagateErrors is set
			if p.config.PropagateErrors {
				// Store error for potential use, but don't short-circuit
			}
		} else {
			p.logger.Debug("PreHook completed",
				"plugin", plugin.Name(),
				"duration", duration,
				"request_id", ctx.RequestID,
			)
		}

		executedCount = i + 1

		if shortCircuit != nil {
			p.logger.Debug("PreHook short-circuit",
				"plugin", plugin.Name(),
				"has_response", shortCircuit.Response != nil,
				"has_error", shortCircuit.Error != nil,
				"allow_fallback", shortCircuit.AllowFallback,
				"request_id", ctx.RequestID,
			)
			return req, shortCircuit, executedCount
		}
	}

	return req, nil, executedCount
}

// RunPostHooks executes PostHooks in reverse priority order (descending).
// Only executes hooks for plugins that had their PreHook run (based on runFrom).
func (p *Pipeline) RunPostHooks(
	ctx *Context,
	resp *types.ChatResponse,
	respErr error,
	runFrom int,
) (*types.ChatResponse, error) {
	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	if len(plugins) == 0 {
		return resp, respErr
	}

	// Boundary check
	if runFrom < 0 {
		runFrom = 0
	}
	if runFrom > len(plugins) {
		runFrom = len(plugins)
	}

	// Execute in reverse order (LIFO - last PreHook first PostHook)
	for i := runFrom - 1; i >= 0; i-- {
		plugin := plugins[i]

		// Create timeout context for this hook
		hookCtx, cancel := context.WithTimeout(ctx.Context, p.config.PostHookTimeout)
		originalCtx := ctx.Context
		ctx.Context = hookCtx

		p.logger.Debug("running PostHook",
			"plugin", plugin.Name(),
			"priority", plugin.Priority(),
			"request_id", ctx.RequestID,
		)

		startTime := time.Now()
		var pluginErr error
		resp, respErr, pluginErr = plugin.PostHook(ctx, resp, respErr)
		duration := time.Since(startTime)

		// Restore original context
		ctx.Context = originalCtx
		cancel()

		if pluginErr != nil {
			p.logger.Warn("PostHook error",
				"plugin", plugin.Name(),
				"error", pluginErr,
				"duration", duration,
				"request_id", ctx.RequestID,
			)
		} else {
			p.logger.Debug("PostHook completed",
				"plugin", plugin.Name(),
				"duration", duration,
				"request_id", ctx.RequestID,
			)
		}
	}

	return resp, respErr
}

// RunStreamPreHooks executes PreStreamHooks for stream-capable plugins.
func (p *Pipeline) RunStreamPreHooks(
	ctx *Context,
	req *types.ChatRequest,
) (*types.ChatRequest, *StreamShortCircuit, int) {
	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	if len(plugins) == 0 {
		return req, nil, 0
	}

	var shortCircuit *StreamShortCircuit
	executedCount := 0

	for i, plugin := range plugins {
		streamPlugin, ok := plugin.(StreamPlugin)
		if !ok {
			// Skip non-stream plugins but count them for PostHook
			executedCount = i + 1
			continue
		}

		// Create timeout context
		hookCtx, cancel := context.WithTimeout(ctx.Context, p.config.PreHookTimeout)
		originalCtx := ctx.Context
		ctx.Context = hookCtx

		p.logger.Debug("running PreStreamHook",
			"plugin", plugin.Name(),
			"request_id", ctx.RequestID,
		)

		startTime := time.Now()
		var err error
		req, shortCircuit, err = streamPlugin.PreStreamHook(ctx, req)
		duration := time.Since(startTime)

		ctx.Context = originalCtx
		cancel()

		if err != nil {
			p.logger.Warn("PreStreamHook error",
				"plugin", plugin.Name(),
				"error", err,
				"duration", duration,
			)
		}

		executedCount = i + 1

		if shortCircuit != nil {
			p.logger.Debug("PreStreamHook short-circuit",
				"plugin", plugin.Name(),
				"has_stream", shortCircuit.Stream != nil,
				"has_error", shortCircuit.Error != nil,
			)
			return req, shortCircuit, executedCount
		}
	}

	return req, nil, executedCount
}

// RunOnStreamChunk executes OnStreamChunk for all stream-capable plugins.
func (p *Pipeline) RunOnStreamChunk(
	ctx *Context,
	chunk *types.StreamChunk,
) (*types.StreamChunk, error) {
	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	if len(plugins) == 0 || chunk == nil {
		return chunk, nil
	}

	var lastErr error
	for _, plugin := range plugins {
		streamPlugin, ok := plugin.(StreamPlugin)
		if !ok {
			continue
		}

		var err error
		chunk, err = streamPlugin.OnStreamChunk(ctx, chunk)
		if err != nil {
			p.logger.Warn("OnStreamChunk error",
				"plugin", plugin.Name(),
				"error", err,
			)
			lastErr = err
		}

		// If chunk is nil, it was filtered out
		if chunk == nil {
			return nil, nil
		}
	}

	return chunk, lastErr
}

// RunStreamPostHooks executes PostStreamHooks in reverse order.
func (p *Pipeline) RunStreamPostHooks(
	ctx *Context,
	err error,
	runFrom int,
) error {
	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	if len(plugins) == 0 {
		return err
	}

	if runFrom < 0 {
		runFrom = 0
	}
	if runFrom > len(plugins) {
		runFrom = len(plugins)
	}

	for i := runFrom - 1; i >= 0; i-- {
		streamPlugin, ok := plugins[i].(StreamPlugin)
		if !ok {
			continue
		}

		hookCtx, cancel := context.WithTimeout(ctx.Context, p.config.PostHookTimeout)
		originalCtx := ctx.Context
		ctx.Context = hookCtx

		hookErr := streamPlugin.PostStreamHook(ctx, err)

		ctx.Context = originalCtx
		cancel()

		if hookErr != nil {
			p.logger.Warn("PostStreamHook error",
				"plugin", plugins[i].Name(),
				"error", hookErr,
			)
		}
	}

	return err
}

// Shutdown gracefully shuts down all plugins.
func (p *Pipeline) Shutdown() error {
	if p.closed.Swap(true) {
		return ErrPipelineClosed
	}

	p.mu.RLock()
	plugins := p.plugins
	p.mu.RUnlock()

	var errs []error
	for _, plugin := range plugins {
		if err := plugin.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", plugin.Name(), err))
			p.logger.Error("plugin cleanup failed",
				"plugin", plugin.Name(),
				"error", err,
			)
		} else {
			p.logger.Debug("plugin cleaned up", "plugin", plugin.Name())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("plugin cleanup errors: %v", errs)
	}

	p.logger.Info("plugin pipeline shutdown complete", "plugins", len(plugins))
	return nil
}

// IsClosed returns whether the pipeline has been shut down.
func (p *Pipeline) IsClosed() bool {
	return p.closed.Load()
}
