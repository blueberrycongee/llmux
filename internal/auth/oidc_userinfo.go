// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/httputil"
)

// UserInfoConfig contains configuration for OIDC UserInfo endpoint.
type UserInfoConfig struct {
	Enabled  bool          `json:"enabled"`
	CacheTTL time.Duration `json:"cache_ttl"` // Cache duration for user info
}

// UserInfoClient provides access to OIDC UserInfo endpoint with caching.
type UserInfoClient struct {
	httpClient  *http.Client
	userInfoURL string
	cacheTTL    time.Duration
	cache       map[string]*userInfoCacheEntry
	mu          sync.RWMutex
}

// userInfoCacheEntry represents a cached user info response.
type userInfoCacheEntry struct {
	info      *UserInfoResponse
	expiresAt time.Time
}

// UserInfoResponse represents the response from OIDC UserInfo endpoint.
type UserInfoResponse struct {
	// Standard OIDC claims
	Subject           string `json:"sub"`
	Name              string `json:"name,omitempty"`
	GivenName         string `json:"given_name,omitempty"`
	FamilyName        string `json:"family_name,omitempty"`
	MiddleName        string `json:"middle_name,omitempty"`
	Nickname          string `json:"nickname,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Profile           string `json:"profile,omitempty"`
	Picture           string `json:"picture,omitempty"`
	Website           string `json:"website,omitempty"`
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	Gender            string `json:"gender,omitempty"`
	Birthdate         string `json:"birthdate,omitempty"`
	Zoneinfo          string `json:"zoneinfo,omitempty"`
	Locale            string `json:"locale,omitempty"`
	PhoneNumber       string `json:"phone_number,omitempty"`
	PhoneVerified     bool   `json:"phone_number_verified,omitempty"`
	UpdatedAt         int64  `json:"updated_at,omitempty"`

	// Common custom claims
	Groups []string `json:"groups,omitempty"`
	Roles  []string `json:"roles,omitempty"`
	Teams  []string `json:"teams,omitempty"`

	// Raw claims for custom processing
	RawClaims map[string]any `json:"-"`
}

// NewUserInfoClient creates a new UserInfo client.
func NewUserInfoClient(userInfoURL string, cacheTTL time.Duration) *UserInfoClient {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute // Default 5 minute cache
	}

	return &UserInfoClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		userInfoURL: userInfoURL,
		cacheTTL:    cacheTTL,
		cache:       make(map[string]*userInfoCacheEntry),
	}
}

// GetUserInfo fetches user information from the OIDC UserInfo endpoint.
// Results are cached based on the access token.
func (c *UserInfoClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	// Check cache first
	if info := c.getFromCache(accessToken); info != nil {
		return info, nil
	}

	// Fetch from UserInfo endpoint
	info, err := c.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.setCache(accessToken, info)

	return info, nil
}

// getFromCache retrieves user info from cache if not expired.
func (c *UserInfoClient) getFromCache(token string) *UserInfoResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[token]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil // Expired
	}

	return entry.info
}

// setCache stores user info in cache.
func (c *UserInfoClient) setCache(token string, info *UserInfoResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple size limit to prevent memory leaks
	if len(c.cache) >= 1000 {
		// Clear cache if it gets too big (simple strategy)
		// In a real production system, we might want LRU
		c.cache = make(map[string]*userInfoCacheEntry)
	}

	c.cache[token] = &userInfoCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
}

// fetchUserInfo makes the HTTP request to the UserInfo endpoint.
func (c *UserInfoClient) fetchUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.userInfoURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := httputil.ReadLimitedBody(resp.Body, httputil.DefaultMaxResponseBodyBytes)
		return nil, fmt.Errorf("user info endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := httputil.ReadLimitedBody(resp.Body, httputil.DefaultMaxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var info UserInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse user info: %w", err)
	}

	// Also store raw claims for custom processing
	var rawClaims map[string]any
	if err := json.Unmarshal(body, &rawClaims); err == nil {
		info.RawClaims = rawClaims
	}

	return &info, nil
}

// ClearCache clears all cached user info entries.
func (c *UserInfoClient) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*userInfoCacheEntry)
}

// ClearExpiredCache removes expired entries from the cache.
func (c *UserInfoClient) ClearExpiredCache() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cleared := 0

	for token, entry := range c.cache {
		if now.After(entry.expiresAt) {
			delete(c.cache, token)
			cleared++
		}
	}

	return cleared
}

// CacheSize returns the current number of cached entries.
func (c *UserInfoClient) CacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// GetClaim retrieves a specific claim from the user info.
func (info *UserInfoResponse) GetClaim(claimName string) any {
	if info.RawClaims == nil {
		return nil
	}
	return info.RawClaims[claimName]
}

// GetStringClaim retrieves a string claim from the user info.
func (info *UserInfoResponse) GetStringClaim(claimName string) string {
	val := info.GetClaim(claimName)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// GetStringSliceClaim retrieves a string slice claim from the user info.
func (info *UserInfoResponse) GetStringSliceClaim(claimName string) []string {
	val := info.GetClaim(claimName)
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}
