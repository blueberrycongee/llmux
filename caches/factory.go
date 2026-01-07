// Package caches provides public cache implementations for LLMux library mode.
// It includes memory, redis, and dual-tier cache backends.
package caches

import (
	"github.com/blueberrycongee/llmux/caches/dual"
	"github.com/blueberrycongee/llmux/caches/memory"
	"github.com/blueberrycongee/llmux/caches/redis"
	"github.com/blueberrycongee/llmux/pkg/cache"
)

// Type re-exports cache types for convenience.
type Type = cache.Type

// Cache type constants.
const (
	TypeLocal = cache.TypeLocal
	TypeRedis = cache.TypeRedis
	TypeDual  = cache.TypeDual
)

// NewMemory creates a new in-memory cache with the given configuration.
func NewMemory(cfg memory.Config) *memory.Cache {
	return memory.New(cfg)
}

// NewMemoryDefault creates a new in-memory cache with default configuration.
func NewMemoryDefault() *memory.Cache {
	return memory.New(memory.DefaultConfig())
}

// NewRedis creates a new Redis cache with the given configuration.
func NewRedis(cfg redis.Config) (*redis.Cache, error) {
	return redis.New(cfg)
}

// NewRedisDefault creates a new Redis cache with default configuration.
// Returns error if Redis connection fails.
func NewRedisDefault() (*redis.Cache, error) {
	return redis.New(redis.DefaultConfig())
}

// NewDual creates a new dual-tier cache with memory (L1) and Redis (L2).
func NewDual(local *memory.Cache, remote *redis.Cache, cfg dual.Config) *dual.Cache {
	return dual.New(local, remote, cfg)
}

// NewDualDefault creates a new dual-tier cache with default configurations.
// Returns error if Redis connection fails.
func NewDualDefault() (*dual.Cache, error) {
	local := memory.New(memory.DefaultConfig())
	remote, err := redis.New(redis.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return dual.New(local, remote, dual.DefaultConfig()), nil
}

// Re-export config types for convenience.
type (
	MemoryConfig = memory.Config
	RedisConfig  = redis.Config
	DualConfig   = dual.Config
)

// Re-export default config functions.
var (
	DefaultMemoryConfig = memory.DefaultConfig
	DefaultRedisConfig  = redis.DefaultConfig
	DefaultDualConfig   = dual.DefaultConfig
)
