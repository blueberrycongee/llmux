package cache

import (
	"fmt"
	"time"

	"github.com/blueberrycongee/llmux/internal/cache/semantic"
)

// Config holds the complete cache configuration.
type Config struct {
	Type      CacheType         `yaml:"type"`      // Cache type: local, redis, dual, semantic
	Enabled   bool              `yaml:"enabled"`   // Enable/disable caching
	Namespace string            `yaml:"namespace"` // Key namespace prefix
	TTL       time.Duration     `yaml:"ttl"`       // Default TTL
	Memory    MemoryCacheConfig `yaml:"memory"`    // In-memory cache config
	Redis     RedisCacheConfig  `yaml:"redis"`     // Redis cache config
	Dual      DualCacheConfig   `yaml:"dual"`      // Dual cache config
	Semantic  semantic.Config   `yaml:"semantic"`  // Semantic cache config
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Type:      CacheTypeLocal,
		Enabled:   false,
		Namespace: "llmux",
		TTL:       time.Hour,
		Memory:    DefaultMemoryCacheConfig(),
		Redis:     DefaultRedisCacheConfig(),
		Dual:      DefaultDualCacheConfig(),
		Semantic:  semantic.DefaultConfig(),
	}
}

// NewCache creates a cache instance based on configuration.
func NewCache(cfg Config) (Cache, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	switch cfg.Type {
	case CacheTypeLocal:
		return NewMemoryCache(cfg.Memory), nil

	case CacheTypeRedis:
		redisCfg := cfg.Redis
		if cfg.Namespace != "" {
			redisCfg.Namespace = cfg.Namespace
		}
		if cfg.TTL > 0 {
			redisCfg.DefaultTTL = cfg.TTL
		}
		return NewRedisCache(redisCfg)

	case CacheTypeDual:
		// Create local cache
		local := NewMemoryCache(cfg.Memory)

		// Create Redis cache
		redisCfg := cfg.Redis
		if cfg.Namespace != "" {
			redisCfg.Namespace = cfg.Namespace
		}
		if cfg.TTL > 0 {
			redisCfg.DefaultTTL = cfg.TTL
		}
		redis, err := NewRedisCache(redisCfg)
		if err != nil {
			return nil, fmt.Errorf("create redis cache: %w", err)
		}

		// Create dual cache
		dualCfg := cfg.Dual
		if cfg.TTL > 0 {
			dualCfg.RedisTTL = cfg.TTL
		}
		return NewDualCache(local, redis, dualCfg), nil

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Type)
	}
}

// NewCacheHandler creates a complete cache handler with the given configuration.
func NewCacheHandler(cfg Config) (*Handler, error) {
	cache, err := NewCache(cfg)
	if err != nil {
		return nil, err
	}

	keyGen := NewKeyGenerator(cfg.Namespace)

	handlerCfg := HandlerConfig{
		Enabled:            cfg.Enabled,
		DefaultTTL:         cfg.TTL,
		SupportedCallTypes: []string{"completion", "acompletion"},
		MaxCacheableSize:   10 * 1024 * 1024,
	}

	return NewHandler(cache, keyGen, handlerCfg), nil
}
