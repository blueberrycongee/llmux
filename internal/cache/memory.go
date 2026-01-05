package cache

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryCache implements an in-memory cache with LRU + TTL eviction.
// It uses a min-heap for efficient TTL-based expiration.
type MemoryCache struct {
	mu sync.RWMutex

	// Core storage
	data map[string]*memoryCacheEntry
	ttls map[string]int64 // key -> expiration timestamp (unix nano)

	// Expiration heap (min-heap by expiration time)
	expirationHeap expirationHeap

	// Configuration
	maxSize       int
	defaultTTL    time.Duration
	maxItemSize   int // Maximum size per item in bytes
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	// Statistics
	hits   atomic.Int64
	misses atomic.Int64
	sets   atomic.Int64
}

type memoryCacheEntry struct {
	value      []byte
	expiration int64 // Unix nano timestamp
}

// expirationEntry represents an entry in the expiration heap.
type expirationEntry struct {
	key        string
	expiration int64
	index      int // Index in the heap
}

// expirationHeap implements heap.Interface for TTL-based eviction.
type expirationHeap []*expirationEntry

func (h expirationHeap) Len() int           { return len(h) }
func (h expirationHeap) Less(i, j int) bool { return h[i].expiration < h[j].expiration }
func (h expirationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *expirationHeap) Push(x any) {
	n := len(*h)
	entry, ok := x.(*expirationEntry)
	if !ok {
		return
	}
	entry.index = n
	*h = append(*h, entry)
}

func (h *expirationHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil // avoid memory leak
	entry.index = -1
	*h = old[0 : n-1]
	return entry
}

// MemoryCacheConfig holds configuration for MemoryCache.
type MemoryCacheConfig struct {
	MaxSize         int           // Maximum number of items (default: 1000)
	DefaultTTL      time.Duration // Default TTL (default: 10 minutes)
	MaxItemSize     int           // Maximum size per item in bytes (default: 1MB)
	CleanupInterval time.Duration // Cleanup interval (default: 1 minute)
}

// DefaultMemoryCacheConfig returns sensible defaults.
func DefaultMemoryCacheConfig() MemoryCacheConfig {
	return MemoryCacheConfig{
		MaxSize:         1000,
		DefaultTTL:      10 * time.Minute,
		MaxItemSize:     1024 * 1024, // 1MB
		CleanupInterval: time.Minute,
	}
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(cfg MemoryCacheConfig) *MemoryCache {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1000
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 10 * time.Minute
	}
	if cfg.MaxItemSize <= 0 {
		cfg.MaxItemSize = 1024 * 1024
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = time.Minute
	}

	c := &MemoryCache{
		data:           make(map[string]*memoryCacheEntry),
		ttls:           make(map[string]int64),
		expirationHeap: make(expirationHeap, 0),
		maxSize:        cfg.MaxSize,
		defaultTTL:     cfg.DefaultTTL,
		maxItemSize:    cfg.MaxItemSize,
		stopCleanup:    make(chan struct{}),
	}

	heap.Init(&c.expirationHeap)

	// Start background cleanup
	c.cleanupTicker = time.NewTicker(cfg.CleanupInterval)
	go c.cleanupLoop()

	return c
}

// cleanupLoop periodically removes expired entries.
func (c *MemoryCache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.evictExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// evictExpired removes all expired entries.
func (c *MemoryCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	for c.expirationHeap.Len() > 0 {
		entry := c.expirationHeap[0]

		// Check if entry is outdated (key was updated)
		if storedExp, ok := c.ttls[entry.key]; !ok || storedExp != entry.expiration {
			heap.Pop(&c.expirationHeap)
			continue
		}

		// Check if expired
		if entry.expiration <= now {
			heap.Pop(&c.expirationHeap)
			delete(c.data, entry.key)
			delete(c.ttls, entry.key)
		} else {
			break // Heap is sorted, no more expired entries
		}
	}
}

// evictIfNeeded removes entries if cache is at capacity.
func (c *MemoryCache) evictIfNeeded() {
	now := time.Now().UnixNano()

	// First, remove expired entries
	for c.expirationHeap.Len() > 0 && len(c.data) >= c.maxSize {
		entry := c.expirationHeap[0]

		// Skip outdated entries
		if storedExp, ok := c.ttls[entry.key]; !ok || storedExp != entry.expiration {
			heap.Pop(&c.expirationHeap)
			continue
		}

		// Remove if expired or if we need space
		if entry.expiration <= now || len(c.data) >= c.maxSize {
			heap.Pop(&c.expirationHeap)
			delete(c.data, entry.key)
			delete(c.ttls, entry.key)
		} else {
			break
		}
	}
}

// Get retrieves a value from the cache.
func (c *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		return nil, nil
	}

	// Check expiration
	if entry.expiration > 0 && entry.expiration <= time.Now().UnixNano() {
		c.misses.Add(1)
		// Lazy deletion
		c.mu.Lock()
		delete(c.data, key)
		delete(c.ttls, key)
		c.mu.Unlock()
		return nil, nil
	}

	c.hits.Add(1)
	// Return a copy to prevent mutation
	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, nil
}

// Set stores a value in the cache.
func (c *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Check item size
	if len(value) > c.maxItemSize {
		return nil // Silently skip oversized items (like LiteLLM)
	}

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	expiration := time.Now().Add(ttl).UnixNano()

	// Make a copy of the value
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if needed
	if len(c.data) >= c.maxSize {
		c.evictIfNeeded()
	}

	c.data[key] = &memoryCacheEntry{
		value:      valueCopy,
		expiration: expiration,
	}
	c.ttls[key] = expiration

	// Add to expiration heap
	heap.Push(&c.expirationHeap, &expirationEntry{
		key:        key,
		expiration: expiration,
	})

	c.sets.Add(1)
	return nil
}

// Delete removes a key from the cache.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	delete(c.ttls, key)
	return nil
}

// SetPipeline performs batch set operations.
func (c *MemoryCache) SetPipeline(ctx context.Context, entries []CacheEntry) error {
	for _, entry := range entries {
		if err := c.Set(ctx, entry.Key, entry.Value, entry.TTL); err != nil {
			return err
		}
	}
	return nil
}

// GetMulti retrieves multiple keys at once.
func (c *MemoryCache) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(keys))

	c.mu.RLock()
	now := time.Now().UnixNano()
	for _, key := range keys {
		if entry, ok := c.data[key]; ok {
			if entry.expiration == 0 || entry.expiration > now {
				valueCopy := make([]byte, len(entry.value))
				copy(valueCopy, entry.value)
				result[key] = valueCopy
				c.hits.Add(1)
			} else {
				c.misses.Add(1)
			}
		} else {
			c.misses.Add(1)
		}
	}
	c.mu.RUnlock()

	return result, nil
}

// Ping always returns nil for memory cache.
func (c *MemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Close stops the cleanup goroutine and releases resources.
func (c *MemoryCache) Close() error {
	c.cleanupTicker.Stop()
	close(c.stopCleanup)
	return nil
}

// Stats returns cache statistics.
func (c *MemoryCache) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStats{
		Hits:    hits,
		Misses:  misses,
		Sets:    c.sets.Load(),
		HitRate: hitRate,
	}
}

// Len returns the number of items in the cache.
func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Flush removes all entries from the cache.
func (c *MemoryCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*memoryCacheEntry)
	c.ttls = make(map[string]int64)
	c.expirationHeap = make(expirationHeap, 0)
	heap.Init(&c.expirationHeap)
}
