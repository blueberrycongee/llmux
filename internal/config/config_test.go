package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Server.Port)
	}

	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("default read timeout = %v, want 30s", cfg.Server.ReadTimeout)
	}

	if cfg.Routing.Strategy != "simple-shuffle" {
		t.Errorf("default strategy = %s, want simple-shuffle", cfg.Routing.Strategy)
	}

	if !cfg.Metrics.Enabled {
		t.Error("metrics should be enabled by default")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Server: ServerConfig{Port: 8080},
				Providers: []ProviderConfig{
					{Name: "openai", Type: "openai"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port zero",
			cfg: &Config{
				Server: ServerConfig{Port: 0},
				Providers: []ProviderConfig{
					{Name: "openai", Type: "openai"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port too high",
			cfg: &Config{
				Server: ServerConfig{Port: 70000},
				Providers: []ProviderConfig{
					{Name: "openai", Type: "openai"},
				},
			},
			wantErr: true,
		},
		{
			name: "no providers",
			cfg: &Config{
				Server:    ServerConfig{Port: 8080},
				Providers: []ProviderConfig{},
			},
			wantErr: true,
		},
		{
			name: "provider missing name",
			cfg: &Config{
				Server: ServerConfig{Port: 8080},
				Providers: []ProviderConfig{
					{Name: "", Type: "openai"},
				},
			},
			wantErr: true,
		},
		{
			name: "provider missing type",
			cfg: &Config{
				Server: ServerConfig{Port: 8080},
				Providers: []ProviderConfig{
					{Name: "openai", Type: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		content := `
server:
  port: 9090
  read_timeout: 10s
providers:
  - name: test-provider
    type: openai
    api_key: test-key
    models:
      - gpt-4
`
		path := createTempFile(t, content)
		defer os.Remove(path)

		cfg, err := LoadFromFile(path)
		if err != nil {
			t.Fatalf("LoadFromFile() error = %v", err)
		}

		if cfg.Server.Port != 9090 {
			t.Errorf("port = %d, want 9090", cfg.Server.Port)
		}

		if cfg.Server.ReadTimeout != 10*time.Second {
			t.Errorf("read_timeout = %v, want 10s", cfg.Server.ReadTimeout)
		}

		if len(cfg.Providers) != 1 {
			t.Fatalf("providers count = %d, want 1", len(cfg.Providers))
		}

		if cfg.Providers[0].Name != "test-provider" {
			t.Errorf("provider name = %s, want test-provider", cfg.Providers[0].Name)
		}
	})

	t.Run("environment variable expansion", func(t *testing.T) {
		os.Setenv("TEST_API_KEY", "secret-key-123")
		defer os.Unsetenv("TEST_API_KEY")

		content := `
server:
  port: 8080
providers:
  - name: openai
    type: openai
    api_key: ${TEST_API_KEY}
`
		path := createTempFile(t, content)
		defer os.Remove(path)

		cfg, err := LoadFromFile(path)
		if err != nil {
			t.Fatalf("LoadFromFile() error = %v", err)
		}

		if cfg.Providers[0].APIKey != "secret-key-123" {
			t.Errorf("api_key = %s, want secret-key-123", cfg.Providers[0].APIKey)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadFromFile("/nonexistent/path/config.yaml")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		content := `
server:
  port: [invalid
`
		path := createTempFile(t, content)
		defer os.Remove(path)

		_, err := LoadFromFile(path)
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}
