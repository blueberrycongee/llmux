package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewModelAccess_AllowsIntersection(t *testing.T) {
	store := NewMemoryStore()
	orgID := "org-1"
	userID := "user-1"

	org := &Organization{
		ID:     orgID,
		Alias:  "org",
		Models: []string{"gpt-4"},
	}
	user := &User{
		ID:     userID,
		Role:   string(UserRoleInternalUser),
		Models: []string{"gpt-4"},
	}

	if err := store.CreateOrganization(context.Background(), org); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	team := &Team{ID: "team-1", Models: []string{"gpt-4"}}

	key := &APIKey{
		ID:             "key-1",
		IsActive:       true,
		AllowedModels:  []string{"gpt-4"},
		OrganizationID: &orgID,
		UserID:         &userID,
	}

	authCtx := &AuthContext{
		APIKey: key,
		Team:   team,
	}

	access, err := NewModelAccess(context.Background(), store, authCtx)
	if err != nil {
		t.Fatalf("NewModelAccess: %v", err)
	}

	if !access.Allows("gpt-4") {
		t.Fatalf("expected model to be allowed")
	}

	if access.Allows("gpt-3.5") {
		t.Fatalf("expected model to be denied")
	}
}

func TestNewModelAccess_LoadsUserAndOrg(t *testing.T) {
	store := NewMemoryStore()
	orgID := "org-2"
	userID := "user-2"

	org := &Organization{
		ID:     orgID,
		Alias:  "org",
		Models: []string{"gpt-4"},
	}
	user := &User{
		ID:     userID,
		Role:   string(UserRoleInternalUser),
		Models: []string{"gpt-4"},
	}

	if err := store.CreateOrganization(context.Background(), org); err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	key := &APIKey{
		ID:             "key-2",
		IsActive:       true,
		OrganizationID: &orgID,
		UserID:         &userID,
	}

	authCtx := &AuthContext{
		APIKey: key,
	}

	access, err := NewModelAccess(context.Background(), store, authCtx)
	if err != nil {
		t.Fatalf("NewModelAccess: %v", err)
	}

	if !access.Allows("gpt-4") {
		t.Fatalf("expected model to be allowed")
	}

	if access.Allows("gpt-3.5") {
		t.Fatalf("expected model to be denied")
	}
}

func TestModelAccessMiddleware_AllowsAndPreservesBody(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:        "key-allow",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	expectedBody := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != expectedBody {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(expectedBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestModelAccessMiddleware_AllowsCompletionsBody(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:        "key-allow-completions",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	expectedBody := `{"model":"gpt-4","prompt":"hi"}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != expectedBody {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(expectedBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestModelAccessMiddleware_DeniesDisallowedModel(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:            "key-deny",
		KeyHash:       hash,
		KeyPrefix:     ExtractKeyPrefix(fullKey),
		IsActive:      true,
		AllowedModels: []string{"gpt-3.5"},
		CreatedAt:     time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if called {
		t.Fatalf("handler should not be called for disallowed model")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "permission_error") {
		t.Fatalf("expected permission_error, got %s", rr.Body.String())
	}
}

func TestModelAccessMiddleware_UsesTeamRestrictions(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	teamID := "team-1"
	team := &Team{
		ID:       teamID,
		Models:   []string{"gpt-3.5"},
		IsActive: true,
	}
	if err := store.CreateTeam(context.Background(), team); err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:            "key-team",
		KeyHash:       hash,
		KeyPrefix:     ExtractKeyPrefix(fullKey),
		IsActive:      true,
		AllowedModels: []string{"gpt-4"},
		TeamID:        &teamID,
		CreatedAt:     time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestModelAccessMiddleware_PassesThroughInvalidJSON(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:        "key-invalid-json",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":`))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
}

func TestModelAccessMiddleware_PassesThroughMissingModel(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:        "key-missing-model",
		KeyHash:   hash,
		KeyPrefix: ExtractKeyPrefix(fullKey),
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	reqBody := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
}

func TestModelAccessMiddleware_ChecksEmbeddingAliases(t *testing.T) {
	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	fullKey, hash, _ := GenerateAPIKey()
	key := &APIKey{
		ID:            "key-embed",
		KeyHash:       hash,
		KeyPrefix:     ExtractKeyPrefix(fullKey),
		IsActive:      true,
		AllowedModels: []string{"text-embedding-3-small"},
		CreatedAt:     time.Now(),
	}
	if err := store.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	middleware := NewMiddleware(&MiddlewareConfig{
		Store:   store,
		Logger:  logger,
		Enabled: true,
	})

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	reqBody := `{"model":"text-embedding-3-small","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/embeddings", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+fullKey)

	rr := httptest.NewRecorder()
	chain := middleware.Authenticate(middleware.ModelAccessMiddleware(handler))
	chain.ServeHTTP(rr, req)

	if !called {
		t.Fatalf("expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
