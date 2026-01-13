// Package config provides configuration management with hot-reload support.
// It uses fsnotify to watch for file changes and atomic pointer swaps for zero-downtime updates.
package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/blueberrycongee/llmux/internal/observability"
)

// Config represents the complete gateway configuration.
type Config struct {
	Server        ServerConfig                      `yaml:"server"`
	Deployment    DeploymentConfig                  `yaml:"deployment"`
	Providers     []ProviderConfig                  `yaml:"providers"`
	Routing       RoutingConfig                     `yaml:"routing"`
	Stream        StreamConfig                      `yaml:"stream"`
	RateLimit     RateLimitConfig                   `yaml:"rate_limit"`
	Governance    GovernanceConfig                  `yaml:"governance"`
	Logging       LoggingConfig                     `yaml:"logging"`
	Metrics       MetricsConfig                     `yaml:"metrics"`
	Tracing       TracingConfig                     `yaml:"tracing"`
	Observability observability.ObservabilityConfig `yaml:"observability"`
	CORS          CORSConfig                        `yaml:"cors"`
	Auth          AuthConfig                        `yaml:"auth"`
	Database      DatabaseConfig                    `yaml:"database"`
	Cache         CacheConfig                       `yaml:"cache"`
	HealthCheck   HealthCheckConfig                 `yaml:"healthcheck"`
	MCP           MCPConfig                         `yaml:"mcp"`
	Vault         VaultConfig                       `yaml:"vault"`
	PricingFile   string                            `yaml:"pricing_file"`
}

// DeploymentConfig contains deployment mode settings.
// Modes: standalone, distributed, development.
type DeploymentConfig struct {
	Mode string `yaml:"mode"`
}

// VaultConfig contains HashiCorp Vault settings.
type VaultConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Address    string `yaml:"address"`
	AuthMethod string `yaml:"auth_method"` // "approle", "cert"

	// Auth Params
	RoleID   string `yaml:"role_id"`
	SecretID string `yaml:"secret_id"`

	// TLS Config
	CACert     string `yaml:"ca_cert"`
	ClientCert string `yaml:"client_cert"`
	ClientKey  string `yaml:"client_key"`
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

// HealthCheckConfig contains proactive health probe settings.
type HealthCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
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
// AuthConfig contains authentication settings.
type AuthConfig struct {
	Enabled                bool          `yaml:"enabled"`
	SkipPaths              []string      `yaml:"skip_paths"` // Paths to skip authentication
	LastUsedUpdateInterval time.Duration `yaml:"last_used_update_interval"`
	OIDC                   OIDCConfig    `yaml:"oidc"` // OIDC configuration
}

// OIDCConfig contains OIDC provider settings.
// This configuration aligns with LiteLLM's advanced SSO features.
type OIDCConfig struct {
	IssuerURL    string       `yaml:"issuer_url"`
	ClientID     string       `yaml:"client_id"`
	ClientSecret string       `yaml:"client_secret"`
	ClaimMapping ClaimMapping `yaml:"claim_mapping"`

	// User provisioning settings
	UserIDUpsert           bool   `yaml:"user_id_upsert"`            // Auto-create users on SSO login
	TeamIDUpsert           bool   `yaml:"team_id_upsert"`            // Auto-create teams on SSO login
	UserAllowedEmailDomain string `yaml:"user_allowed_email_domain"` // Restrict to specific email domain

	// OIDC UserInfo endpoint settings
	OIDCUserInfoEnabled  bool  `yaml:"oidc_userinfo_enabled"`   // Enable fetching additional claims from userinfo endpoint
	OIDCUserInfoCacheTTL int64 `yaml:"oidc_userinfo_cache_ttl"` // Cache TTL in seconds (default: 300)

	// User-Team Synchronization settings (LiteLLM sync_user_role_and_teams compatibility)
	UserTeamSync UserTeamSyncConfig `yaml:"user_team_sync"`
}

// UserTeamSyncConfig contains configuration for user-team synchronization.
// This setting enables automatic user and team management based on JWT claims.
type UserTeamSyncConfig struct {
	Enabled                 bool   `yaml:"enabled"`                    // Enable automatic sync on SSO login
	AutoCreateUsers         bool   `yaml:"auto_create_users"`          // Create users if they don't exist
	AutoCreateTeams         bool   `yaml:"auto_create_teams"`          // Create teams if they don't exist
	RemoveFromUnlistedTeams bool   `yaml:"remove_from_unlisted_teams"` // Remove user from teams not in JWT
	SyncUserRole            bool   `yaml:"sync_user_role"`             // Sync user role from JWT
	DefaultRole             string `yaml:"default_role"`               // Default role for new users
	DefaultOrganizationID   string `yaml:"default_organization_id"`    // Default organization for new users
}

// ClaimMapping defines rules for mapping OIDC claims to LLMux roles, teams, and organizations.
// Supports LiteLLM-compatible hierarchical role priority.
type ClaimMapping struct {
	// Role mapping
	RoleClaim string            `yaml:"role_claim"` // JWT claim for role extraction (e.g. "groups", "roles")
	Roles     map[string]string `yaml:"roles"`      // Claim value to role mapping (e.g. "admin-group": "proxy_admin")

	// Role hierarchy for priority-based role assignment
	// When multiple roles match, the highest priority role is assigned.
	// Order: proxy_admin > proxy_admin_viewer > org_admin > internal_user > internal_user_viewer
	UseRoleHierarchy bool `yaml:"use_role_hierarchy"`

	// Team mapping from JWT claims
	TeamIDJWTField  string            `yaml:"team_id_jwt_field"`  // Single team ID field (e.g. "team_id")
	TeamIDsJWTField string            `yaml:"team_ids_jwt_field"` // Multiple team IDs field (e.g. "team_ids")
	TeamAliasMap    map[string]string `yaml:"team_alias_map"`     // JWT team alias to internal team ID mapping

	// Organization mapping from JWT claims
	OrgIDJWTField string            `yaml:"org_id_jwt_field"` // Organization ID field (e.g. "org_id")
	OrgAliasMap   map[string]string `yaml:"org_alias_map"`    // JWT org alias to internal org ID mapping

	// User ID mapping
	UserIDJWTField    string `yaml:"user_id_jwt_field"`    // Custom user ID field (default: "sub")
	UserEmailJWTField string `yaml:"user_email_jwt_field"` // Email field (default: "email")

	// End user tracking (for downstream customer identification)
	EndUserIDJWTField string `yaml:"end_user_id_jwt_field"` // End user ID field for tracking

	// Default values
	DefaultRole   string `yaml:"default_role"`    // Default role if no mapping matches
	DefaultTeamID string `yaml:"default_team_id"` // Default team if no team claim found
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
	AdminPort    int           `yaml:"admin_port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// StreamConfig contains stream-specific behavior.
type StreamConfig struct {
	RecoveryMode string `yaml:"recovery_mode"` // off, append, retry
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
	Strategy        string        `yaml:"strategy"` // round-robin, simple-shuffle, lowest-latency, least-busy, lowest-tpm-rpm, lowest-cost, tag-based
	FallbackEnabled bool          `yaml:"fallback_enabled"`
	RetryCount      int           `yaml:"retry_count"`
	RetryBackoff    time.Duration `yaml:"retry_backoff"`
	RetryMaxBackoff time.Duration `yaml:"retry_max_backoff"`
	RetryJitter     float64       `yaml:"retry_jitter"`
	CooldownPeriod  time.Duration `yaml:"cooldown_period"`
	Distributed     bool          `yaml:"distributed"` // Enable Redis-backed distributed routing stats
}

// RateLimitConfig defines rate limiting parameters.
type RateLimitConfig struct {
	Enabled           bool          `yaml:"enabled"`
	RequestsPerMinute int64         `yaml:"requests_per_minute"` // RPM limit
	TokensPerMinute   int64         `yaml:"tokens_per_minute"`   // TPM limit
	BurstSize         int           `yaml:"burst_size"`
	WindowSize        time.Duration `yaml:"window_size"`         // Sliding window duration (default: 1m)
	KeyStrategy       string        `yaml:"key_strategy"`        // api_key, user, model, api_key_model
	FailOpen          bool          `yaml:"fail_open"`           // Allow requests when limiter backend fails
	TrustedProxyCIDRs []string      `yaml:"trusted_proxy_cidrs"` // Trusted proxies for forwarded headers

	// Distributed rate limiting (Redis-backed)
	Distributed bool `yaml:"distributed"` // Enable Redis-backed distributed rate limiting
}

// GovernanceConfig defines governance engine behavior.
type GovernanceConfig struct {
	Enabled           bool          `yaml:"enabled"`
	AsyncAccounting   bool          `yaml:"async_accounting"`
	IdempotencyWindow time.Duration `yaml:"idempotency_window"`
	AuditEnabled      bool          `yaml:"audit_enabled"`
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

// CORSConfig defines cross-origin settings for the gateway.
type CORSConfig struct {
	Enabled           bool          `yaml:"enabled"`
	AllowAllOrigins   bool          `yaml:"allow_all_origins"`
	AllowCredentials  bool          `yaml:"allow_credentials"`
	AllowMethods      []string      `yaml:"allow_methods"`
	AllowHeaders      []string      `yaml:"allow_headers"`
	ExposeHeaders     []string      `yaml:"expose_headers"`
	MaxAge            time.Duration `yaml:"max_age"`
	DataOrigins       CORSOrigins   `yaml:"data_origins"`
	AdminOrigins      CORSOrigins   `yaml:"admin_origins"`
	AdminPathPrefixes []string      `yaml:"admin_path_prefixes"`
}

// CORSOrigins contains allowlist/denylist origins.
type CORSOrigins struct {
	Allowlist []string `yaml:"allowlist"`
	Denylist  []string `yaml:"denylist"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			AdminPort:    0,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Deployment: DeploymentConfig{
			Mode: "standalone",
		},
		Routing: RoutingConfig{
			Strategy:        "simple-shuffle",
			FallbackEnabled: true,
			RetryCount:      3,
			RetryBackoff:    100 * time.Millisecond,
			RetryMaxBackoff: 5 * time.Second,
			RetryJitter:     0.2,
			CooldownPeriod:  60 * time.Second,
		},
		Stream: StreamConfig{
			RecoveryMode: "retry",
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerMinute: 60,
			TokensPerMinute:   100000,
			BurstSize:         10,
			WindowSize:        time.Minute,
			KeyStrategy:       "api_key",
			FailOpen:          true,
			Distributed:       false,
			TrustedProxyCIDRs: []string{},
		},
		Governance: GovernanceConfig{
			Enabled:           true,
			AsyncAccounting:   true,
			IdempotencyWindow: 10 * time.Minute,
			AuditEnabled:      true,
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
		Observability: observability.DefaultObservabilityConfig(),
		CORS: CORSConfig{
			Enabled:          false,
			AllowAllOrigins:  false,
			AllowCredentials: false,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With"},
			ExposeHeaders:    []string{},
			MaxAge:           10 * time.Minute,
			DataOrigins:      CORSOrigins{},
			AdminOrigins:     CORSOrigins{},
			AdminPathPrefixes: []string{
				"/key/",
				"/team/",
				"/user/",
				"/organization/",
				"/spend/",
				"/audit/",
				"/global/",
				"/invitation/",
				"/control/",
				"/metrics",
			},
		},
		Auth: AuthConfig{
			Enabled:                false,
			SkipPaths:              []string{"/health/live", "/health/ready", "/metrics"},
			LastUsedUpdateInterval: time.Minute,
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
		HealthCheck: HealthCheckConfig{
			Enabled:  false,
			Interval: 30 * time.Second,
			Timeout:  10 * time.Second,
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
	mode, err := normalizeDeploymentMode(c.Deployment.Mode)
	if err != nil {
		return err
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Server.AdminPort != 0 {
		if c.Server.AdminPort <= 0 || c.Server.AdminPort > 65535 {
			return fmt.Errorf("invalid admin port: %d", c.Server.AdminPort)
		}
		if c.Server.AdminPort == c.Server.Port {
			return fmt.Errorf("admin port must differ from server port: %d", c.Server.AdminPort)
		}
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
	if c.Routing.RetryBackoff < 0 {
		return fmt.Errorf("routing.retry_backoff cannot be negative")
	}
	if c.Routing.RetryMaxBackoff < 0 {
		return fmt.Errorf("routing.retry_max_backoff cannot be negative")
	}
	if c.Routing.RetryJitter < 0 || c.Routing.RetryJitter > 1 {
		return fmt.Errorf("routing.retry_jitter must be between 0 and 1")
	}
	if c.Routing.CooldownPeriod < 0 {
		return fmt.Errorf("routing.cooldown_period cannot be negative")
	}
	if c.HealthCheck.Interval < 0 {
		return fmt.Errorf("healthcheck.interval cannot be negative")
	}
	if c.HealthCheck.Timeout < 0 {
		return fmt.Errorf("healthcheck.timeout cannot be negative")
	}
	switch c.Stream.RecoveryMode {
	case "", "off", "append", "retry":
	default:
		return fmt.Errorf("stream.recovery_mode must be one of: off, append, retry")
	}

	if c.CORS.MaxAge < 0 {
		return fmt.Errorf("cors.max_age cannot be negative")
	}
	if c.Governance.IdempotencyWindow < 0 {
		return fmt.Errorf("governance.idempotency_window cannot be negative")
	}
	if !c.CORS.AllowAllOrigins {
		if containsWildcard(c.CORS.DataOrigins.Allowlist) {
			return fmt.Errorf("cors.data_origins.allowlist cannot include wildcard when allow_all_origins is false")
		}
		if containsWildcard(c.CORS.AdminOrigins.Allowlist) {
			return fmt.Errorf("cors.admin_origins.allowlist cannot include wildcard when allow_all_origins is false")
		}
	}
	for i, value := range c.RateLimit.TrustedProxyCIDRs {
		if !isValidIPOrCIDR(value) {
			return fmt.Errorf("rate_limit.trusted_proxy_cidrs[%d] must be a valid IP or CIDR", i)
		}
	}

	if c.Database.Enabled {
		if c.Database.Host == "" {
			return fmt.Errorf("database.host is required when database is enabled")
		}
		if c.Database.Port <= 0 || c.Database.Port > 65535 {
			return fmt.Errorf("database.port must be between 1 and 65535")
		}
		if c.Database.User == "" {
			return fmt.Errorf("database.user is required when database is enabled")
		}
		if c.Database.Database == "" {
			return fmt.Errorf("database.database is required when database is enabled")
		}
		if c.Database.SSLMode == "" {
			return fmt.Errorf("database.ssl_mode is required when database is enabled")
		}
		if c.Database.MaxOpenConns < 0 {
			return fmt.Errorf("database.max_open_conns cannot be negative")
		}
		if c.Database.MaxIdleConns < 0 {
			return fmt.Errorf("database.max_idle_conns cannot be negative")
		}
		if c.Database.ConnLifetime < 0 {
			return fmt.Errorf("database.conn_lifetime cannot be negative")
		}
	}

	if mode == "distributed" {
		if !c.Database.Enabled {
			return fmt.Errorf("deployment.mode=distributed requires database.enabled=true for auth and usage storage")
		}
		if !c.Routing.Distributed {
			return fmt.Errorf("deployment.mode=distributed requires routing.distributed=true for shared routing stats")
		}
		if c.Routing.Distributed && !hasRedisConfig(c.Cache.Redis) {
			return fmt.Errorf("deployment.mode=distributed requires cache.redis.addr or cache.redis.cluster_addrs for routing stats")
		}
		if c.RateLimit.Enabled && !c.RateLimit.Distributed {
			return fmt.Errorf("deployment.mode=distributed requires rate_limit.distributed=true when rate_limit.enabled")
		}
		if c.RateLimit.Enabled && c.RateLimit.Distributed && !hasRedisConfig(c.Cache.Redis) {
			return fmt.Errorf("deployment.mode=distributed requires cache.redis.addr or cache.redis.cluster_addrs for rate limiting")
		}
	}

	return nil
}

func normalizeDeploymentMode(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return "standalone", nil
	}
	switch normalized {
	case "standalone", "distributed", "development":
		return normalized, nil
	default:
		return "", fmt.Errorf("deployment.mode must be one of: standalone, distributed, development")
	}
}

func hasRedisConfig(cfg RedisCacheConfig) bool {
	return cfg.Addr != "" || len(cfg.ClusterAddrs) > 0
}

func containsWildcard(values []string) bool {
	for _, value := range values {
		if value == "*" {
			return true
		}
	}
	return false
}

func isValidIPOrCIDR(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "/") {
		_, _, err := net.ParseCIDR(value)
		return err == nil
	}
	return net.ParseIP(value) != nil
}
