package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTenantRateLimiter_Allow(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60, // 1 per second
		DefaultBurst: 5,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-1"

	// First 5 requests should be allowed (burst)
	for i := 0; i < 5; i++ {
		if !trl.Allow(tenantID) {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}

	// 6th request should be denied (burst exhausted)
	if trl.Allow(tenantID) {
		t.Error("6th request should be denied")
	}
}

func TestTenantRateLimiter_AllowN(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 10,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-2"

	// Request 5 tokens
	if !trl.AllowN(tenantID, 5) {
		t.Error("AllowN(5) should succeed")
	}

	// Request another 5 tokens
	if !trl.AllowN(tenantID, 5) {
		t.Error("AllowN(5) should succeed")
	}

	// Request 1 more should fail
	if trl.AllowN(tenantID, 1) {
		t.Error("AllowN(1) should fail after burst exhausted")
	}
}

func TestTenantRateLimiter_Wait(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   6000, // 100 per second for faster test
		DefaultBurst: 1,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-3"

	// First request
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := trl.Wait(ctx, tenantID); err != nil {
		t.Errorf("Wait() error = %v", err)
	}

	// Second request should wait
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	start := time.Now()
	err := trl.Wait(ctx2, tenantID)
	elapsed := time.Since(start)

	// Should either succeed after waiting or timeout
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("Wait() unexpected error = %v", err)
	}
	if err == nil && elapsed < 5*time.Millisecond {
		t.Error("Wait() should have waited")
	}
}

func TestTenantRateLimiter_AllowWithCustomRate(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-4"

	// Use custom rate with higher burst
	for i := 0; i < 10; i++ {
		if !trl.AllowWithCustomRate(tenantID, 120, 10) {
			t.Errorf("request %d should be allowed with custom rate", i+1)
		}
	}

	// 11th should fail
	if trl.AllowWithCustomRate(tenantID, 120, 10) {
		t.Error("11th request should be denied")
	}
}

func TestTenantRateLimiter_SetRate(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-5"

	// First verify the limiter is created with default settings
	stats := trl.Stats()
	if stats["default_rpm"].(int) != 60 {
		t.Errorf("default_rpm = %d, want 60", stats["default_rpm"])
	}

	// Update rate for a new tenant
	trl.SetRate(tenantID, 120, 20)

	// The new tenant should have the updated burst
	for i := 0; i < 20; i++ {
		if !trl.Allow(tenantID) {
			t.Errorf("request %d should be allowed with updated burst", i+1)
		}
	}

	// 21st should fail
	if trl.Allow(tenantID) {
		t.Error("21st request should be denied")
	}
}

func TestTenantRateLimiter_Remove(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		CleanupTTL:   time.Minute,
	})

	tenantID := "tenant-6"

	// Exhaust burst
	for i := 0; i < 5; i++ {
		trl.Allow(tenantID)
	}

	// Remove limiter
	trl.Remove(tenantID)

	// Should get fresh limiter with full burst
	for i := 0; i < 5; i++ {
		if !trl.Allow(tenantID) {
			t.Errorf("request %d should be allowed after remove", i+1)
		}
	}
}

func TestTenantRateLimiter_Stats(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 10,
		CleanupTTL:   time.Minute,
	})

	// Create some tenants
	trl.Allow("tenant-a")
	trl.Allow("tenant-b")
	trl.Allow("tenant-c")

	stats := trl.Stats()
	if stats["active_tenants"].(int) != 3 {
		t.Errorf("active_tenants = %d, want 3", stats["active_tenants"])
	}
	if stats["default_rpm"].(int) != 60 {
		t.Errorf("default_rpm = %d, want 60", stats["default_rpm"])
	}
}

func TestTenantRateLimiter_Middleware(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 2,
		CleanupTTL:   time.Minute,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test without auth context (IP-based limiting)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: expected 429, got %d", rr.Code)
	}
}

func TestTenantRateLimiter_MiddlewareWithAuth(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		CleanupTTL:   time.Minute,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create auth context with custom rate limit
	rpm := int64(60) // 1 per second, burst = 10
	authCtx := &AuthContext{
		APIKey: &APIKey{
			ID:       "key-1",
			RPMLimit: &rpm,
		},
	}

	// Make requests with auth context
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx := context.WithValue(req.Context(), AuthContextKey, authCtx)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	// 11th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), AuthContextKey, authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("11th request: expected 429, got %d", rr.Code)
	}
}

func TestTenantRateLimiter_MiddlewareWithAuth_DefaultBurstOverride(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:      60,
		DefaultBurst:    3,
		CleanupTTL:      time.Minute,
		UseDefaultBurst: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rpm := int64(60)
	authCtx := &AuthContext{
		APIKey: &APIKey{
			ID:       "key-override",
			RPMLimit: &rpm,
		},
	}

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx := context.WithValue(req.Context(), AuthContextKey, authCtx)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), AuthContextKey, authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	trl.RateLimitMiddleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("4th request: expected 429, got %d", rr.Code)
	}
}

func TestTenantRateLimiter_BurstForRate_DefaultBurstCapped(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:      60,
		DefaultBurst:    10,
		CleanupTTL:      time.Minute,
		UseDefaultBurst: true,
	})

	burst := trl.burstForRate(5, 1)
	if burst != 1 {
		t.Errorf("burst for rpm=5 = %d, want 1", burst)
	}

	trlLowerDefault := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:      60,
		DefaultBurst:    3,
		CleanupTTL:      time.Minute,
		UseDefaultBurst: true,
	})

	burst = trlLowerDefault.burstForRate(60, 1)
	if burst != 3 {
		t.Errorf("burst for rpm=60 with lower default = %d, want 3", burst)
	}
}

func TestTenantRateLimiter_IsolatedTenants(t *testing.T) {
	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 3,
		CleanupTTL:   time.Minute,
	})

	// Exhaust tenant-a's burst
	for i := 0; i < 3; i++ {
		trl.Allow("tenant-a")
	}

	// tenant-a should be denied
	if trl.Allow("tenant-a") {
		t.Error("tenant-a should be denied")
	}

	// tenant-b should still have full burst
	for i := 0; i < 3; i++ {
		if !trl.Allow("tenant-b") {
			t.Errorf("tenant-b request %d should be allowed", i+1)
		}
	}
}
