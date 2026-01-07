package plugin

import "errors"

// Common plugin errors.
var (
	// ErrTooManyPlugins is returned when the maximum plugin count is exceeded.
	ErrTooManyPlugins = errors.New("too many plugins registered")

	// ErrPluginNotFound is returned when a plugin is not found.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrDuplicatePlugin is returned when a plugin with the same name already exists.
	ErrDuplicatePlugin = errors.New("duplicate plugin name")

	// ErrNilPlugin is returned when attempting to register a nil plugin.
	ErrNilPlugin = errors.New("plugin is nil")

	// ErrPipelineClosed is returned when operations are attempted on a closed pipeline.
	ErrPipelineClosed = errors.New("pipeline is closed")
)
