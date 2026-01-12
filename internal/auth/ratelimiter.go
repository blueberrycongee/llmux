package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/blueberrycongee/llmux/internal/metrics"
	"github.com/blueberrycongee/llmux/internal/resilience"
)

// TenantRateLimiter provides per-tenant rate limiting.
type TenantRateLimiter struct {
	mu                 sync.RWMutex
	limiters           map[string]*rate.Limiter
	defaultRate        rate.Limit
	defaultBurst       int
	useDefaultBurst    bool
	cleanupTTL         time.Duration
	lastAccess         map[string]time.Time
	distributedLimiter resilience.DistributedLimiter
	failOpen           bool
	logger             *slog.Logger
	trustedProxies     []*net.IPNet
}

// TenantRateLimiterConfig contains configuration for the tenant rate limiter.
type TenantRateLimiterConfig struct {
	DefaultRPM        int           // Default requests per minute
	DefaultBurst      int           // Default burst size
	UseDefaultBurst   bool          // Override burst for custom RPM limits
	CleanupTTL        time.Duration // TTL for inactive limiters
	FailOpen          bool          // Allow requests when limiter backend fails
	Logger            *slog.Logger  // Optional logger for limiter events
	TrustedProxyCIDRs []string      // Trusted proxy IPs/CIDRs for forwarded headers
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
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	trustedProxies, invalidProxyCIDRs := parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)
	for _, value := range invalidProxyCIDRs {
		cfg.Logger.Warn("invalid trusted proxy cidr ignored", "value", value)
	}

	trl := &TenantRateLimiter{
		limiters:        make(map[string]*rate.Limiter),
		defaultRate:     rate.Limit(float64(cfg.DefaultRPM) / 60.0),
		defaultBurst:    cfg.DefaultBurst,
		useDefaultBurst: cfg.UseDefaultBurst,
		cleanupTTL:      cfg.CleanupTTL,
		lastAccess:      make(map[string]time.Time),
		failOpen:        cfg.FailOpen,
		logger:          cfg.Logger,
		trustedProxies:  trustedProxies,
	}

	// Start cleanup goroutine
	go trl.cleanupLoop()

	return trl
}

// SetDistributedLimiter sets the distributed limiter instance.
func (trl *TenantRateLimiter) SetDistributedLimiter(l resilience.DistributedLimiter) {
	trl.mu.Lock()
	defer trl.mu.Unlock()
	trl.distributedLimiter = l
}

// Check checks if a request is allowed, using distributed limiter if available,
// with fail-open/close behavior on backend errors.
func (trl *TenantRateLimiter) Check(ctx context.Context, tenantID string, rpm, burst int) (bool, error) {
	// Try distributed limiter first if available
	if trl.distributedLimiter != nil {
		limit := int64(rpm)
		if limit <= 0 {
			limit = int64(trl.defaultRate * 60)
		}

		desc := resilience.Descriptor{
			Key:    tenantID,
			Value:  "request",
			Limit:  limit,
			Type:   resilience.LimitTypeRequests,
			Window: time.Minute,
		}

		results, err := trl.distributedLimiter.CheckAllow(ctx, []resilience.Descriptor{desc})
		if err == nil && len(results) > 0 {
			return results[0].Allowed, nil
		}
		if err == nil {
			err = fmt.Errorf("distributed rate limiter returned no results")
		}
		action := "allow"
		if !trl.failOpen {
			action = "deny"
		}
		metrics.RateLimiterBackendErrors.WithLabelValues("gateway", action).Inc()
		trl.logger.Warn("distributed rate limiter check failed",
			"error", err,
			"fail_open", trl.failOpen,
			"action", action,
		)
		if trl.failOpen {
			return true, err
		}
		return false, err
	}

	// Local fallback
	return trl.AllowWithCustomRate(tenantID, rpm, burst), nil
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
func (trl *TenantRateLimiter) AllowWithCustomRate(tenantID string, rpm, burst int) bool {
	limiter := trl.getLimiter(tenantID, rpm, burst)
	return limiter.Allow()
}

// getLimiter returns or creates a rate limiter for the tenant.
func (trl *TenantRateLimiter) getLimiter(tenantID string, rpm, burst int) *rate.Limiter {
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
func (trl *TenantRateLimiter) SetRate(tenantID string, rpm, burst int) {
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
			tenantID := anonymousRateLimitKey(r, trl.trustedProxies)
			allowed, _ := trl.Check(r.Context(), tenantID, 0, 0)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Use API key ID as tenant ID
		tenantID := authCtx.APIKey.ID
		var rpm int
		if authCtx.APIKey.RPMLimit != nil {
			rpm = int(*authCtx.APIKey.RPMLimit)
		}
		burst := trl.burstForRate(rpm, 1)

		// Check team rate limit if applicable
		if authCtx.Team != nil && authCtx.Team.RPMLimit != nil && *authCtx.Team.RPMLimit > 0 {
			teamID := "team:" + authCtx.Team.ID
			teamRPM := int(*authCtx.Team.RPMLimit)
			allowed, _ := trl.Check(r.Context(), teamID, teamRPM, trl.burstForRate(teamRPM, 0))
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"team rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
		}

		// Check API key rate limit
		if rpm > 0 {
			allowed, _ := trl.Check(r.Context(), tenantID, rpm, burst)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
		} else {
			allowed, _ := trl.Check(r.Context(), tenantID, 0, 0)
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// AnonymousKey derives a tenant key for unauthenticated requests.
func (trl *TenantRateLimiter) AnonymousKey(r *http.Request) string {
	return anonymousRateLimitKey(r, trl.trustedProxies)
}

// BurstForRate calculates the burst size for a given RPM.
func (trl *TenantRateLimiter) BurstForRate(rpm int, minBurst int) int {
	return trl.burstForRate(rpm, minBurst)
}

func anonymousRateLimitKey(r *http.Request, trustedProxies []*net.IPNet) string {
	if r == nil {
		return ""
	}
	remoteHost := remoteAddrHost(r.RemoteAddr)
	if remoteHost == "" {
		return ""
	}
	if len(trustedProxies) == 0 {
		return remoteHost
	}
	remoteIP := parseIP(remoteHost)
	if remoteIP == nil || !ipInNets(remoteIP, trustedProxies) {
		return remoteHost
	}
	if ip := forwardedClientIP(r.Header.Get("Forwarded"), trustedProxies); ip != "" {
		return ip
	}
	if ip := xForwardedForClientIP(r.Header.Get("X-Forwarded-For"), trustedProxies); ip != "" {
		return ip
	}
	if ip := headerClientIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	return remoteHost
}

func (trl *TenantRateLimiter) burstForRate(rpm int, minBurst int) int {
	burst := rpm / 6
	if burst < minBurst {
		burst = minBurst
	}
	if trl.useDefaultBurst && trl.defaultBurst > 0 {
		if trl.defaultBurst < burst {
			return trl.defaultBurst
		}
		return burst
	}
	return burst
}

func remoteAddrHost(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err == nil && host != "" {
		return host
	}
	return addr
}

func forwardedClientIP(header string, trustedProxies []*net.IPNet) string {
	return selectClientIP(parseForwardedFor(header), trustedProxies)
}

func xForwardedForClientIP(header string, trustedProxies []*net.IPNet) string {
	return selectClientIP(parseXForwardedFor(header), trustedProxies)
}

func headerClientIP(value string) string {
	ip := parseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func selectClientIP(ips []net.IP, trustedProxies []*net.IPNet) string {
	if len(ips) == 0 {
		return ""
	}
	for i := len(ips) - 1; i >= 0; i-- {
		ip := normalizeIP(ips[i])
		if ip == nil {
			continue
		}
		if !ipInNets(ip, trustedProxies) {
			return ip.String()
		}
	}
	for _, ip := range ips {
		ip = normalizeIP(ip)
		if ip != nil {
			return ip.String()
		}
	}
	return ""
}

func parseForwardedFor(header string) []net.IP {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	ips := make([]net.IP, 0, len(parts))
	for _, part := range parts {
		for _, param := range strings.Split(part, ";") {
			param = strings.TrimSpace(param)
			if len(param) < 4 || !strings.EqualFold(param[:4], "for=") {
				continue
			}
			value := strings.TrimSpace(param[4:])
			if ip := parseForwardedForValue(value); ip != nil {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func parseXForwardedFor(header string) []net.IP {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	ips := make([]net.IP, 0, len(parts))
	for _, part := range parts {
		if ip := parseIP(part); ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func parseForwardedForValue(value string) net.IP {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	if value == "" || strings.EqualFold(value, "unknown") {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		if idx := strings.Index(value, "]"); idx != -1 {
			return parseIP(value[1:idx])
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return parseIP(host)
	}
	return parseIP(value)
}

func parseIP(value string) net.IP {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if idx := strings.IndexByte(value, '%'); idx != -1 {
		value = value[:idx]
	}
	ip := net.ParseIP(value)
	return normalizeIP(ip)
}

func normalizeIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4
	}
	return ip
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, ipNet := range nets {
		if ipNet != nil && ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(values []string) ([]*net.IPNet, []string) {
	if len(values) == 0 {
		return nil, nil
	}
	trusted := make([]*net.IPNet, 0, len(values))
	var invalid []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			invalid = append(invalid, value)
			continue
		}
		if strings.Contains(value, "/") {
			_, ipNet, err := net.ParseCIDR(value)
			if err != nil {
				invalid = append(invalid, value)
				continue
			}
			trusted = append(trusted, ipNet)
			continue
		}
		ip := normalizeIP(net.ParseIP(value))
		if ip == nil {
			invalid = append(invalid, value)
			continue
		}
		maskBits := 128
		if ip.To4() != nil {
			maskBits = 32
		}
		trusted = append(trusted, &net.IPNet{IP: ip, Mask: net.CIDRMask(maskBits, maskBits)})
	}
	return trusted, invalid
}
