// Package config provides configuration management with hot-reload support.
// It uses fsnotify to watch for file changes and atomic pointer swaps for zero-downtime updates.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete gateway configuration.
type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Providers []ProviderConfig `yaml:"providers"`
	Routing   RoutingConfig    `yaml:"routing"`
	RateLimit RateLimitConfig  `yaml:"rate_limit"`
	Logging   LoggingConfig    `yaml:"logging"`
	Metrics   MetricsConfig    `yaml:"metrics"`
	Tracing   TracingConfig    `yaml:"tracing"`
	Auth      AuthConfig       `yaml:"auth"`
	Database  DatabaseConfig   `yaml:"database"`
	Cache     CacheConfig      `yaml:"cache"`
	MCP       MCPConfig        `yaml:"mcp"`
}

// MCPConfig contains MCP (Model Context Protocol) settings.
type MCPConfig struct {
	Enabled                  bool              `yaml:"enabled"`
	Clients                  []MCPClientConfig `yaml:"clients"`
	DefaultConnectionTimeout time.Duration     `yaml:"default_connection_timeout"`
	DefaultExecutionTimeout  time.Duration     `yaml:"default_execution_timeout"`
}

// MCPClientConfig defines a single MCP client configuration.
type MCPClientConfig struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	Type              string            `yaml:"type"` // http, stdio, sse
	URL               string            `yaml:"url,omitempty"`
	Command           string            `yaml:"command,omitempty"`
	Args              []string          `yaml:"args,omitempty"`
	Envs              []string          `yaml:"envs,omitempty"`
	Headers           map[string]string `yaml:"headers,omitempty"`
	ToolsToExecute    []string          `yaml:"tools_to_execute,omitempty"`
	ConnectionTimeout time.Duration     `yaml:"connection_timeout,omitempty"`
	ExecutionTimeout  time.Duration     `yaml:"execution_timeout,omitempty"`
}

// CacheConfig contains caching settings.
type CacheConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Type      string            `yaml:"type"`      // local, redis, dual
	Namespace string            `yaml:"namespace"` // Key namespace prefix
	TTL       time.Duration     `yaml:"ttl"`       // Default TTL
	Memory    MemoryCacheConfig `yaml:"memory"`    // In-memory cache config
	Redis     RedisCacheConfig  `yaml:"redis"`     // Redis cache config
}

// MemoryCacheConfig contains in-memory cache settings.
type MemoryCacheConfig struct {
	MaxSize         int           `yaml:"max_size"`         // Maximum number of items
	DefaultTTL      time.Duration `yaml:"default_ttl"`      // Default TTL
	MaxItemSize     int           `yaml:"max_item_size"`    // Maximum size per item in bytes
	CleanupInterval time.Duration `yaml:"cleanup_interval"` // Cleanup interval
}

// RedisCacheConfig contains Redis cache settings.
type RedisCacheConfig struct {
	Addr           string        `yaml:"addr"`            // Redis address
	Password       string        `yaml:"password"`        // Redis password
	DB             int           `yaml:"db"`              // Redis database number
	ClusterAddrs   []string      `yaml:"cluster_addrs"`   // Redis cluster addresses
	SentinelAddrs  []string      `yaml:"sentinel_addrs"`  // Sentinel addresses
	SentinelMaster string        `yaml:"sentinel_master"` // Sentinel master name
	DialTimeout    time.Duration `yaml:"dial_timeout"`    // Connection timeout
	ReadTimeout    time.Duration `yaml:"read_timeout"`    // Read timeout
	WriteTimeout   time.Duration `yaml:"write_timeout"`   // Write timeout
	PoolSize       int           `yaml:"pool_size"`       // Connection pool size
	MinIdleConns   int           `yaml:"min_idle_conns"`  // Minimum idle connections
	MaxRetries     int           `yaml:"max_retries"`     // Maximum retries
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	Enabled   bool     `yaml:"enabled"`
	SkipPaths []string `yaml:"skip_paths"` // Paths to skip authentication
}

// DatabaseConfig contains PostgreSQL connection settings.
type DatabaseConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	User         string        `yaml:"user"`
	Password     string        `yaml:"password"`
	Database     string        `yaml:"database"`
	SSLMode      string        `yaml:"ssl_mode"`
	MaxOpenConns int           `yaml:"max_open_conns"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	ConnLifetime time.Duration `yaml:"conn_lifetime"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// ProviderConfig defines a single LLM provider configuration.
type ProviderConfig struct {
	Name          string            `yaml:"name"`
	Type          string            `yaml:"type"`
	APIKey        string            `yaml:"api_key"`
	BaseURL       string            `yaml:"base_url"`
	Models        []string          `yaml:"models"`
	MaxConcurrent int               `yaml:"max_concurrent"`
	Timeout       time.Duration     `yaml:"timeout"`
	Headers       map[string]string `yaml:"headers"`
}

// RoutingConfig contains routing and load balancing settings.
type RoutingConfig struct {
	DefaultProvider string        `yaml:"default_provider"`
	Strategy        string        `yaml:"strategy"` // simple-shuffle, lowest-latency, least-busy
	FallbackEnabled bool          `yaml:"fallback_enabled"`
	RetryCount      int           `yaml:"retry_count"`
	CooldownPeriod  time.Duration `yaml:"cooldown_period"`
}

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	BurstSize         int  `yaml:"burst_size"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// MetricsConfig contains Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// TracingConfig contains OpenTelemetry tracing settings.
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Endpoint    string  `yaml:"endpoint"`     // OTLP endpoint (e.g., "localhost:4317")
	ServiceName string  `yaml:"service_name"` // Service name for traces
	SampleRate  float64 `yaml:"sample_rate"`  // Sampling rate (0.0 to 1.0)
	Insecure    bool    `yaml:"insecure"`     // Use insecure connection (no TLS)
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Routing: RoutingConfig{
			Strategy:        "simple-shuffle",
			FallbackEnabled: true,
			RetryCount:      3,
			CooldownPeriod:  60 * time.Second,
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerMinute: 60,
			BurstSize:         10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		Tracing: TracingConfig{
			Enabled:     false,
			Endpoint:    "localhost:4317",
			ServiceName: "llmux",
			SampleRate:  1.0,
			Insecure:    true,
		},
		Auth: AuthConfig{
			Enabled:   false,
			SkipPaths: []string{"/health/live", "/health/ready", "/metrics"},
		},
		Database: DatabaseConfig{
			Enabled:      false,
			Host:         "localhost",
			Port:         5432,
			Database:     "llmux",
			SSLMode:      "disable",
			MaxOpenConns: 25,
			MaxIdleConns: 5,
			ConnLifetime: 5 * time.Minute,
		},
		Cache: CacheConfig{
			Enabled:   false,
			Type:      "local",
			Namespace: "llmux",
			TTL:       time.Hour,
			Memory: MemoryCacheConfig{
				MaxSize:         1000,
				DefaultTTL:      10 * time.Minute,
				MaxItemSize:     1024 * 1024,
				CleanupInterval: time.Minute,
			},
			Redis: RedisCacheConfig{
				Addr:         "localhost:6379",
				DB:           0,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				PoolSize:     10,
				MinIdleConns: 2,
				MaxRetries:   3,
			},
		},
		MCP: MCPConfig{
			Enabled:                  false,
			Clients:                  []MCPClientConfig{},
			DefaultConnectionTimeout: 30 * time.Second,
			DefaultExecutionTimeout:  60 * time.Second,
		},
	}
}

// LoadFromFile reads and parses a YAML configuration file.
// Environment variables in the format ${VAR_NAME} are expanded.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider must be configured")
	}

	for i, p := range c.Providers {
		if p.Name == "" {
			return fmt.Errorf("provider[%d]: name is required", i)
		}
		if p.Type == "" {
			return fmt.Errorf("provider[%d]: type is required", i)
		}
		if p.APIKey == "" {
			return fmt.Errorf("provider[%d] %q: api_key is required", i, p.Name)
		}
		if len(p.Models) == 0 {
			return fmt.Errorf("provider[%d] %q: at least one model must be configured", i, p.Name)
		}
		if p.Timeout < 0 {
			return fmt.Errorf("provider[%d] %q: timeout cannot be negative", i, p.Name)
		}
		if p.MaxConcurrent < 0 {
			return fmt.Errorf("provider[%d] %q: max_concurrent cannot be negative", i, p.Name)
		}
	}

	// Validate routing config
	if c.Routing.RetryCount < 0 {
		return fmt.Errorf("routing.retry_count cannot be negative")
	}
	if c.Routing.CooldownPeriod < 0 {
		return fmt.Errorf("routing.cooldown_period cannot be negative")
	}

	return nil
}
