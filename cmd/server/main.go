// Package main is the entry point for the LLMux gateway server.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/metrics"
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
	configPath := flag.String("config", "config/config.yaml", "path to configuration file")
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

	// Initialize OpenTelemetry tracing
	tracingCfg := observability.TracingConfig{
		Enabled:     cfg.Tracing.Enabled,
		Endpoint:    cfg.Tracing.Endpoint,
		ServiceName: cfg.Tracing.ServiceName,
		SampleRate:  cfg.Tracing.SampleRate,
		Insecure:    cfg.Tracing.Insecure,
	}
	tracerProvider, err := observability.InitTracing(context.Background(), tracingCfg)
	if err != nil {
		logger.Error("failed to initialize tracing", "error", err)
	} else if cfg.Tracing.Enabled {
		logger.Info("tracing enabled", "endpoint", cfg.Tracing.Endpoint)
	}

	// Start config watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if watchErr := cfgManager.Watch(ctx); watchErr != nil {
		logger.Warn("config hot-reload disabled", "error", watchErr)
	}

	// Build llmux.Client options from config
	opts := buildClientOptions(cfg, logger, secretManager)

	// Create llmux.Client
	client, err := llmux.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create llmux client: %w", err)
	}
	defer func() { _ = client.Close() }()

	// ========================================================================
	// ENTERPRISE FEATURE INTEGRATION (P0 Fix)
	// Initialize auth stores, management handlers, and SSO sync
	// ========================================================================

	// Initialize auth Store (Memory or Postgres based on config)
	var authStore auth.Store
	var auditStore auth.AuditLogStore

	// TODO: Add PostgresStore initialization when cfg.Database.Enabled is true.
	// PostgresStore fully implements Store interface (see postgres.go, postgres_ext.go, postgres_ext2.go).
	if cfg.Database.Enabled {
		logger.Warn("database.enabled is true but PostgresStore is not wired up yet; using MemoryStore")
	}

	// Use in-memory store for development/testing
	authStore = auth.NewMemoryStore()
	auditStore = auth.NewMemoryAuditLogStore()
	logger.Info("using in-memory auth store (for development only)")

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
	_ = auditLogger // Will be used by management handlers

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

	// Initialize API handler using ClientHandler (wraps llmux.Client)
	// Now with Store integration for usage logging and budget tracking
	handlerCfg := &api.ClientHandlerConfig{
		Store: authStore,
	}
	handler := api.NewClientHandler(client, logger, handlerCfg)

	// Initialize ManagementHandler for enterprise API endpoints
	mgmtHandler := api.NewManagementHandler(authStore, auditStore, logger)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("GET /health/live", handler.HealthCheck)
	mux.HandleFunc("GET /health/ready", handler.HealthCheck)

	// OpenAI-compatible endpoints
	mux.HandleFunc("POST /v1/chat/completions", handler.ChatCompletions)
	mux.HandleFunc("GET /v1/models", handler.ListModels)

	// ========================================================================
	// MANAGEMENT ENDPOINTS REGISTRATION (P0 Fix - Critical Missing Feature)
	// Register all management routes (Key, Team, User, Organization, Audit, etc.)
	// ========================================================================
	mgmtHandler.RegisterRoutes(mux)
	logger.Info("management endpoints registered",
		"endpoints", []string{"/key/*", "/team/*", "/user/*", "/organization/*", "/spend/*", "/audit/*"})

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		mux.Handle("GET "+cfg.Metrics.Path, promhttp.Handler())
	}

	// UI Static Files
	uiFS, err := fs.Sub(uiAssets, "ui_assets")
	if err != nil {
		logger.Error("failed to load UI assets", "error", err)
	} else {
		// Serve UI at root
		mux.Handle("/", http.FileServer(http.FS(uiFS)))
	}

	// Apply middleware stack
	var httpHandler http.Handler = mux

	// API Key Authentication Middleware (enterprise feature)
	// This validates API keys and performs budget checks BEFORE requests
	if cfg.Auth.Enabled {
		authMiddleware := auth.NewMiddleware(&auth.MiddlewareConfig{
			Store:     authStore,
			Logger:    logger,
			SkipPaths: cfg.Auth.SkipPaths,
			Enabled:   true,
		})
		httpHandler = authMiddleware.Authenticate(httpHandler)
		logger.Info("API key authentication middleware enabled")
	}

	// Gateway-level Rate Limiting Middleware (P0 Fix - Critical Security Feature)
	// This is the FIRST line of defense against abuse, applied AFTER authentication
	// so we have access to API Key/Team context for per-tenant limits.
	// Note: This is separate from application-level rate limiting in client.go
	if cfg.RateLimit.Enabled {
		defaultRPM := int(cfg.RateLimit.RequestsPerMinute)
		rateLimiter := auth.NewTenantRateLimiter(&auth.TenantRateLimiterConfig{
			DefaultRPM:   defaultRPM,
			DefaultBurst: defaultRPM / 6, // ~10 seconds burst
			CleanupTTL:   10 * time.Minute,
		})

		// Inject distributed limiter if configured (for multi-instance deployments)
		if cfg.RateLimit.Distributed && cfg.Cache.Redis.Addr != "" {
			redisClient := redis.NewClient(&redis.Options{
				Addr:         cfg.Cache.Redis.Addr,
				Password:     cfg.Cache.Redis.Password,
				DB:           cfg.Cache.Redis.DB,
				DialTimeout:  cfg.Cache.Redis.DialTimeout,
				ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
				WriteTimeout: cfg.Cache.Redis.WriteTimeout,
				PoolSize:     cfg.Cache.Redis.PoolSize,
				MinIdleConns: cfg.Cache.Redis.MinIdleConns,
				MaxRetries:   cfg.Cache.Redis.MaxRetries,
			})

			// Test connection before using
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := redisClient.Ping(pingCtx).Err(); err != nil {
				logger.Warn("distributed rate limiting unavailable, using local limiter", "error", err)
			} else {
				distributedLimiter := resilience.NewRedisLimiter(redisClient)
				rateLimiter.SetDistributedLimiter(distributedLimiter)
				logger.Info("gateway rate limiting using distributed Redis backend")
			}
			pingCancel()
		}

		httpHandler = rateLimiter.RateLimitMiddleware(httpHandler)
		logger.Info("gateway rate limiting enabled",
			"default_rpm", cfg.RateLimit.RequestsPerMinute,
			"distributed", cfg.RateLimit.Distributed,
		)
	}

	// OIDC Middleware (SSO authentication with sync integration)
	if cfg.Auth.Enabled && cfg.Auth.OIDC.IssuerURL != "" {
		oidcCfg := auth.OIDCConfig{
			IssuerURL:    cfg.Auth.OIDC.IssuerURL,
			ClientID:     cfg.Auth.OIDC.ClientID,
			ClientSecret: cfg.Auth.OIDC.ClientSecret,
			RoleClaim:    cfg.Auth.OIDC.ClaimMapping.RoleClaim,
			RolesMap:     cfg.Auth.OIDC.ClaimMapping.Roles,
		}
		// Use OIDCMiddlewareWithSync instead of OIDCMiddleware
		// This injects the syncer to enable automatic user-team sync from JWT claims
		oidcMiddleware, err := auth.OIDCMiddlewareWithSync(oidcCfg, syncer)
		if err != nil {
			return fmt.Errorf("failed to initialize OIDC middleware: %w", err)
		}
		httpHandler = oidcMiddleware(httpHandler)
		logger.Info("OIDC authentication enabled", "issuer", cfg.Auth.OIDC.IssuerURL, "sync_enabled", syncer != nil)
	}

	httpHandler = metrics.Middleware(httpHandler)
	httpHandler = observability.RequestIDMiddleware(httpHandler)

	// CORS middleware for development (Next.js on :3000)
	httpHandler = corsMiddleware(httpHandler)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

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

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Shutdown tracing
	if tracerProvider != nil {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			logger.Error("tracer shutdown error", "error", err)
		}
	}

	logger.Info("server stopped")
	return nil
}

// buildClientOptions converts config.Config to llmux.Option slice.
func buildClientOptions(cfg *config.Config, logger *slog.Logger, secretManager *secret.Manager) []llmux.Option {
	// Pre-allocate with estimated capacity
	opts := make([]llmux.Option, 0, len(cfg.Providers)+6)

	// Add logger
	opts = append(opts, llmux.WithLogger(logger))

	// Add providers from config
	for _, provCfg := range cfg.Providers {
		pCfg := llmux.ProviderConfig{
			Name:    provCfg.Name,
			Type:    provCfg.Type,
			APIKey:  provCfg.APIKey,
			BaseURL: provCfg.BaseURL,
			Models:  provCfg.Models,
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

	// Set routing strategy
	strategy := mapRoutingStrategy(cfg.Routing.Strategy)
	opts = append(opts, llmux.WithRouterStrategy(strategy))

	// Set cooldown period
	if cfg.Routing.CooldownPeriod > 0 {
		opts = append(opts, llmux.WithCooldown(cfg.Routing.CooldownPeriod))
	}

	// Set timeout
	if cfg.Server.WriteTimeout > 0 {
		opts = append(opts, llmux.WithTimeout(cfg.Server.WriteTimeout))
	}

	// Enable retry and fallback
	opts = append(opts,
		llmux.WithRetry(3, 100*time.Millisecond),
		llmux.WithFallback(true),
	)

	// Set pricing file
	if cfg.PricingFile != "" {
		opts = append(opts, llmux.WithPricingFile(cfg.PricingFile))
	}

	// Initialize distributed routing
	if cfg.Routing.Distributed {
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			var redisClient *redis.Client

			if len(cfg.Cache.Redis.ClusterAddrs) > 0 {
				logger.Warn("Redis cluster mode not fully supported for routing stats, using single-node mode")
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.ClusterAddrs[0],
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			} else {
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.Addr,
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			}

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Error("failed to connect to Redis for distributed routing", "error", err)
			} else {
				statsStore := routers.NewRedisStatsStore(redisClient)
				opts = append(opts, llmux.WithStatsStore(statsStore))
				logger.Info("distributed routing enabled", "redis_addr", cfg.Cache.Redis.Addr)
			}
		} else {
			logger.Warn("distributed routing enabled but no Redis configured")
		}
	}

	// Initialize distributed rate limiting
	if cfg.RateLimit.Enabled && cfg.RateLimit.Distributed {
		// Use Redis from Cache config for distributed rate limiting
		if cfg.Cache.Redis.Addr != "" || len(cfg.Cache.Redis.ClusterAddrs) > 0 {
			var redisClient *redis.Client

			if len(cfg.Cache.Redis.ClusterAddrs) > 0 {
				// For cluster mode, we need ClusterClient, but RedisLimiter uses *redis.Client
				// For now, use single node. Full cluster support requires updating RedisLimiter.
				logger.Warn("Redis cluster mode not fully supported for rate limiting, using single-node mode")
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.ClusterAddrs[0],
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			} else {
				redisClient = redis.NewClient(&redis.Options{
					Addr:         cfg.Cache.Redis.Addr,
					Password:     cfg.Cache.Redis.Password,
					DB:           cfg.Cache.Redis.DB,
					DialTimeout:  cfg.Cache.Redis.DialTimeout,
					ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
					WriteTimeout: cfg.Cache.Redis.WriteTimeout,
					PoolSize:     cfg.Cache.Redis.PoolSize,
					MinIdleConns: cfg.Cache.Redis.MinIdleConns,
					MaxRetries:   cfg.Cache.Redis.MaxRetries,
				})
			}

			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				logger.Error("failed to connect to Redis for rate limiting", "error", err)
			} else {
				limiter := resilience.NewRedisLimiter(redisClient)
				opts = append(opts, llmux.WithRateLimiter(limiter))
				logger.Info("distributed rate limiting enabled",
					"redis_addr", cfg.Cache.Redis.Addr,
					"rpm_limit", cfg.RateLimit.RequestsPerMinute,
					"tpm_limit", cfg.RateLimit.TokensPerMinute,
				)
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
		}))
	}

	return opts
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

// corsMiddleware adds CORS headers for development (Next.js frontend on :3000)
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from Next.js dev server
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
