package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

func TestInitAuthStoresMemory(t *testing.T) {
	oldPostgresStores := newPostgresStores
	oldMemoryStore := newMemoryStore
	oldMemoryAuditStore := newMemoryAuditStore
	t.Cleanup(func() {
		newPostgresStores = oldPostgresStores
		newMemoryStore = oldMemoryStore
		newMemoryAuditStore = oldMemoryAuditStore
	})

	called := false
	newPostgresStores = func(*auth.PostgresConfig) (auth.Store, auth.AuditLogStore, error) {
		called = true
		return nil, nil, errors.New("unexpected")
	}

	expectedStore := auth.NewMemoryStore()
	expectedAuditStore := auth.NewMemoryAuditLogStore()
	newMemoryStore = func() auth.Store { return expectedStore }
	newMemoryAuditStore = func() auth.AuditLogStore { return expectedAuditStore }

	cfg := &config.Config{Database: config.DatabaseConfig{Enabled: false}}
	store, auditStore, err := initAuthStores(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("initAuthStores returned error: %v", err)
	}
	if called {
		t.Fatal("expected postgres store factory not to be called")
	}
	if store != expectedStore {
		t.Fatalf("store = %v, want %v", store, expectedStore)
	}
	if auditStore != expectedAuditStore {
		t.Fatalf("audit store = %v, want %v", auditStore, expectedAuditStore)
	}
}

func TestInitAuthStoresPostgres(t *testing.T) {
	oldPostgresStores := newPostgresStores
	oldMemoryStore := newMemoryStore
	oldMemoryAuditStore := newMemoryAuditStore
	t.Cleanup(func() {
		newPostgresStores = oldPostgresStores
		newMemoryStore = oldMemoryStore
		newMemoryAuditStore = oldMemoryAuditStore
	})

	called := false
	expectedStore := auth.NewMemoryStore()
	expectedAuditStore := auth.NewMemoryAuditLogStore()
	newPostgresStores = func(*auth.PostgresConfig) (auth.Store, auth.AuditLogStore, error) {
		called = true
		return expectedStore, expectedAuditStore, nil
	}
	newMemoryStore = func() auth.Store { return auth.NewMemoryStore() }
	newMemoryAuditStore = func() auth.AuditLogStore { return auth.NewMemoryAuditLogStore() }

	cfg := &config.Config{Database: config.DatabaseConfig{Enabled: true}}
	store, auditStore, err := initAuthStores(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("initAuthStores returned error: %v", err)
	}
	if !called {
		t.Fatal("expected postgres store factory to be called")
	}
	if store != expectedStore {
		t.Fatalf("store = %v, want %v", store, expectedStore)
	}
	if auditStore != expectedAuditStore {
		t.Fatalf("audit store = %v, want %v", auditStore, expectedAuditStore)
	}
}

func TestInitAuthStoresPostgresError(t *testing.T) {
	oldPostgresStores := newPostgresStores
	t.Cleanup(func() {
		newPostgresStores = oldPostgresStores
	})

	newPostgresStores = func(*auth.PostgresConfig) (auth.Store, auth.AuditLogStore, error) {
		return nil, nil, errors.New("db down")
	}

	cfg := &config.Config{Database: config.DatabaseConfig{Enabled: true}}
	store, auditStore, err := initAuthStores(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if store != nil {
		t.Fatalf("expected nil store on error, got %v", store)
	}
	if auditStore != nil {
		t.Fatalf("expected nil audit store on error, got %v", auditStore)
	}
}

func TestBuildPostgresConfigDefaults(t *testing.T) {
	got := buildPostgresConfig(config.DatabaseConfig{})
	want := auth.DefaultPostgresConfig()

	if got.Host != want.Host {
		t.Fatalf("Host = %q, want %q", got.Host, want.Host)
	}
	if got.Port != want.Port {
		t.Fatalf("Port = %d, want %d", got.Port, want.Port)
	}
	if got.Database != want.Database {
		t.Fatalf("Database = %q, want %q", got.Database, want.Database)
	}
	if got.SSLMode != want.SSLMode {
		t.Fatalf("SSLMode = %q, want %q", got.SSLMode, want.SSLMode)
	}
	if got.MaxOpenConns != want.MaxOpenConns {
		t.Fatalf("MaxOpenConns = %d, want %d", got.MaxOpenConns, want.MaxOpenConns)
	}
	if got.MaxIdleConns != want.MaxIdleConns {
		t.Fatalf("MaxIdleConns = %d, want %d", got.MaxIdleConns, want.MaxIdleConns)
	}
	if got.ConnLifetime != want.ConnLifetime {
		t.Fatalf("ConnLifetime = %v, want %v", got.ConnLifetime, want.ConnLifetime)
	}
}

func TestBuildPostgresConfigOverrides(t *testing.T) {
	dbCfg := config.DatabaseConfig{
		Host:         "db",
		Port:         6543,
		User:         "llmux",
		Password:     "secret",
		Database:     "llmux_prod",
		SSLMode:      "require",
		MaxOpenConns: 50,
		MaxIdleConns: 10,
		ConnLifetime: 2 * time.Minute,
	}
	got := buildPostgresConfig(dbCfg)

	if got.Host != "db" {
		t.Fatalf("Host = %q, want %q", got.Host, "db")
	}
	if got.Port != 6543 {
		t.Fatalf("Port = %d, want %d", got.Port, 6543)
	}
	if got.User != "llmux" {
		t.Fatalf("User = %q, want %q", got.User, "llmux")
	}
	if got.Password != "secret" {
		t.Fatalf("Password = %q, want %q", got.Password, "secret")
	}
	if got.Database != "llmux_prod" {
		t.Fatalf("Database = %q, want %q", got.Database, "llmux_prod")
	}
	if got.SSLMode != "require" {
		t.Fatalf("SSLMode = %q, want %q", got.SSLMode, "require")
	}
	if got.MaxOpenConns != 50 {
		t.Fatalf("MaxOpenConns = %d, want %d", got.MaxOpenConns, 50)
	}
	if got.MaxIdleConns != 10 {
		t.Fatalf("MaxIdleConns = %d, want %d", got.MaxIdleConns, 10)
	}
	if got.ConnLifetime != 2*time.Minute {
		t.Fatalf("ConnLifetime = %v, want %v", got.ConnLifetime, 2*time.Minute)
	}
}
