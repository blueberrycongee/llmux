package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
)

// TestServer manages a LLMux server instance for testing.
type TestServer struct {
	server   *http.Server
	listener net.Listener
	config   *config.Config
	baseURL  string
	logger   *slog.Logger
	client   *llmux.Client
	store    auth.Store
	pricingFile string
}

// ServerOption configures the test server.
type ServerOption func(*serverOptions)

type serverOptions struct {
	mockProviderURL string
	mockAPIKey      string
	models          []string
	port            int
	cacheEnabled    bool
	cacheType       string
	redisURL        string
	authEnabled     bool
	timeout         time.Duration
	retryCount      int
	retryBackoff    time.Duration
	providers       []ProviderConfig // Multiple providers for fallback testing
	oidcConfig      *config.OIDCConfig
}

// ProviderConfig defines a provider for testing.
type ProviderConfig struct {
	Name   string
	URL    string
	Models []string
}

// WithMockProvider configures the server to use a mock LLM provider.
func WithMockProvider(mockURL string) ServerOption {
	return func(o *serverOptions) {
		o.mockProviderURL = mockURL
	}
}

// WithMultipleProviders configures multiple providers for fallback testing.
func WithMultipleProviders(providers []ProviderConfig) ServerOption {
	return func(o *serverOptions) {
		o.providers = providers
	}
}

// WithMockAPIKey sets the API key for the mock provider.
func WithMockAPIKey(apiKey string) ServerOption {
	return func(o *serverOptions) {
		o.mockAPIKey = apiKey
	}
}

// WithModels sets the models to register.
func WithModels(models ...string) ServerOption {
	return func(o *serverOptions) {
		o.models = models
	}
}

// WithPort sets a specific port for the server.
func WithPort(port int) ServerOption {
	return func(o *serverOptions) {
		o.port = port
	}
}

// WithCache enables caching with the specified type.
func WithCache(cacheType, redisURL string) ServerOption {
	return func(o *serverOptions) {
		o.cacheEnabled = true
		o.cacheType = cacheType
		o.redisURL = redisURL
	}
}

// WithAuth enables authentication.
func WithAuth() ServerOption {
	return func(o *serverOptions) {
		o.authEnabled = true
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.timeout = timeout
	}
}

// WithRetry sets retry behavior for the test server's client.
func WithRetry(count int, backoff time.Duration) ServerOption {
	return func(o *serverOptions) {
		o.retryCount = count
		o.retryBackoff = backoff
	}
}

// WithOIDC configures OIDC authentication.
func WithOIDC(oidcConfig *config.OIDCConfig) ServerOption {
	return func(o *serverOptions) {
		o.authEnabled = true
		o.oidcConfig = oidcConfig
	}
}

// NewTestServer creates a new test server with the given options.
func NewTestServer(opts ...ServerOption) (*TestServer, error) {
	options := &serverOptions{
		mockAPIKey: "test-api-key",
		models:     []string{"gpt-4o-mock", "gpt-3.5-turbo-mock"},
		port:       0, // Random port
		timeout:    30 * time.Second,
		retryCount: 0,
		retryBackoff: 0,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Create logger (discard in tests)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors in tests
	}))

	// Create config
	cfg := config.DefaultConfig()
	cfg.Server.Port = options.port
	cfg.Auth.Enabled = options.authEnabled
	cfg.Cache.Enabled = options.cacheEnabled
	if options.cacheType != "" {
		cfg.Cache.Type = options.cacheType
	}
	if options.redisURL != "" {
		cfg.Cache.Redis.Addr = options.redisURL
	}
	if options.oidcConfig != nil {
		cfg.Auth.OIDC = *options.oidcConfig
	}

	// Build llmux.Client options (aligns test server with production handler)
	clientOpts := []llmux.Option{
		llmux.WithLogger(logger),
		llmux.WithCooldown(0),
		llmux.WithRetry(options.retryCount, options.retryBackoff),
	}
	if options.timeout > 0 {
		clientOpts = append(clientOpts, llmux.WithTimeout(options.timeout))
	}

	pricingFile, err := writePricingFileFromOptions(options)
	if err != nil {
		return nil, err
	}
	clientOpts = append(clientOpts, llmux.WithPricingFile(pricingFile))

	if len(options.providers) > 0 {
		for _, p := range options.providers {
			clientOpts = append(clientOpts, llmux.WithProvider(llmux.ProviderConfig{
				Name:                p.Name,
				Type:                "openai",
				APIKey:              options.mockAPIKey,
				BaseURL:             p.URL,
				AllowPrivateBaseURL: true,
				Models:              p.Models,
				MaxConcurrent:       100,
				Timeout:             options.timeout,
			}))
		}
	} else if options.mockProviderURL != "" {
		clientOpts = append(clientOpts, llmux.WithProvider(llmux.ProviderConfig{
			Name:                "mock-openai",
			Type:                "openai",
			APIKey:              options.mockAPIKey,
			BaseURL:             options.mockProviderURL,
			AllowPrivateBaseURL: true,
			Models:              options.models,
			MaxConcurrent:       100,
			Timeout:             options.timeout,
		}))
	} else {
		return nil, fmt.Errorf("no mock provider configured")
	}

	// Initialize store
	store := auth.NewMemoryStore()
	auditStore := auth.NewMemoryAuditLogStore()
	invitationStore := auth.NewMemoryInvitationLinkStore()

	client, err := llmux.New(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	// Initialize services
	invitationService := auth.NewInvitationService(invitationStore, store, logger)

	// Create handler
	handler := api.NewClientHandler(client, logger, &api.ClientHandlerConfig{
		Store: store,
	})
	auditLogger := auth.NewAuditLogger(auditStore, true)
	mgmtHandler := api.NewManagementHandler(store, auditStore, logger, nil, nil, auditLogger)
	invitationHandler := api.NewInvitationHandler(invitationService, invitationStore, logger)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", handler.HealthCheck)
	mux.HandleFunc("GET /health/ready", handler.HealthCheck)
	mux.HandleFunc("POST /v1/chat/completions", handler.ChatCompletions)
	mux.HandleFunc("POST /v1/completions", handler.Completions)
	mux.HandleFunc("POST /v1/embeddings", handler.Embeddings)
	mux.HandleFunc("POST /embeddings", handler.Embeddings)
	mux.HandleFunc("POST /v1/responses", handler.Responses)
	mux.HandleFunc("GET /v1/models", handler.ListModels)
	mux.Handle("GET /metrics", promhttp.Handler())

	// Register management routes
	mgmtHandler.RegisterRoutes(mux)
	invitationHandler.RegisterInvitationRoutes(mux)

	// Apply middleware
	var httpHandler http.Handler = mux

	var authMiddleware *auth.Middleware
	if options.authEnabled {
		authMiddleware = auth.NewMiddleware(&auth.MiddlewareConfig{
			Store:                  store,
			Logger:                 logger,
			SkipPaths:              cfg.Auth.SkipPaths,
			Enabled:                true,
			LastUsedUpdateInterval: cfg.Auth.LastUsedUpdateInterval,
		})
		httpHandler = authMiddleware.ModelAccessMiddleware(httpHandler)
		httpHandler = authMiddleware.Authenticate(httpHandler)
	}

	// Apply OIDC authentication middleware if configured
	if options.oidcConfig != nil && options.oidcConfig.IssuerURL != "" {
		oidcCfg := auth.OIDCConfig{
			IssuerURL:        options.oidcConfig.IssuerURL,
			ClientID:         options.oidcConfig.ClientID,
			ClientSecret:     options.oidcConfig.ClientSecret,
			RoleClaim:        options.oidcConfig.ClaimMapping.RoleClaim,
			RolesMap:         options.oidcConfig.ClaimMapping.Roles,
			UseRoleHierarchy: options.oidcConfig.ClaimMapping.UseRoleHierarchy,
			UserIDUpsert:     options.oidcConfig.UserIDUpsert,
			TeamIDUpsert:     options.oidcConfig.TeamIDUpsert,
		}

		oidcMiddleware, err := auth.OIDCMiddleware(oidcCfg)
		if err != nil {
			return nil, fmt.Errorf("create OIDC middleware: %w", err)
		}
		httpHandler = oidcMiddleware(httpHandler)
	}

	httpHandler = metrics.Middleware(httpHandler)

	// Create listener
	addr := fmt.Sprintf("127.0.0.1:%d", options.port)
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	server := &http.Server{
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &TestServer{
		server:   server,
		listener: listener,
		config:   cfg,
		baseURL:  fmt.Sprintf("http://%s", listener.Addr().String()),
		logger:   logger,
		client:   client,
		store:    store,
		pricingFile: pricingFile,
	}, nil
}

// Start starts the test server in a goroutine.
func (s *TestServer) Start() error {
	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "error", err)
		}
	}()

	// Wait for server to be ready
	return s.waitForReady(5 * time.Second)
}

// Stop gracefully shuts down the test server.
func (s *TestServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	if s.client != nil {
		_ = s.client.Close()
	}
	if s.pricingFile != "" {
		_ = os.Remove(s.pricingFile)
	}
	return err
}

// URL returns the server's base URL.
func (s *TestServer) URL() string {
	return s.baseURL
}

// Client returns an HTTP client configured for the test server.
func (s *TestServer) Client() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// Config returns the server's configuration.
func (s *TestServer) Config() *config.Config {
	return s.config
}

// Store returns the auth store.
func (s *TestServer) Store() auth.Store {
	return s.store
}

func (s *TestServer) waitForReady(timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	ctx := context.Background()

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/health/ready", http.NoBody)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("server not ready after %v", timeout)
}

func writePricingFileFromOptions(options *serverOptions) (string, error) {
	models := map[string]struct{}{}
	for _, model := range options.models {
		if model == "" {
			continue
		}
		models[model] = struct{}{}
	}
	for _, provider := range options.providers {
		for _, model := range provider.Models {
			if model == "" {
				continue
			}
			models[model] = struct{}{}
		}
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no models configured for pricing")
	}

	prices := make(map[string]map[string]any, len(models))
	for model := range models {
		prices[model] = map[string]any{
			"input_cost_per_token":  0.00001,
			"output_cost_per_token": 0.00002,
		}
	}

	file, err := os.CreateTemp("", "llmux_pricing_*.json")
	if err != nil {
		return "", fmt.Errorf("create pricing file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := json.NewEncoder(file).Encode(prices); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write pricing file: %w", err)
	}

	return file.Name(), nil
}
