package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMiddleware_Authenticate(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a test API key
	fullKey, hash, _ := GenerateAPIKey()
	testKey := &APIKey{
		ID:        "test-key-id",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Test Key",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	store.CreateAPIKey(context.Background(), testKey)

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:     store,
		Logger:    logger,
		SkipPaths: []string{"/health", "/metrics"},
		Enabled:   true,
	})

	// Handler that checks auth context (only for authenticated paths)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth context check for skip paths
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			w.WriteHeader(http.StatusOK)
			return
		}
		auth := GetAuthContext(r.Context())
		if auth == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		path           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "valid api key",
			path:           "/v1/chat/completions",
			authHeader:     "Bearer " + fullKey,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing auth header",
			path:           "/v1/chat/completions",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid api key",
			path:           "/v1/chat/completions",
			authHeader:     "Bearer invalid_key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "skip path - health",
			path:           "/health",
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "skip path - metrics",
			path:           "/metrics",
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.Authenticate(handler).ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestMiddleware_ExpiredKey(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create an expired API key
	fullKey, hash, _ := GenerateAPIKey()
	expiredTime := time.Now().Add(-24 * time.Hour)
	testKey := &APIKey{
		ID:        "expired-key-id",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Expired Key",
		IsActive:  true,
		ExpiresAt: &expiredTime,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	store.CreateAPIKey(context.Background(), testKey)

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	middleware.Authenticate(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d for expired key, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddleware_InactiveKey(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create an inactive API key
	fullKey, hash, _ := GenerateAPIKey()
	testKey := &APIKey{
		ID:        "inactive-key-id",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Inactive Key",
		IsActive:  false,
		CreatedAt: time.Now(),
	}
	store.CreateAPIKey(context.Background(), testKey)

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	middleware.Authenticate(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d for inactive key, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddleware_BudgetExceeded(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create an API key that exceeded budget
	fullKey, hash, _ := GenerateAPIKey()
	testKey := &APIKey{
		ID:          "budget-key-id",
		KeyHash:     hash,
		KeyPrefix:   ExtractKeyPrefix(fullKey),
		Name:        "Budget Key",
		IsActive:    true,
		MaxBudget:   100.0,
		SpentBudget: 150.0, // Over budget
		CreatedAt:   time.Now(),
	}
	store.CreateAPIKey(context.Background(), testKey)

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	middleware.Authenticate(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("expected status %d for budget exceeded, got %d", http.StatusPaymentRequired, rr.Code)
	}
}

func TestMiddleware_Disabled(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: false, // Disabled
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request without auth header should pass when disabled
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()
	middleware.Authenticate(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d when auth disabled, got %d", http.StatusOK, rr.Code)
	}
}

func TestMiddleware_TeamBudgetExceeded(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a team that exceeded budget
	teamID := "team-over-budget"
	alias := "Over Budget Team"
	team := &Team{
		ID:          teamID,
		Alias:       &alias,
		MaxBudget:   1000.0,
		SpentBudget: 1500.0, // Over budget
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	store.CreateTeam(context.Background(), team)

	// Create an API key associated with the team
	fullKey, hash, _ := GenerateAPIKey()
	testKey := &APIKey{
		ID:        "team-key-id",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Team Key",
		TeamID:    &teamID,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	store.CreateAPIKey(context.Background(), testKey)

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	middleware.Authenticate(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("expected status %d for team budget exceeded, got %d", http.StatusPaymentRequired, rr.Code)
	}
}

func TestGetAuthContext(t *testing.T) {
	// Test with no auth context
	ctx := context.Background()
	auth := GetAuthContext(ctx)
	if auth != nil {
		t.Error("expected nil auth context for empty context")
	}

	// Test with auth context
	authCtx := &AuthContext{
		APIKey: &APIKey{ID: "test-id"},
	}
	ctx = context.WithValue(ctx, AuthContextKey, authCtx)
	auth = GetAuthContext(ctx)
	if auth == nil {
		t.Fatal("expected non-nil auth context")
	}
	if auth.APIKey.ID != "test-id" {
		t.Errorf("expected key ID 'test-id', got %q", auth.APIKey.ID)
	}
}
