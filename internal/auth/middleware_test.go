package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
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

func TestMiddleware_Authenticate_SkipsWhenAuthContextAlreadyPresent(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:     store,
		Logger:    logger,
		SkipPaths: nil,
		Enabled:   true,
	})

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	authCtx := &AuthContext{
		User:     &User{ID: "u1", Role: string(UserRoleProxyAdmin)},
		UserRole: UserRoleProxyAdmin,
	}

	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req = req.WithContext(context.WithValue(req.Context(), AuthContextKey, authCtx))

	rr := httptest.NewRecorder()
	middleware.Authenticate(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Fatalf("expected next handler to be called")
	}
}

func TestMiddleware_Authenticate_RejectsBlockedAPIKey(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	testKey := &APIKey{
		ID:        "blocked-key-id",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		Name:      "Blocked Key",
		IsActive:  true,
		Blocked:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), testKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

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
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddleware_Authenticate_RejectsBlockedTeam(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	teamID := "blocked-team-id"
	team := &Team{
		ID:        teamID,
		IsActive:  true,
		Blocked:   true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateTeam(context.Background(), team); err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}

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
	if err := store.CreateAPIKey(context.Background(), testKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

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
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestMiddleware_LastUsedUpdateWindow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("updates when last_used_at is nil", func(t *testing.T) {
		store, fullKey := setupCountingStore(t, nil)
		middleware := NewMiddleware(&MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			Enabled:                true,
			LastUsedUpdateInterval: time.Minute,
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey)
		rr := httptest.NewRecorder()
		middleware.Authenticate(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if got := store.Calls(); got != 1 {
			t.Fatalf("expected last_used update, got %d calls", got)
		}
	})

	t.Run("skips update within window", func(t *testing.T) {
		lastUsed := time.Now().Add(-30 * time.Second)
		store, fullKey := setupCountingStore(t, &lastUsed)
		middleware := NewMiddleware(&MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			Enabled:                true,
			LastUsedUpdateInterval: time.Hour,
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey)
		rr := httptest.NewRecorder()
		middleware.Authenticate(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if got := store.Calls(); got != 0 {
			t.Fatalf("expected no last_used update, got %d calls", got)
		}
	})

	t.Run("updates when outside window", func(t *testing.T) {
		lastUsed := time.Now().Add(-2 * time.Minute)
		store, fullKey := setupCountingStore(t, &lastUsed)
		middleware := NewMiddleware(&MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			Enabled:                true,
			LastUsedUpdateInterval: time.Minute,
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey)
		rr := httptest.NewRecorder()
		middleware.Authenticate(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if got := store.Calls(); got != 1 {
			t.Fatalf("expected last_used update, got %d calls", got)
		}
	})

	t.Run("updates when interval is disabled", func(t *testing.T) {
		lastUsed := time.Now()
		store, fullKey := setupCountingStore(t, &lastUsed)
		middleware := NewMiddleware(&MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			Enabled:                true,
			LastUsedUpdateInterval: 0,
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey)
		rr := httptest.NewRecorder()
		middleware.Authenticate(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if got := store.Calls(); got != 1 {
			t.Fatalf("expected last_used update, got %d calls", got)
		}
	})

	t.Run("skips update when last_used_at is in the future", func(t *testing.T) {
		lastUsed := time.Now().Add(2 * time.Minute)
		store, fullKey := setupCountingStore(t, &lastUsed)
		middleware := NewMiddleware(&MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			Enabled:                true,
			LastUsedUpdateInterval: time.Minute,
		})

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+fullKey)
		rr := httptest.NewRecorder()
		middleware.Authenticate(handler).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if got := store.Calls(); got != 0 {
			t.Fatalf("expected no last_used update, got %d calls", got)
		}
	})
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

func TestMiddleware_BudgetExceededDoesNotBlock(t *testing.T) {
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

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for budget exceeded, got %d", http.StatusOK, rr.Code)
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

func TestMiddleware_TeamBudgetExceededDoesNotBlock(t *testing.T) {
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

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d for team budget exceeded, got %d", http.StatusOK, rr.Code)
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

type countingStore struct {
	*MemoryStore
	mu       sync.Mutex
	calls    int
	lastUsed time.Time
}

func newCountingStore() *countingStore {
	return &countingStore{MemoryStore: NewMemoryStore()}
}

func (s *countingStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	s.mu.Lock()
	s.calls++
	s.lastUsed = lastUsed
	s.mu.Unlock()
	return s.MemoryStore.UpdateAPIKeyLastUsed(ctx, keyID, lastUsed)
}

func (s *countingStore) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func setupCountingStore(t *testing.T, lastUsed *time.Time) (*countingStore, string) {
	t.Helper()

	store := newCountingStore()
	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:         "test-key-id",
		KeyHash:    hash,
		KeyPrefix:  ExtractKeyPrefix(fullKey),
		Name:       "Test Key",
		IsActive:   true,
		CreatedAt:  time.Now(),
		LastUsedAt: lastUsed,
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	return store, fullKey
}
