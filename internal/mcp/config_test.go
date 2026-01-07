package mcp

import (
	"testing"
	"time"
)

func TestClientConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr bool
	}{
		{
			name: "valid HTTP config",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: ConnectionTypeHTTP,
				URL:  "http://localhost:3000",
			},
			wantErr: false,
		},
		{
			name: "valid STDIO config",
			cfg: ClientConfig{
				ID:      "test",
				Name:    "Test",
				Type:    ConnectionTypeSTDIO,
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
			},
			wantErr: false,
		},
		{
			name: "valid SSE config",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: ConnectionTypeSSE,
				URL:  "http://localhost:3000/sse",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			cfg: ClientConfig{
				Name: "Test",
				Type: ConnectionTypeHTTP,
				URL:  "http://localhost:3000",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			cfg: ClientConfig{
				ID:   "test",
				Type: ConnectionTypeHTTP,
				URL:  "http://localhost:3000",
			},
			wantErr: true,
		},
		{
			name: "missing type",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				URL:  "http://localhost:3000",
			},
			wantErr: true,
		},
		{
			name: "HTTP missing URL",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: ConnectionTypeHTTP,
			},
			wantErr: true,
		},
		{
			name: "STDIO missing command",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: ConnectionTypeSTDIO,
			},
			wantErr: true,
		},
		{
			name: "SSE missing URL",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: ConnectionTypeSSE,
			},
			wantErr: true,
		},
		{
			name: "unknown type",
			cfg: ClientConfig{
				ID:   "test",
				Name: "Test",
				Type: "unknown",
			},
			wantErr: true,
		},
		{
			name: "negative connection timeout",
			cfg: ClientConfig{
				ID:                "test",
				Name:              "Test",
				Type:              ConnectionTypeHTTP,
				URL:               "http://localhost:3000",
				ConnectionTimeout: -1 * time.Second,
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

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "disabled config skips validation",
			cfg: Config{
				Enabled: false,
				Clients: []ClientConfig{
					{}, // Invalid but should be skipped
				},
			},
			wantErr: false,
		},
		{
			name: "valid enabled config",
			cfg: Config{
				Enabled: true,
				Clients: []ClientConfig{
					{
						ID:   "test",
						Name: "Test",
						Type: ConnectionTypeHTTP,
						URL:  "http://localhost:3000",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate client IDs",
			cfg: Config{
				Enabled: true,
				Clients: []ClientConfig{
					{
						ID:   "test",
						Name: "Test1",
						Type: ConnectionTypeHTTP,
						URL:  "http://localhost:3000",
					},
					{
						ID:   "test",
						Name: "Test2",
						Type: ConnectionTypeHTTP,
						URL:  "http://localhost:3001",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid client config",
			cfg: Config{
				Enabled: true,
				Clients: []ClientConfig{
					{
						ID: "test",
						// Missing Name and Type
					},
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

func TestClientConfigGetTimeout(t *testing.T) {
	defaultTimeout := 30 * time.Second

	t.Run("uses default when not set", func(t *testing.T) {
		cfg := ClientConfig{}
		if got := cfg.GetConnectionTimeout(defaultTimeout); got != defaultTimeout {
			t.Errorf("GetConnectionTimeout() = %v, want %v", got, defaultTimeout)
		}
		if got := cfg.GetExecutionTimeout(defaultTimeout); got != defaultTimeout {
			t.Errorf("GetExecutionTimeout() = %v, want %v", got, defaultTimeout)
		}
	})

	t.Run("uses custom when set", func(t *testing.T) {
		customTimeout := 60 * time.Second
		cfg := ClientConfig{
			ConnectionTimeout: customTimeout,
			ExecutionTimeout:  customTimeout,
		}
		if got := cfg.GetConnectionTimeout(defaultTimeout); got != customTimeout {
			t.Errorf("GetConnectionTimeout() = %v, want %v", got, customTimeout)
		}
		if got := cfg.GetExecutionTimeout(defaultTimeout); got != customTimeout {
			t.Errorf("GetExecutionTimeout() = %v, want %v", got, customTimeout)
		}
	})
}
