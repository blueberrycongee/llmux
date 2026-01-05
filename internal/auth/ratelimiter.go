package auth

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TenantRateLimiter provides per-tenant rate limiting.
type TenantRateLimiter struct {
	mu           sync.RWMutex
	limiters     map[string]*rate.Limiter
	defaultRate  rate.Limit
	defaultBurst int
	cleanupTTL   time.Duration
	lastAccess   map[string]time.Time
}

// TenantRateLimiterConfig contains configuration for the tenant rate limiter.
type TenantRateLimiterConfig struct {
	DefaultRPM   int           // Default requests per minute
	DefaultBurst int           // Default burst size
	CleanupTTL   time.Duration // TTL for inactive limiters
}

// NewTenantRateLimiter creates a new per-tenant rate limiter.
func NewTenantRateLimiter(cfg *TenantRateLimiterConfig) *TenantRateLimiter {
	if cfg.DefaultRPM <= 0 {
		cfg.DefaultRPM = 60
	}
	if cfg.DefaultBurst <= 0 {
		cfg.DefaultBurst = 10
	}
	if cfg.CleanupTTL <= 0 {
		cfg.CleanupTTL = 10 * time.Minute
	}

	trl := &TenantRateLimiter{
		limiters:     make(map[string]*rate.Limiter),
		defaultRate:  rate.Limit(float64(cfg.DefaultRPM) / 60.0),
		defaultBurst: cfg.DefaultBurst,
		cleanupTTL:   cfg.CleanupTTL,
		lastAccess:   make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go trl.cleanupLoop()

	return trl
}

// Allow checks if a request is allowed for the given tenant.
func (trl *TenantRateLimiter) Allow(tenantID string) bool {
	limiter := trl.getLimiter(tenantID, 0, 0)
	return limiter.Allow()
}

// AllowN checks if n requests are allowed for the given tenant.
func (trl *TenantRateLimiter) AllowN(tenantID string, n int) bool {
	limiter := trl.getLimiter(tenantID, 0, 0)
	return limiter.AllowN(time.Now(), n)
}

// Wait blocks until a request is allowed or context is canceled.
func (trl *TenantRateLimiter) Wait(ctx context.Context, tenantID string) error {
	limiter := trl.getLimiter(tenantID, 0, 0)
	return limiter.Wait(ctx)
}

// AllowWithCustomRate checks if a request is allowed using a custom rate.
func (trl *TenantRateLimiter) AllowWithCustomRate(tenantID string, rpm int, burst int) bool {
	limiter := trl.getLimiter(tenantID, rpm, burst)
	return limiter.Allow()
}

// getLimiter returns or creates a rate limiter for the tenant.
func (trl *TenantRateLimiter) getLimiter(tenantID string, rpm int, burst int) *rate.Limiter {
	trl.mu.RLock()
	limiter, exists := trl.limiters[tenantID]
	trl.mu.RUnlock()

	if exists {
		trl.mu.Lock()
		trl.lastAccess[tenantID] = time.Now()
		trl.mu.Unlock()
		return limiter
	}

	// Create new limiter
	trl.mu.Lock()
	defer trl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = trl.limiters[tenantID]; exists {
		trl.lastAccess[tenantID] = time.Now()
		return limiter
	}

	// Use custom rate if provided, otherwise use default
	r := trl.defaultRate
	b := trl.defaultBurst
	if rpm > 0 {
		r = rate.Limit(float64(rpm) / 60.0)
	}
	if burst > 0 {
		b = burst
	}

	limiter = rate.NewLimiter(r, b)
	trl.limiters[tenantID] = limiter
	trl.lastAccess[tenantID] = time.Now()

	return limiter
}

// SetRate updates the rate limit for a specific tenant.
func (trl *TenantRateLimiter) SetRate(tenantID string, rpm int, burst int) {
	trl.mu.Lock()
	defer trl.mu.Unlock()

	r := rate.Limit(float64(rpm) / 60.0)
	if limiter, exists := trl.limiters[tenantID]; exists {
		limiter.SetLimit(r)
		limiter.SetBurst(burst)
	} else {
		trl.limiters[tenantID] = rate.NewLimiter(r, burst)
	}
	trl.lastAccess[tenantID] = time.Now()
}

// Remove removes the rate limiter for a tenant.
func (trl *TenantRateLimiter) Remove(tenantID string) {
	trl.mu.Lock()
	defer trl.mu.Unlock()
	delete(trl.limiters, tenantID)
	delete(trl.lastAccess, tenantID)
}

// Stats returns statistics about the rate limiter.
func (trl *TenantRateLimiter) Stats() map[string]any {
	trl.mu.RLock()
	defer trl.mu.RUnlock()

	return map[string]any{
		"active_tenants": len(trl.limiters),
		"default_rpm":    int(trl.defaultRate * 60),
		"default_burst":  trl.defaultBurst,
	}
}

// cleanupLoop periodically removes inactive limiters.
func (trl *TenantRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(trl.cleanupTTL / 2)
	defer ticker.Stop()

	for range ticker.C {
		trl.cleanup()
	}
}

func (trl *TenantRateLimiter) cleanup() {
	trl.mu.Lock()
	defer trl.mu.Unlock()

	now := time.Now()
	for tenantID, lastAccess := range trl.lastAccess {
		if now.Sub(lastAccess) > trl.cleanupTTL {
			delete(trl.limiters, tenantID)
			delete(trl.lastAccess, tenantID)
		}
	}
}

// RateLimitMiddleware creates an HTTP middleware for rate limiting.
func (trl *TenantRateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get auth context
		authCtx := GetAuthContext(r.Context())
		if authCtx == nil || authCtx.APIKey == nil {
			// No auth context, use IP-based limiting
			tenantID := r.RemoteAddr
			if !trl.Allow(tenantID) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Use API key ID as tenant ID
		tenantID := authCtx.APIKey.ID
		rpm := authCtx.APIKey.RateLimit
		burst := rpm / 6 // Default burst = 10% of RPM
		if burst < 1 {
			burst = 1
		}

		// Check team rate limit if applicable
		if authCtx.Team != nil && authCtx.Team.RateLimit > 0 {
			teamID := "team:" + authCtx.Team.ID
			if !trl.AllowWithCustomRate(teamID, authCtx.Team.RateLimit, authCtx.Team.RateLimit/6) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"message":"team rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
		}

		// Check API key rate limit
		if rpm > 0 {
			if !trl.AllowWithCustomRate(tenantID, rpm, burst) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
		} else if !trl.Allow(tenantID) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
