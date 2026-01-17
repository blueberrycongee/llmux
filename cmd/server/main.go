// Package main is the entry point for the LLMux gateway server.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/healthcheck"
	"github.com/blueberrycongee/llmux/internal/mcp"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/blueberrycongee/llmux/internal/secret"
	"github.com/blueberrycongee/llmux/internal/secret/env"
	"github.com/blueberrycongee/llmux/internal/secret/vault"
	"github.com/blueberrycongee/llmux/routers"
)

//go:embed all:ui_assets
var uiAssets embed.FS

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.example.yaml", "path to configuration file")
	flag.Parse()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting LLMux gateway", "version", "0.1.0")

	// Initialize Secret Manager
	secretManager := secret.NewManager()
	defer func() {
		if err := secretManager.Close(); err != nil {
			logger.Error("failed to close secret manager", "error", err)
		}
	}()

	// Register 'env' provider
	secretManager.Register("env", env.New())

	// Load configuration
	cfgManager, err := config.NewManager(*configPath, logger)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	defer func() { _ = cfgManager.Close() }()

	cfg := cfgManager.Get()
	for _, w := range cfg.Warnings() {
		logger.Warn(w.Message, "code", w.Code)
	}

	// Register 'vault' provider if configured
	var vConfig vault.Config
	if cfg.Vault.Enabled {
		vConfig = vault.Config{
			Address:    cfg.Vault.Address,
			AuthMethod: cfg.Vault.AuthMethod,
			RoleID:     cfg.Vault.RoleID,
			SecretID:   cfg.Vault.SecretID,
			CACert:     cfg.Vault.CACert,
			ClientCert: cfg.Vault.ClientCert,
			ClientKey:  cfg.Vault.ClientKey,
		}
	} else if os.Getenv("VAULT_ADDR") != "" {
		// Backward compatibility: Construct from Env
		vConfig = vault.Config{
			Address:    os.Getenv("VAULT_ADDR"),
			AuthMethod: "approle", // Default for env var legacy
			RoleID:     os.Getenv("VAULT_ROLE_ID"),
			SecretID:   os.Getenv("VAULT_SECRET_ID"),
		}
	}

	if vConfig.Address != "" {
		logger.Info("initializing vault secret provider", "addr", vConfig.Address, "auth_method", vConfig.AuthMethod)
		vProvider, vErr := vault.New(vConfig)
		if vErr != nil {
			return fmt.Errorf("failed to initialize vault provider: %w", vErr)
		}
		// Wrap with cache (TTL 5 minutes)
		cachedVault := secret.NewCachedProvider(vProvider, 5*time.Minute)
		secretManager.Register("vault", cachedVault)
	} else {
		logger.Info("vault provider disabled")
	}

	// Initialize observability manager
	obsCfg := cfg.Observability
	if cfg.Tracing.Enabled && !obsCfg.OpenTelemetry.Enabled {
		obsCfg.OpenTelemetry = observability.TracingConfig{
			Enabled:      true,
			Endpoint:     cfg.Tracing.Endpoint,
			ExporterType: observability.ExporterGRPC,
			ServiceName:  cfg.Tracing.ServiceName,
			SampleRate:   cfg.Tracing.SampleRate,
			Insecure:     cfg.Tracing.Insecure,
		}
	}
	if obsCfg.OpenTelemetry.Enabled && !hasCallback(obsCfg.EnabledCallbacks, "otel", "opentelemetry") {
		obsCfg.EnabledCallbacks = append(obsCfg.EnabledCallbacks, "otel")
	}
	obsMgr, err := observability.NewObservabilityManager(obsCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize observability: %w", err)
	}
	if len(obsCfg.EnabledCallbacks) > 0 {
		logger.Info("observability callbacks enabled", "callbacks", obsCfg.EnabledCallbacks)
	}

	// Start config watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build llmux.Client options from config
	opts := buildClientOptions(cfg, logger, secretManager, obsMgr)

	// Create llmux.Client
	client, err := llmux.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create llmux client: %w", err)
	}
	clientSwapper := api.NewClientSwapper(client)
	defer clientSwapper.Close()

	reloader := newClientReloader(logger, clientSwapper, func(nextCfg *config.Config) (*llmux.Client, error) {
		nextOpts := buildClientOptions(nextCfg, logger, secretManager, obsMgr)
		return llmux.New(nextOpts...)
	})
	cfgManager.OnChange(reloader.Reload)
	cfgManager.OnChange(func(nextCfg *config.Config) {
		for _, w := range nextCfg.Warnings() {
			logger.Warn(w.Message, "code", w.Code)
		}
	})

	if watchErr := cfgManager.Watch(ctx); watchErr != nil {
		logger.Warn("config hot-reload disabled", "error", watchErr)
	}

	if cfg.HealthCheck.Enabled {
		proberCfg := healthcheck.Config{
			Enabled:        true,
			Interval:       cfg.HealthCheck.Interval,
			Timeout:        cfg.HealthCheck.Timeout,
			CooldownPeriod: cfg.Routing.CooldownPeriod,
		}
		prober := healthcheck.NewProber(proberCfg, swapperClientProvider{swapper: clientSwapper}, logger)
		prober.Start(ctx)
		logger.Info("healthcheck prober started",
			"interval", proberCfg.Interval,
			"timeout", proberCfg.Timeout,
			"cooldown_period", proberCfg.CooldownPeriod,
		)
	}

	// ========================================================================
	// ENTERPRISE FEATURE INTEGRATION (P0 Fix)
	// Initialize auth stores, management handlers, and SSO sync
	// ========================================================================

	// Initialize auth Store (Memory or Postgres based on config)
	authStore, auditStore, err := initAuthStores(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize auth stores: %w", err)
	}

	// Ensure store is closed on shutdown
	defer func() {
		if authStore != nil {
			if closeErr := authStore.Close(); closeErr != nil {
				logger.Error("failed to close auth store", "error", closeErr)
			}
		}
	}()

	// Create AuditLogger
	auditLogger := auth.NewAuditLogger(auditStore, true)

	// Initialize Casbin RBAC
	enforcer, err := initCasbin(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize casbin: %w", err)
	}

	if provider, ok := authStore.(dbStatsProvider); ok {
		stopMetrics := startDBPoolMetrics(ctx, provider, logger, 30*time.Second)
		if stopMetrics != nil {
			defer stopMetrics()
		}
	}

	runner := startJobRunner(cfg, authStore, logger, nil)
	if runner != nil {
		defer runner.Stop()
	}

	governanceEngine := buildGovernanceEngine(cfg, authStore, auditLogger, logger, enforcer)
	if governanceEngine != nil {
		cfgManager.OnChange(func(nextCfg *config.Config) {
			governanceEngine.UpdateConfig(mapGovernanceConfig(nextCfg.Governance))
		})
	}

	// Initialize UserTeamSyncer for SSO user-team synchronization
	var syncer *auth.UserTeamSyncer
	if cfg.Auth.Enabled && cfg.Auth.OIDC.UserTeamSync.Enabled {
		syncCfg := auth.UserTeamSyncConfig{
			Enabled:                 cfg.Auth.OIDC.UserTeamSync.Enabled,
			AutoCreateUsers:         cfg.Auth.OIDC.UserTeamSync.AutoCreateUsers,
			AutoCreateTeams:         cfg.Auth.OIDC.UserTeamSync.AutoCreateTeams,
			RemoveFromUnlistedTeams: cfg.Auth.OIDC.UserTeamSync.RemoveFromUnlistedTeams,
			SyncUserRole:            cfg.Auth.OIDC.UserTeamSync.SyncUserRole,
			DefaultRole:             cfg.Auth.OIDC.UserTeamSync.DefaultRole,
			DefaultOrganizationID:   cfg.Auth.OIDC.UserTeamSync.DefaultOrganizationID,
		}
		syncer = auth.NewUserTeamSyncer(authStore, syncCfg, logger)
		logger.Info("user-team sync enabled", "auto_create_users", syncCfg.AutoCreateUsers)
	}

	sessionManager, err := buildSessionManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	// Initialize MCP Manager
	var mcpManager mcp.Manager
	if cfg.MCP.Enabled {
		mcpCfg := mcp.FromConfig(cfg.MCP)
		manager, mcpErr := mcp.NewManager(ctx, mcpCfg, logger)
		if mcpErr != nil {
			return fmt.Errorf("failed to initialize MCP manager: %w", mcpErr)
		}
		mcpManager = manager
		defer func() {
			if closeErr := mcpManager.Close(); closeErr != nil {
				logger.Error("failed to close MCP manager", "error", closeErr)
			}
		}()
	}

	// Initialize API handler using ClientHandler (wraps llmux.Client)
	// Now with Store integration for usage logging and budget tracking
	handlerCfg := &api.ClientHandlerConfig{
		Store:         authStore,
		MCPManager:    mcpManager,
		Observability: obsMgr,
		Governance:    governanceEngine,
	}
	handler := api.NewClientHandlerWithSwapper(clientSwapper, logger, handlerCfg)

	// Initialize ManagementHandler for enterprise API endpoints
	mgmtHandler := api.NewManagementHandler(authStore, auditStore, logger, clientSwapper, cfgManager, auditLogger)

	// Initialize Invitation endpoints (LiteLLM-compatible enterprise surface)
	var invitationStore auth.InvitationLinkStore
	if pg, ok := authStore.(*auth.PostgresStore); ok {
		invitationStore = pg
	} else {
		invitationStore = auth.NewMemoryInvitationLinkStore()
	}
	invitationService := auth.NewInvitationService(invitationStore, authStore, logger)
	invitationHandler := api.NewInvitationHandler(invitationService, invitationStore, logger)

	authHandler, err := api.NewAuthHandler(mapOIDCConfig(cfg.Auth.OIDC), sessionManager, syncer, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize auth handler: %w", err)
	}

	// Setup HTTP routes
	muxes, err := buildMuxes(cfg, handler, multiRegistrar{mgmtHandler, invitationHandler, authHandler}, logger, uiAssets)
	if err != nil {
		return fmt.Errorf("failed to build routes: %w", err)
	}

	if mcpManager != nil {
		mcpHandler := mcp.NewHTTPHandler(mcpManager)
		if muxes.Admin != nil {
			mcpHandler.RegisterRoutes(muxes.Admin)
			logger.Info("MCP management endpoints registered",
				"endpoints", []string{"/mcp/clients", "/mcp/clients/{id}", "/mcp/tools"},
				"admin_port", cfg.Server.AdminPort,
			)
		} else {
			logger.Warn("MCP management endpoints disabled (set server.admin_port to enable)",
				"endpoints", []string{"/mcp/clients", "/mcp/clients/{id}", "/mcp/tools"},
			)
		}
	}

	if muxes.Admin != nil {
		logger.Info("management endpoints registered",
			"endpoints", []string{"/key/*", "/team/*", "/user/*", "/organization/*", "/spend/*", "/audit/*"},
			"admin_port", cfg.Server.AdminPort,
		)
	} else {
		logger.Warn("management endpoints disabled (set server.admin_port to enable)",
			"endpoints", []string{"/key/*", "/team/*", "/user/*", "/organization/*", "/spend/*", "/audit/*"},
		)
	}

	middleware, err := buildMiddlewareStack(cfg, authStore, logger, syncer, enforcer, sessionManager)
	if err != nil {
		return fmt.Errorf("failed to initialize middleware stack: %w", err)
	}
	if mcpManager != nil {
		next := middleware
		middleware = func(h http.Handler) http.Handler {
			return mcp.Middleware(mcpManager)(next(h))
		}
	}

	dataHandler := middleware(muxes.Data)

	// Create data server
	dataServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      dataHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	var adminServer *http.Server
	if muxes.Admin != nil {
		adminHandler := middleware(muxes.Admin)
		adminServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Server.AdminPort),
			Handler:      adminHandler,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		}
	}

	// Start server(s) in goroutines
	serverErr := make(chan error, 2)
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := dataServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()
	if adminServer != nil {
		go func() {
			logger.Info("admin server listening", "port", cfg.Server.AdminPort)
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr <- err
			}
		}()
	}

	// Wait for shutdown signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("shutting down server...")
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	if err := dataServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	if adminServer != nil {
		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("admin server shutdown error", "error", err)
		}
	}

	// Shutdown observability
	if obsMgr != nil {
		if err := obsMgr.Shutdown(shutdownCtx); err != nil {
			logger.Error("observability shutdown error", "error", err)
		}
	}

	logger.Info("server stopped")
	return nil
}

func hasCallback(callbacks []string, names ...string) bool {
	for _, cb := range callbacks {
		for _, name := range names {
			if strings.EqualFold(cb, name) {
				return true
			}
		}
	}
	return false
}

// buildRoutingOptions converts routing-related config to llmux.Option slice.
func buildRoutingOptions(cfg *config.Config) []llmux.Option {
	opts := make([]llmux.Option, 0, 4)

	strategy := mapRoutingStrategy(cfg.Routing.Strategy)
	opts = append(opts, llmux.WithRouterStrategy(strategy))

	if cfg.Routing.DefaultProvider != "" {
		opts = append(opts, llmux.WithDefaultProvider(cfg.Routing.DefaultProvider))
	}

	if cfg.Routing.CooldownPeriod > 0 {
		opts = append(opts, llmux.WithCooldown(cfg.Routing.CooldownPeriod))
	}

	if cfg.Routing.EWMAAlpha > 0 {
		opts = append(opts, llmux.WithEWMAAlpha(cfg.Routing.EWMAAlpha))
	}

	if cfg.Server.WriteTimeout > 0 {
		opts = append(opts, llmux.WithTimeout(cfg.Server.WriteTimeout))
	}

	opts = append(opts,
		llmux.WithRetry(cfg.Routing.RetryCount, cfg.Routing.RetryBackoff),
		llmux.WithRetryMaxBackoff(cfg.Routing.RetryMaxBackoff),
		llmux.WithRetryJitter(cfg.Routing.RetryJitter),
		llmux.WithFallback(cfg.Routing.FallbackEnabled),
	)

	return opts
}

// buildClientOptions converts config.Config to llmux.Option slice.
func buildClientOptions(cfg *config.Config, logger *slog.Logger, secretManager *secret.Manager, obsMgr *observability.ObservabilityManager) []llmux.Option {
	// Pre-allocate with estimated capacity
	opts := make([]llmux.Option, 0, len(cfg.Providers)+6)

	// Add logger
	opts = append(opts, llmux.WithLogger(logger))

	// Add providers from config
	for _, provCfg := range cfg.Providers {
		pCfg := llmux.ProviderConfig{
			Name:                provCfg.Name,
			Type:                provCfg.Type,
			APIKey:              provCfg.APIKey,
			BaseURL:             provCfg.BaseURL,
			AllowPrivateBaseURL: provCfg.AllowPrivateBaseURL,
			Models:              provCfg.Models,
			Timeout:             provCfg.Timeout,
			// MaxConcurrent is enforced by the client semaphore per deployment.
			MaxConcurrent: provCfg.MaxConcurrent,
			Headers:       provCfg.Headers,
		}

		// Check if APIKey is a secret URI (contains "://")
		if strings.Contains(provCfg.APIKey, "://") {
			pCfg.TokenSource = &SecretTokenSource{
				mgr:  secretManager,
				path: provCfg.APIKey,
			}
		}

		opts = append(opts, llmux.WithProvider(pCfg))
	}

	opts = append(opts, buildRoutingOptions(cfg)...)
	if obsMgr != nil {
		opts = append(opts, llmux.WithFallbackReporter(obsMgr.LogFallback))
	}

	// Set pricing file
	if cfg.PricingFile != "" {
		opts = append(opts, llmux.WithPricingFile(cfg.PricingFile))
	}

	// Stream recovery mode
	if cfg.Stream.RecoveryMode != "" {
		opts = append(opts, llmux.WithStreamRecoveryMode(mapStreamRecoveryMode(cfg.Stream.RecoveryMode)))
	}
	opts = append(opts, llmux.WithStreamRecoveryMaxAccumulatedBytes(cfg.Stream.MaxAccumulatedBytes))

	// Initialize cache
	cacheOpts, cacheErr := buildCacheOptions(&cfg.Cache, logger)
	if cacheErr != nil {
		logger.Warn("failed to initialize cache, disabling", "error", cacheErr)
	} else if len(cacheOpts) > 0 {
		opts = append(opts, cacheOpts...)
	}

	// Initialize distributed routing
	if cfg.Routing.Distributed {
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			redisClient, isCluster, err := newRedisUniversalClient(cfg.Cache.Redis)
			if err != nil {
				logger.Error("failed to initialize Redis for distributed routing", "error", err)
			} else {
				// Test Redis connection
				pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := redisClient.Ping(pingCtx).Err(); err != nil {
					logger.Error("failed to connect to Redis for distributed routing", "error", err)
				} else {
					statsStore := routers.NewRedisStatsStore(redisClient)
					opts = append(opts, llmux.WithStatsStore(statsStore))
					rrStore := routers.NewRedisRoundRobinStore(redisClient)
					opts = append(opts, llmux.WithRoundRobinStore(rrStore))
					logger.Info("distributed routing enabled", "cluster", isCluster)
				}
				pingCancel()
			}
		} else {
			logger.Warn("distributed routing enabled but no Redis configured")
		}
	}

	// Initialize distributed rate limiting
	if cfg.RateLimit.Enabled && cfg.RateLimit.Distributed {
		// Use Redis from Cache config for distributed rate limiting
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			redisClient, isCluster, err := newRedisUniversalClient(cfg.Cache.Redis)
			if err != nil {
				logger.Error("failed to initialize Redis for rate limiting", "error", err)
			} else {
				// Test Redis connection
				pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := redisClient.Ping(pingCtx).Err(); err != nil {
					logger.Error("failed to connect to Redis for rate limiting", "error", err)
				} else {
					limiter := resilience.NewRedisLimiter(redisClient)
					opts = append(opts, llmux.WithRateLimiter(limiter))
					logger.Info("distributed rate limiting enabled",
						"cluster", isCluster,
						"rpm_limit", cfg.RateLimit.RequestsPerMinute,
						"tpm_limit", cfg.RateLimit.TokensPerMinute,
					)
				}
				pingCancel()
			}
		} else {
			logger.Warn("distributed rate limiting enabled but no Redis configured")
		}
	}

	// Set rate limiter config
	if cfg.RateLimit.Enabled {
		windowSize := cfg.RateLimit.WindowSize
		if windowSize == 0 {
			windowSize = time.Minute
		}
		opts = append(opts, llmux.WithRateLimiterConfig(llmux.RateLimiterConfig{
			Enabled:     cfg.RateLimit.Enabled,
			RPMLimit:    cfg.RateLimit.RequestsPerMinute,
			TPMLimit:    cfg.RateLimit.TokensPerMinute,
			WindowSize:  windowSize,
			KeyStrategy: mapKeyStrategy(cfg.RateLimit.KeyStrategy),
			FailOpen:    cfg.RateLimit.FailOpen,
		}))
	}

	return opts
}

// mapStreamRecoveryMode converts config recovery mode to llmux.StreamRecoveryMode.
func mapStreamRecoveryMode(mode string) llmux.StreamRecoveryMode {
	switch mode {
	case "off":
		return llmux.StreamRecoveryOff
	case "append":
		return llmux.StreamRecoveryAppend
	case "retry":
		return llmux.StreamRecoveryRetry
	default:
		return llmux.StreamRecoveryRetry
	}
}

func newRedisUniversalClient(cfg config.RedisCacheConfig) (redis.UniversalClient, bool, error) {
	addrs := cfg.ClusterAddrs
	isCluster := len(addrs) > 0
	if len(addrs) == 0 && cfg.Addr != "" {
		addrs = []string{cfg.Addr}
	}
	if len(addrs) == 0 {
		return nil, false, fmt.Errorf("redis address not configured")
	}

	options := &redis.UniversalOptions{
		Addrs:        addrs,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
	}
	if isCluster {
		options.IsClusterMode = true
	}

	return redis.NewUniversalClient(options), isCluster, nil
}

// mapKeyStrategy converts config key strategy string to llmux.RateLimitKeyStrategy.
func mapKeyStrategy(strategy string) llmux.RateLimitKeyStrategy {
	switch strategy {
	case "api_key":
		return llmux.RateLimitKeyByAPIKey
	case "user":
		return llmux.RateLimitKeyByUser
	case "model":
		return llmux.RateLimitKeyByModel
	case "api_key_model":
		return llmux.RateLimitKeyByAPIKeyAndModel
	default:
		return llmux.RateLimitKeyByAPIKey
	}
}

// mapRoutingStrategy converts config strategy string to llmux.Strategy.
func mapRoutingStrategy(strategy string) llmux.Strategy {
	switch strategy {
	case "shuffle", "random":
		return llmux.StrategyShuffle
	case "round-robin", "roundrobin":
		return llmux.StrategyRoundRobin
	case "lowest-latency", "latency":
		return llmux.StrategyLowestLatency
	case "least-busy", "leastbusy":
		return llmux.StrategyLeastBusy
	case "lowest-tpm-rpm", "tpm-rpm":
		return llmux.StrategyLowestTPMRPM
	case "lowest-cost", "cost":
		return llmux.StrategyLowestCost
	default:
		return llmux.StrategyShuffle
	}
}

// SecretTokenSource adapts secret.Manager to provider.TokenSource interface.
type SecretTokenSource struct {
	mgr  *secret.Manager
	path string
}

// Token retrieves the secret value using the secret manager.
func (s *SecretTokenSource) Token() (string, error) {
	// Use background context as TokenSource interface doesn't support context
	return s.mgr.Get(context.Background(), s.path)
}
