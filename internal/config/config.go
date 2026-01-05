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
	Level  string `yaml:"level"` // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// MetricsConfig contains Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
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
	}

	return nil
}
