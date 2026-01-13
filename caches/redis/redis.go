// Package redis provides a Redis-based cache implementation.
package redis

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
	goredis "github.com/redis/go-redis/v9"

	"github.com/blueberrycongee/llmux/pkg/cache"
)

// Cache implements cache.Cache interface using Redis as backend.
type Cache struct {
	client     goredis.UniversalClient
	namespace  string
	defaultTTL time.Duration

	// Statistics
	hits   atomic.Int64
	misses atomic.Int64
	sets   atomic.Int64
	errors atomic.Int64
}

// Config holds configuration for Redis Cache.
type Config struct {
	// Single node configuration
	Addr     string `yaml:"addr"`     // Redis address (e.g., "localhost:6379")
	Password string `yaml:"password"` // Redis password
	DB       int    `yaml:"db"`       // Redis database number

	// Cluster configuration
	ClusterAddrs []string `yaml:"cluster_addrs"` // Redis cluster addresses

	// Sentinel configuration
	SentinelAddrs  []string `yaml:"sentinel_addrs"`  // Sentinel addresses
	SentinelMaster string   `yaml:"sentinel_master"` // Sentinel master name

	// Common configuration
	Namespace     string        `yaml:"namespace"`       // Key namespace prefix
	DefaultTTL    time.Duration `yaml:"default_ttl"`     // Default TTL (default: 1 hour)
	DialTimeout   time.Duration `yaml:"dial_timeout"`    // Connection timeout
	ReadTimeout   time.Duration `yaml:"read_timeout"`    // Read timeout
	WriteTimeout  time.Duration `yaml:"write_timeout"`   // Write timeout
	PoolSize      int           `yaml:"pool_size"`       // Connection pool size
	MinIdleConns  int           `yaml:"min_idle_conns"`  // Minimum idle connections
	MaxRetries    int           `yaml:"max_retries"`     // Maximum retries
	TLSEnabled    bool          `yaml:"tls_enabled"`     // Enable TLS
	TLSSkipVerify bool          `yaml:"tls_skip_verify"` // Skip TLS verification
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:         "localhost:6379",
		DB:           0,
		Namespace:    "llmux",
		DefaultTTL:   time.Hour,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
	}
}

// New creates a new Redis cache client.
func New(cfg Config) (*Cache, error) {
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = time.Hour
	}

	var client goredis.UniversalClient

	// Determine which type of client to create
	switch {
	case len(cfg.ClusterAddrs) > 0:
		// Redis Cluster
		client = goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:        cfg.ClusterAddrs,
			Password:     cfg.Password,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
		})
	case len(cfg.SentinelAddrs) > 0:
		// Redis Sentinel
		client = goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:    cfg.SentinelMaster,
			SentinelAddrs: cfg.SentinelAddrs,
			Password:      cfg.Password,
			DB:            cfg.DB,
			DialTimeout:   cfg.DialTimeout,
			ReadTimeout:   cfg.ReadTimeout,
			WriteTimeout:  cfg.WriteTimeout,
			PoolSize:      cfg.PoolSize,
			MinIdleConns:  cfg.MinIdleConns,
			MaxRetries:    cfg.MaxRetries,
		})
	default:
		// Single node
		client = goredis.NewClient(&goredis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
		})
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Cache{
		client:     client,
		namespace:  cfg.Namespace,
		defaultTTL: cfg.DefaultTTL,
	}, nil
}

// prefixKey adds namespace prefix to the key.
func (c *Cache) prefixKey(key string) string {
	if c.namespace == "" {
		return key
	}
	return c.namespace + ":" + key
}

// Get retrieves a value from Redis.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.Get(ctx, prefixedKey).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			c.misses.Add(1)
			return nil, nil
		}
		c.errors.Add(1)
		return nil, fmt.Errorf("redis get: %w", err)
	}

	c.hits.Add(1)
	return val, nil
}

// Set stores a value in Redis with TTL.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	prefixedKey := c.prefixKey(key)

	if err := c.client.Set(ctx, prefixedKey, value, ttl).Err(); err != nil {
		c.errors.Add(1)
		return fmt.Errorf("redis set: %w", err)
	}

	c.sets.Add(1)
	return nil
}

// Delete removes a key from Redis.
func (c *Cache) Delete(ctx context.Context, key string) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.Del(ctx, prefixedKey).Err(); err != nil {
		c.errors.Add(1)
		return fmt.Errorf("redis del: %w", err)
	}

	return nil
}

// SetPipeline performs batch set operations using Redis pipeline.
func (c *Cache) SetPipeline(ctx context.Context, entries []cache.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()

	for _, entry := range entries {
		ttl := entry.TTL
		if ttl <= 0 {
			ttl = c.defaultTTL
		}
		prefixedKey := c.prefixKey(entry.Key)
		pipe.Set(ctx, prefixedKey, entry.Value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		c.errors.Add(1)
		return fmt.Errorf("redis pipeline exec: %w", err)
	}

	c.sets.Add(int64(len(entries)))
	return nil
}

// GetMulti retrieves multiple keys using Redis MGET.
func (c *Cache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// Prefix all keys
	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	vals, err := c.client.MGet(ctx, prefixedKeys...).Result()
	if err != nil {
		c.errors.Add(1)
		return nil, fmt.Errorf("redis mget: %w", err)
	}

	result := make(map[string][]byte, len(keys))
	for i, val := range vals {
		if val != nil {
			switch v := val.(type) {
			case string:
				result[keys[i]] = []byte(v)
				c.hits.Add(1)
			case []byte:
				result[keys[i]] = v
				c.hits.Add(1)
			default:
				c.misses.Add(1)
			}
		} else {
			c.misses.Add(1)
		}
	}

	return result, nil
}

// Ping checks Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Stats returns cache statistics.
func (c *Cache) Stats() cache.Stats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return cache.Stats{
		Hits:    hits,
		Misses:  misses,
		Sets:    c.sets.Load(),
		Errors:  c.errors.Load(),
		HitRate: hitRate,
	}
}

// Increment atomically increments a counter in Redis.
func (c *Cache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.IncrBy(ctx, prefixedKey, delta).Result()
	if err != nil {
		c.errors.Add(1)
		return 0, fmt.Errorf("redis incrby: %w", err)
	}

	// Set TTL if key is new (TTL returns -1 for keys without expiration)
	if ttl > 0 {
		currentTTL, err := c.client.TTL(ctx, prefixedKey).Result()
		if err == nil && currentTTL < 0 {
			_ = c.client.Expire(ctx, prefixedKey, ttl)
		}
	}

	return val, nil
}

// SetNX sets a value only if the key doesn't exist (for distributed locks).
func (c *Cache) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	prefixedKey := c.prefixKey(key)

	ok, err := c.client.SetNX(ctx, prefixedKey, value, ttl).Result()
	if err != nil {
		c.errors.Add(1)
		return false, fmt.Errorf("redis setnx: %w", err)
	}

	if ok {
		c.sets.Add(1)
	}

	return ok, nil
}

// GetWithTTL retrieves a value along with its remaining TTL.
func (c *Cache) GetWithTTL(ctx context.Context, key string) ([]byte, time.Duration, error) {
	prefixedKey := c.prefixKey(key)

	pipe := c.client.Pipeline()
	getCmd := pipe.Get(ctx, prefixedKey)
	ttlCmd := pipe.TTL(ctx, prefixedKey)

	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, goredis.Nil) {
		c.errors.Add(1)
		return nil, 0, fmt.Errorf("redis pipeline: %w", err)
	}

	val, err := getCmd.Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			c.misses.Add(1)
			return nil, 0, nil
		}
		return nil, 0, err
	}

	ttl := ttlCmd.Val()
	c.hits.Add(1)

	return val, ttl, nil
}

// SetJSON stores a JSON-serializable value.
func (c *Cache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return c.Set(ctx, key, data, ttl)
}

// GetJSON retrieves and unmarshals a JSON value.
func (c *Cache) GetJSON(ctx context.Context, key string, dest any) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, dest)
}
