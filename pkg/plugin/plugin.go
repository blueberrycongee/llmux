// Package plugin provides the public API for the LLMux plugin system.
// It re-exports core types and interfaces from internal/plugin to allow
// external developers to create custom plugins.
package plugin

import (
	"github.com/blueberrycongee/llmux/internal/plugin"
)

// Re-export core interfaces and types.
type (
	// Plugin defines the interface for LLMux plugins.
	// Plugins can intercept and modify requests/responses at various lifecycle points.
	Plugin = plugin.Plugin

	// StreamPlugin extends Plugin to support streaming requests.
	StreamPlugin = plugin.StreamPlugin

	// Context provides execution context for plugins.
	// It allows plugins to share data and access request metadata.
	Context = plugin.Context

	// ShortCircuit represents a plugin's decision to short-circuit the request.
	// It can return a response immediately (e.g., cache hit) or an error (e.g., rate limit).
	ShortCircuit = plugin.ShortCircuit

	// StreamShortCircuit is the streaming equivalent of ShortCircuit.
	StreamShortCircuit = plugin.StreamShortCircuit

	// PipelineConfig holds configuration for the plugin pipeline.
	PipelineConfig = plugin.PipelineConfig
)

// Re-export common errors.
var (
	ErrTooManyPlugins  = plugin.ErrTooManyPlugins
	ErrPluginNotFound  = plugin.ErrPluginNotFound
	ErrDuplicatePlugin = plugin.ErrDuplicatePlugin
	ErrNilPlugin       = plugin.ErrNilPlugin
	ErrPipelineClosed  = plugin.ErrPipelineClosed
)

// Re-export helper functions.
var (
	// DefaultPipelineConfig returns the default configuration for the pipeline.
	DefaultPipelineConfig = plugin.DefaultPipelineConfig

	// NewContext creates a new plugin context.
	NewContext = plugin.NewContext
)
