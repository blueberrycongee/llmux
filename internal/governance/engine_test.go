package governance

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/resilience"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

type errorLimiter struct{}

func (errorLimiter) CheckAllow(context.Context, []resilience.Descriptor) ([]resilience.Result, error) {
	return nil, errors.New("backend unavailable")
}

func TestEngineEvaluate_ModelAccessDenied(t *testing.T) {
	store := auth.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(Config{Enabled: true}, WithStore(store), WithLogger(logger))

	apiKey := &auth.APIKey{
		ID:            "key-1",
		AllowedModels: []string{"gpt-3"},
		IsActive:      true,
	}
	ctx := auth.WithAuthContext(context.Background(), &auth.AuthContext{APIKey: apiKey})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := engine.Evaluate(ctx, RequestInput{
		Request: req,
		Model:   "gpt-4",
	})
	if err == nil {
		t.Fatal("expected model access error, got nil")
	}
	var llmErr *llmerrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Type != llmerrors.TypePermissionDenied {
		t.Fatalf("expected permission error, got %q", llmErr.Type)
	}
	if llmErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", llmErr.StatusCode)
	}
}

func TestEngineEvaluate_BudgetExceeded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(Config{Enabled: true}, WithLogger(logger))

	apiKey := &auth.APIKey{
		ID:          "key-1",
		MaxBudget:   100,
		SpentBudget: 150,
		IsActive:    true,
	}
	ctx := auth.WithAuthContext(context.Background(), &auth.AuthContext{APIKey: apiKey})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := engine.Evaluate(ctx, RequestInput{
		Request: req,
		Model:   "gpt-4",
	})
	if err == nil {
		t.Fatal("expected budget error, got nil")
	}
	var llmErr *llmerrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Type != llmerrors.TypeInsufficientQuota {
		t.Fatalf("expected quota error, got %q", llmErr.Type)
	}
	if llmErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("expected status 402, got %d", llmErr.StatusCode)
	}
}

func TestEngineEvaluate_RateLimitFailCloseDenies(t *testing.T) {
	limiter := auth.NewTenantRateLimiter(&auth.TenantRateLimiterConfig{
		DefaultRPM:  60,
		DefaultBurst: 10,
		FailOpen:   false,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	limiter.SetDistributedLimiter(errorLimiter{})
	engine := NewEngine(Config{Enabled: true}, WithRateLimiter(limiter))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "203.0.113.1:1234"

	err := engine.Evaluate(context.Background(), RequestInput{
		Request: req,
		Model:   "gpt-4",
	})
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	var llmErr *llmerrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Type != llmerrors.TypeRateLimit {
		t.Fatalf("expected rate limit error, got %q", llmErr.Type)
	}
}

func TestEngineEvaluate_RateLimitFailOpenAllows(t *testing.T) {
	limiter := auth.NewTenantRateLimiter(&auth.TenantRateLimiterConfig{
		DefaultRPM:  60,
		DefaultBurst: 10,
		FailOpen:   true,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	limiter.SetDistributedLimiter(errorLimiter{})
	engine := NewEngine(Config{Enabled: true}, WithRateLimiter(limiter))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.RemoteAddr = "203.0.113.1:1234"

	err := engine.Evaluate(context.Background(), RequestInput{
		Request: req,
		Model:   "gpt-4",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

type countingStore struct {
	*auth.MemoryStore
	mu       sync.Mutex
	logCalls int
}

func newCountingStore() *countingStore {
	return &countingStore{MemoryStore: auth.NewMemoryStore()}
}

func (s *countingStore) LogUsage(ctx context.Context, log *auth.UsageLog) error {
	s.mu.Lock()
	s.logCalls++
	s.mu.Unlock()
	return s.MemoryStore.LogUsage(ctx, log)
}

func (s *countingStore) LogCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logCalls
}

func TestEngineAccount_Idempotent(t *testing.T) {
	store := newCountingStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(Config{
		Enabled:            true,
		AsyncAccounting:    false,
		IdempotencyWindow:  5 * time.Minute,
	}, WithStore(store), WithLogger(logger), WithIdempotencyStore(NewMemoryIdempotencyStore()))

	apiKey := &auth.APIKey{
		ID:       "key-1",
		IsActive: true,
	}
	if err := store.CreateAPIKey(context.Background(), apiKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}

	ctx := auth.WithAuthContext(context.Background(), &auth.AuthContext{APIKey: apiKey})
	input := AccountInput{
		RequestID: "req-1",
		Model:     "gpt-4",
		CallType:  "chat_completion",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
			Cost:             1.5,
			Provider:         "openai",
		},
		Start:   time.Now(),
		Latency: 2 * time.Second,
	}

	engine.Account(ctx, input)
	engine.Account(ctx, input)

	if store.LogCalls() != 1 {
		t.Fatalf("expected 1 usage log, got %d", store.LogCalls())
	}

	key, err := store.GetAPIKeyByID(context.Background(), apiKey.ID)
	if err != nil {
		t.Fatalf("GetAPIKeyByID() error = %v", err)
	}
	if key.SpentBudget != 1.5 {
		t.Fatalf("expected spent budget 1.5, got %.2f", key.SpentBudget)
	}
}

func TestEngineAccount_UpdatesSpend(t *testing.T) {
	store := newCountingStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := NewEngine(Config{Enabled: true, AsyncAccounting: false}, WithStore(store), WithLogger(logger))

	org := &auth.Organization{ID: "org-1", Spend: 0}
	team := &auth.Team{ID: "team-1", OrganizationID: &org.ID, SpentBudget: 0, IsActive: true}
	user := &auth.User{ID: "user-1", TeamID: &team.ID, OrganizationID: &org.ID, Spend: 0, IsActive: true}
	apiKey := &auth.APIKey{
		ID:             "key-1",
		TeamID:         &team.ID,
		UserID:         &user.ID,
		OrganizationID: &org.ID,
		IsActive:       true,
	}
	endUser := &auth.EndUser{UserID: "end-user-1", Spend: 0}

	if err := store.CreateOrganization(context.Background(), org); err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if err := store.CreateTeam(context.Background(), team); err != nil {
		t.Fatalf("CreateTeam() error = %v", err)
	}
	if err := store.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := store.CreateAPIKey(context.Background(), apiKey); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if err := store.CreateEndUser(context.Background(), endUser); err != nil {
		t.Fatalf("CreateEndUser() error = %v", err)
	}

	ctx := auth.WithAuthContext(context.Background(), &auth.AuthContext{
		APIKey: apiKey,
		Team:   team,
		User:   user,
	})
	engine.Account(ctx, AccountInput{
		RequestID: "req-2",
		Model:     "gpt-4",
		CallType:  "chat_completion",
		EndUserID: endUser.UserID,
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			Cost:             2.0,
			Provider:         "openai",
		},
		Start:   time.Now(),
		Latency: time.Second,
	})

	key, _ := store.GetAPIKeyByID(context.Background(), apiKey.ID)
	if key.SpentBudget != 2.0 {
		t.Fatalf("api key spend = %.2f, want 2.0", key.SpentBudget)
	}
	loadedTeam, _ := store.GetTeam(context.Background(), team.ID)
	if loadedTeam.SpentBudget != 2.0 {
		t.Fatalf("team spend = %.2f, want 2.0", loadedTeam.SpentBudget)
	}
	loadedUser, _ := store.GetUser(context.Background(), user.ID)
	if loadedUser.Spend != 2.0 {
		t.Fatalf("user spend = %.2f, want 2.0", loadedUser.Spend)
	}
	loadedOrg, _ := store.GetOrganization(context.Background(), org.ID)
	if loadedOrg.Spend != 2.0 {
		t.Fatalf("org spend = %.2f, want 2.0", loadedOrg.Spend)
	}
	loadedEndUser, _ := store.GetEndUser(context.Background(), endUser.UserID)
	if loadedEndUser.Spend != 2.0 {
		t.Fatalf("end user spend = %.2f, want 2.0", loadedEndUser.Spend)
	}
}
