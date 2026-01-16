package main

import (
	"fmt"
	"log/slog"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

var newPostgresStores func(*auth.PostgresConfig) (auth.Store, auth.AuditLogStore, error) = auth.NewPostgresStores
var newMemoryStore func() auth.Store = func() auth.Store {
	return auth.NewMemoryStore()
}
var newMemoryAuditStore func() auth.AuditLogStore = func() auth.AuditLogStore {
	return auth.NewMemoryAuditLogStore()
}

func initAuthStores(cfg *config.Config, logger *slog.Logger) (auth.Store, auth.AuditLogStore, error) {
	if cfg.Database.Enabled {
		postgresCfg := buildPostgresConfig(cfg.Database)
		store, auditStore, err := newPostgresStores(postgresCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("init postgres auth store: %w", err)
		}
		logger.Info("using postgres auth store",
			"host", postgresCfg.Host,
			"port", postgresCfg.Port,
			"database", postgresCfg.Database,
		)
		return store, auditStore, nil
	}

	authStore := newMemoryStore()
	auditStore := newMemoryAuditStore()
	logger.Info("using in-memory auth store (for development only)")
	return authStore, auditStore, nil
}

func initCasbin(cfg *config.Config, logger *slog.Logger) (*auth.CasbinEnforcer, error) {
	if !cfg.Auth.Casbin.Enabled {
		return nil, nil
	}

	var enforcer *auth.CasbinEnforcer
	var err error

	if cfg.Auth.Casbin.PolicyPath != "" {
		enforcer, err = auth.NewFileCasbinEnforcer(cfg.Auth.Casbin.PolicyPath)
	} else {
		// Default to in-memory enforcer with basic policies
		enforcer, err = auth.NewCasbinEnforcer(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("init casbin: %w", err)
	}

	if err := enforcer.AddDefaultPolicies(); err != nil {
		logger.Warn("failed to add default casbin policies", "error", err)
	}

	logger.Info("casbin RBAC enabled", "policy_path", cfg.Auth.Casbin.PolicyPath)
	return enforcer, nil
}

func buildPostgresConfig(dbCfg config.DatabaseConfig) *auth.PostgresConfig {
	cfg := auth.DefaultPostgresConfig()

	if dbCfg.Host != "" {
		cfg.Host = dbCfg.Host
	}
	if dbCfg.Port != 0 {
		cfg.Port = dbCfg.Port
	}
	if dbCfg.User != "" {
		cfg.User = dbCfg.User
	}
	if dbCfg.Password != "" {
		cfg.Password = dbCfg.Password
	}
	if dbCfg.Database != "" {
		cfg.Database = dbCfg.Database
	}
	if dbCfg.SSLMode != "" {
		cfg.SSLMode = dbCfg.SSLMode
	}
	if dbCfg.MaxOpenConns != 0 {
		cfg.MaxOpenConns = dbCfg.MaxOpenConns
	}
	if dbCfg.MaxIdleConns != 0 {
		cfg.MaxIdleConns = dbCfg.MaxIdleConns
	}
	if dbCfg.ConnLifetime != 0 {
		cfg.ConnLifetime = dbCfg.ConnLifetime
	}

	return cfg
}
