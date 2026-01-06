// Package metrics provides internal system metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// =============================================================================
// Internal Queue Metrics
// =============================================================================

var (
	// SpendUpdateQueueSize tracks the size of the spend update queue.
	SpendUpdateQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "spend_update_queue_size",
			Help:      "Size of the spend update queue",
		},
		[]string{"queue_type"}, // "memory" or "redis"
	)

	// DailySpendUpdateQueueSize tracks the size of the daily spend update queue.
	DailySpendUpdateQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "daily_spend_update_queue_size",
			Help:      "Size of the daily spend update queue",
		},
		[]string{"queue_type"}, // "memory" or "redis"
	)

	// UsageLogQueueSize tracks the size of the usage log queue.
	UsageLogQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "usage_log_queue_size",
			Help:      "Size of the usage log queue",
		},
		[]string{"queue_type"},
	)

	// CallbackQueueSize tracks the size of callback processing queues.
	CallbackQueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "callback_queue_size",
			Help:      "Size of callback processing queue",
		},
		[]string{"callback_name"},
	)
)

// =============================================================================
// Cache Metrics
// =============================================================================

var (
	// CacheHits counts cache hits.
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total cache hits",
		},
		[]string{"cache_type", "model"},
	)

	// CacheMisses counts cache misses.
	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_misses_total",
			Help:      "Total cache misses",
		},
		[]string{"cache_type", "model"},
	)

	// CacheSize tracks current cache size.
	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cache_size",
			Help:      "Current cache size (entries)",
		},
		[]string{"cache_type"},
	)

	// CacheSavedCost tracks cost saved by cache hits.
	CacheSavedCost = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_saved_cost_total",
			Help:      "Total cost saved by cache hits",
		},
		[]string{"cache_type", "model"},
	)
)

// =============================================================================
// System Health Metrics
// =============================================================================

var (
	// DBConnectionPoolSize tracks database connection pool size.
	DBConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "db_connection_pool_size",
			Help:      "Database connection pool size",
		},
		[]string{"pool_type"}, // "active", "idle", "max"
	)

	// RedisConnectionPoolSize tracks Redis connection pool size.
	RedisConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "redis_connection_pool_size",
			Help:      "Redis connection pool size",
		},
		[]string{"pool_type"},
	)

	// GoroutineCount tracks the number of goroutines.
	GoroutineCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutine_count",
			Help:      "Current number of goroutines",
		},
	)

	// MemoryUsage tracks memory usage.
	MemoryUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_bytes",
			Help:      "Memory usage in bytes",
		},
		[]string{"type"}, // "alloc", "sys", "heap_alloc", "heap_sys"
	)
)

// =============================================================================
// HTTP Server Metrics
// =============================================================================

var (
	// HTTPRequestDuration tracks HTTP request duration by route.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status_code"},
	)

	// HTTPRequestsInFlight tracks currently processing HTTP requests.
	HTTPRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
		[]string{"route"},
	)

	// HTTPRequestSize tracks HTTP request body size.
	HTTPRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request body size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8), // 100B to 10GB
		},
		[]string{"route"},
	)

	// HTTPResponseSize tracks HTTP response body size.
	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response body size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"route"},
	)
)
