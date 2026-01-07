// Package dual provides a two-tier cache with in-memory (L1) and Redis (L2).
package dual

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blueberrycongee/llmux/caches/memory"
	"github.com/blueberrycongee/llmux/caches/redis"
	"github.com/blueberrycongee/llmux/pkg/cache"
)

// Cache implements a two-tier cache with in-memory (L1) and Redis (L2).
// Writes go to both caches, reads check L1 first then L2 with backfill.
type Cache struct {
	local  *memory.Cache
	remote *redis.Cache
	config Config

	// Throttling for batch Redis queries
	mu                  sync.RWMutex
	lastRedisAccessTime map[string]time.Time

	// Statistics
	localHits atomic.Int64
	redisHits atomic.Int64
	misses    atomic.Int64
	backfills atomic.Int64
}

// Config holds configuration for dual Cache.
type Config struct {
	LocalTTL           time.Duration // TTL for local cache (default: 5 minutes)
	RedisTTL           time.Duration // TTL for Redis cache (default: 1 hour)
	BatchThrottleTime  time.Duration // Throttle repeated Redis queries (default: 10 seconds)
	MaxThrottleEntries int           // Max entries in throttle map (default: 10000)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		LocalTTL:           5 * time.Minute,
		RedisTTL:           time.Hour,
		BatchThrottleTime:  10 * time.Second,
		MaxThrottleEntries: 10000,
	}
}

// New creates a new dual-tier cache.
func New(local *memory.Cache, remote *redis.Cache, cfg Config) *Cache {
	if cfg.LocalTTL <= 0 {
		cfg.LocalTTL = 5 * time.Minute
	}
	if cfg.RedisTTL <= 0 {
		cfg.RedisTTL = time.Hour
	}
	if cfg.BatchThrottleTime <= 0 {
		cfg.BatchThrottleTime = 10 * time.Second
	}
	if cfg.MaxThrottleEntries <= 0 {
		cfg.MaxThrottleEntries = 10000
	}

	return &Cache{
		local:               local,
		remote:              remote,
		config:              cfg,
		lastRedisAccessTime: make(map[string]time.Time),
	}
}

// Get retrieves a value, checking local cache first, then Redis.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	// L1: Check local cache first
	if val, err := c.local.Get(ctx, key); err == nil && val != nil {
		c.localHits.Add(1)
		return val, nil
	}

	// L2: Check Redis
	if c.remote != nil {
		val, err := c.remote.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if val != nil {
			c.redisHits.Add(1)
			// Backfill local cache - best-effort, failure doesn't affect main flow
			_ = c.local.Set(ctx, key, val, c.config.LocalTTL) //nolint:errcheck // backfill is best-effort
			c.backfills.Add(1)
			return val, nil
		}
	}

	c.misses.Add(1)
	return nil, nil
}

// Set stores a value in both caches.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Determine TTLs
	localTTL := c.config.LocalTTL
	redisTTL := ttl
	if redisTTL <= 0 {
		redisTTL = c.config.RedisTTL
	}

	// Write to local cache
	if err := c.local.Set(ctx, key, value, localTTL); err != nil {
		return err
	}

	// Write to Redis
	if c.remote != nil {
		if err := c.remote.Set(ctx, key, value, redisTTL); err != nil {
			return err
		}
	}

	return nil
}

// SetLocalOnly stores a value only in the local cache.
func (c *Cache) SetLocalOnly(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.config.LocalTTL
	}
	return c.local.Set(ctx, key, value, ttl)
}

// Delete removes a key from both caches.
func (c *Cache) Delete(ctx context.Context, key string) error {
	_ = c.local.Delete(ctx, key) //nolint:errcheck // best-effort local delete
	if c.remote != nil {
		return c.remote.Delete(ctx, key)
	}
	return nil
}

// SetPipeline performs batch set operations on both caches.
func (c *Cache) SetPipeline(ctx context.Context, entries []cache.Entry) error {
	// Adjust TTLs for local cache
	localEntries := make([]cache.Entry, len(entries))
	for i, e := range entries {
		localEntries[i] = cache.Entry{
			Key:   e.Key,
			Value: e.Value,
			TTL:   c.config.LocalTTL,
		}
	}

	// Write to local cache
	if err := c.local.SetPipeline(ctx, localEntries); err != nil {
		return err
	}

	// Write to Redis
	if c.remote != nil {
		return c.remote.SetPipeline(ctx, entries)
	}

	return nil
}

// GetMulti retrieves multiple keys with throttling for Redis queries.
func (c *Cache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(keys))

	// L1: Check local cache first
	localResults, err := c.local.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	// Track which keys need Redis lookup
	var missingKeys []string
	for _, key := range keys {
		if val, ok := localResults[key]; ok {
			result[key] = val
			c.localHits.Add(1)
		} else {
			missingKeys = append(missingKeys, key)
		}
	}

	// L2: Check Redis for missing keys (with throttling)
	if c.remote != nil && len(missingKeys) > 0 {
		keysToQuery := c.filterThrottledKeys(missingKeys)

		if len(keysToQuery) > 0 {
			redisResults, err := c.remote.GetMulti(ctx, keysToQuery)
			if err != nil {
				return result, err // Return partial results
			}

			// Update throttle times and backfill local cache
			now := time.Now()
			c.mu.Lock()
			for _, key := range keysToQuery {
				c.lastRedisAccessTime[key] = now
			}
			// Cleanup old entries if map is too large
			if len(c.lastRedisAccessTime) > c.config.MaxThrottleEntries {
				c.cleanupThrottleMap(now)
			}
			c.mu.Unlock()

			// Process Redis results
			for key, val := range redisResults {
				result[key] = val
				c.redisHits.Add(1)
				// Backfill local cache - best-effort, failure doesn't affect main flow
				_ = c.local.Set(ctx, key, val, c.config.LocalTTL) //nolint:errcheck // backfill is best-effort
				c.backfills.Add(1)
			}

			// Count misses for keys not found in Redis
			for _, key := range keysToQuery {
				if _, ok := redisResults[key]; !ok {
					c.misses.Add(1)
				}
			}
		}
	}

	return result, nil
}

// filterThrottledKeys returns keys that haven't been queried recently.
func (c *Cache) filterThrottledKeys(keys []string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var result []string

	for _, key := range keys {
		lastAccess, ok := c.lastRedisAccessTime[key]
		if !ok || now.Sub(lastAccess) >= c.config.BatchThrottleTime {
			result = append(result, key)
		}
	}

	return result
}

// cleanupThrottleMap removes old entries from the throttle map.
func (c *Cache) cleanupThrottleMap(now time.Time) {
	threshold := now.Add(-c.config.BatchThrottleTime * 2)
	for key, t := range c.lastRedisAccessTime {
		if t.Before(threshold) {
			delete(c.lastRedisAccessTime, key)
		}
	}
}

// Ping checks both cache backends.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.local.Ping(ctx); err != nil {
		return err
	}
	if c.remote != nil {
		return c.remote.Ping(ctx)
	}
	return nil
}

// Close closes both cache backends.
func (c *Cache) Close() error {
	_ = c.local.Close()
	if c.remote != nil {
		return c.remote.Close()
	}
	return nil
}

// Stats returns combined cache statistics.
func (c *Cache) Stats() cache.Stats {
	localStats := c.local.Stats()
	var redisStats cache.Stats
	if c.remote != nil {
		redisStats = c.remote.Stats()
	}

	totalHits := c.localHits.Load() + c.redisHits.Load()
	totalMisses := c.misses.Load()
	total := totalHits + totalMisses

	var hitRate float64
	if total > 0 {
		hitRate = float64(totalHits) / float64(total)
	}

	return cache.Stats{
		Hits:    totalHits,
		Misses:  totalMisses,
		Sets:    localStats.Sets + redisStats.Sets,
		Errors:  redisStats.Errors,
		HitRate: hitRate,
	}
}

// DetailedStats holds detailed statistics for both tiers.
type DetailedStats struct {
	LocalHits  int64       `json:"local_hits"`
	RedisHits  int64       `json:"redis_hits"`
	Misses     int64       `json:"misses"`
	Backfills  int64       `json:"backfills"`
	HitRate    float64     `json:"hit_rate"`
	LocalStats cache.Stats `json:"local_stats"`
	RedisStats cache.Stats `json:"redis_stats"`
}

// GetDetailedStats returns detailed statistics for both cache tiers.
func (c *Cache) GetDetailedStats() DetailedStats {
	localHits := c.localHits.Load()
	redisHits := c.redisHits.Load()
	misses := c.misses.Load()
	total := localHits + redisHits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(localHits+redisHits) / float64(total)
	}

	stats := DetailedStats{
		LocalHits:  localHits,
		RedisHits:  redisHits,
		Misses:     misses,
		Backfills:  c.backfills.Load(),
		HitRate:    hitRate,
		LocalStats: c.local.Stats(),
	}

	if c.remote != nil {
		stats.RedisStats = c.remote.Stats()
	}

	return stats
}

// Flush clears both caches.
func (c *Cache) Flush() {
	c.local.Flush()
	// Note: Redis flush is intentionally not implemented for safety
}
