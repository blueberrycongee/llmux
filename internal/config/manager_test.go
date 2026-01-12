package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerStatus(t *testing.T) {
	path := writeConfigFile(t, `
server:
  port: 8080
providers:
  - name: test-provider
    type: openai
    api_key: test-key
    models:
      - gpt-4
`)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr, err := NewManager(path, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	status := mgr.Status()
	if status.Path != path {
		t.Fatalf("Status().Path = %q, want %q", status.Path, path)
	}
	if status.Checksum == "" {
		t.Fatal("Status().Checksum is empty")
	}
	if status.LoadedAt.IsZero() {
		t.Fatal("Status().LoadedAt is zero")
	}
	if status.ReloadCount == 0 {
		t.Fatal("Status().ReloadCount should be > 0")
	}
}

func TestManagerReloadUpdatesChecksum(t *testing.T) {
	path := writeConfigFile(t, `
server:
  port: 8080
providers:
  - name: test-provider
    type: openai
    api_key: test-key
    models:
      - gpt-4
`)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr, err := NewManager(path, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	before := mgr.Status()

	if err := os.WriteFile(path, []byte(`
server:
  port: 9090
providers:
  - name: test-provider
    type: openai
    api_key: test-key
    models:
      - gpt-4
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := mgr.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	after := mgr.Status()
	if after.Checksum == before.Checksum {
		t.Fatal("expected checksum to change after reload")
	}
	if after.ReloadCount != before.ReloadCount+1 {
		t.Fatalf("expected reload count %d, got %d", before.ReloadCount+1, after.ReloadCount)
	}
	if mgr.Get().Server.Port != 9090 {
		t.Fatalf("expected server port 9090, got %d", mgr.Get().Server.Port)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
