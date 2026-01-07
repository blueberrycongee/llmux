package mcp

import (
	"fmt"
	"time"
)

// Config is the top-level MCP configuration.
type Config struct {
	// Enabled controls whether MCP integration is active.
	Enabled bool `yaml:"enabled"`

	// Clients defines the MCP client configurations.
	Clients []ClientConfig `yaml:"clients"`

	// DefaultConnectionTimeout is the default timeout for establishing connections.
	DefaultConnectionTimeout time.Duration `yaml:"default_connection_timeout"`

	// DefaultExecutionTimeout is the default timeout for tool execution.
	DefaultExecutionTimeout time.Duration `yaml:"default_execution_timeout"`
}

// ClientConfig defines a single MCP client configuration.
type ClientConfig struct {
	// ID is the unique identifier for this client.
	ID string `yaml:"id" json:"id"`

	// Name is the human-readable name for this client.
	Name string `yaml:"name" json:"name"`

	// Type is the connection type (http, stdio, sse, inprocess).
	Type ConnectionType `yaml:"type" json:"type"`

	// URL is the endpoint URL for HTTP/SSE connections.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Command is the command to execute for STDIO connections.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// Args are the command arguments for STDIO connections.
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Envs are required environment variables for STDIO connections.
	Envs []string `yaml:"envs,omitempty" json:"envs,omitempty"`

	// Headers are HTTP headers for HTTP/SSE connections.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// ToolsToExecute defines which tools to expose from this client.
	// Semantics:
	//   - nil or omitted: no tools exposed (safe default)
	//   - []: no tools exposed
	//   - ["*"]: all tools exposed
	//   - ["tool1", "tool2"]: only specified tools exposed
	ToolsToExecute []string `yaml:"tools_to_execute,omitempty" json:"tools_to_execute,omitempty"`

	// ConnectionTimeout overrides the default connection timeout.
	ConnectionTimeout time.Duration `yaml:"connection_timeout,omitempty" json:"connection_timeout,omitempty"`

	// ExecutionTimeout overrides the default execution timeout.
	ExecutionTimeout time.Duration `yaml:"execution_timeout,omitempty" json:"execution_timeout,omitempty"`
}

// DefaultConfig returns the default MCP configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:                  false,
		Clients:                  []ClientConfig{},
		DefaultConnectionTimeout: DefaultConnectionTimeout,
		DefaultExecutionTimeout:  DefaultExecutionTimeout,
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	seen := make(map[string]bool)
	for i := range c.Clients {
		if err := c.Clients[i].Validate(); err != nil {
			return fmt.Errorf("clients[%d]: %w", i, err)
		}
		if seen[c.Clients[i].ID] {
			return fmt.Errorf("clients[%d]: duplicate id %q", i, c.Clients[i].ID)
		}
		seen[c.Clients[i].ID] = true
	}

	return nil
}

// Validate checks the client configuration for errors.
func (c *ClientConfig) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}

	switch c.Type {
	case ConnectionTypeHTTP, ConnectionTypeSSE:
		if c.URL == "" {
			return fmt.Errorf("url is required for %s connection", c.Type)
		}
	case ConnectionTypeSTDIO:
		if c.Command == "" {
			return fmt.Errorf("command is required for stdio connection")
		}
	case ConnectionTypeInProcess:
		// InProcess connections are configured programmatically
	case "":
		return fmt.Errorf("type is required")
	default:
		return fmt.Errorf("unknown connection type: %s", c.Type)
	}

	if c.ConnectionTimeout < 0 {
		return fmt.Errorf("connection_timeout cannot be negative")
	}
	if c.ExecutionTimeout < 0 {
		return fmt.Errorf("execution_timeout cannot be negative")
	}

	return nil
}

// GetConnectionTimeout returns the effective connection timeout.
func (c *ClientConfig) GetConnectionTimeout(defaultTimeout time.Duration) time.Duration {
	if c.ConnectionTimeout > 0 {
		return c.ConnectionTimeout
	}
	return defaultTimeout
}

// GetExecutionTimeout returns the effective execution timeout.
func (c *ClientConfig) GetExecutionTimeout(defaultTimeout time.Duration) time.Duration {
	if c.ExecutionTimeout > 0 {
		return c.ExecutionTimeout
	}
	return defaultTimeout
}
