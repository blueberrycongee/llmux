package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/pkg/types"
)

type stubProvider struct {
	name   string
	models []string
}

func (p *stubProvider) Name() string                    { return p.name }
func (p *stubProvider) SupportedModels() []string       { return p.models }
func (p *stubProvider) SupportsModel(model string) bool { return containsModel(p.models, model) }
func (p *stubProvider) SupportEmbedding() bool          { return false }
func (p *stubProvider) ParseStreamChunk([]byte) (*types.StreamChunk, error) {
	return nil, nil
}
func (p *stubProvider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com", http.NoBody)
}
func (p *stubProvider) ParseResponse(*http.Response) (*types.ChatResponse, error) {
	return &types.ChatResponse{}, nil
}
func (p *stubProvider) MapError(statusCode int, body []byte) error {
	return &llmux.LLMError{Message: "stub error"}
}
func (p *stubProvider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, "http://example.com", http.NoBody)
}
func (p *stubProvider) ParseEmbeddingResponse(*http.Response) (*types.EmbeddingResponse, error) {
	return &types.EmbeddingResponse{}, nil
}

type deploymentListResponse struct {
	Data []struct {
		Deployment struct {
			ID           string `json:"id"`
			ProviderName string `json:"provider_name"`
			ModelName    string `json:"model_name"`
		} `json:"deployment"`
		Stats struct {
			CooldownUntil time.Time `json:"cooldown_until"`
		} `json:"stats"`
		CooldownActive bool `json:"cooldown_active"`
	} `json:"data"`
}

type providerListResponse struct {
	Data []struct {
		Provider   string `json:"provider"`
		Resilience struct {
			ConcurrentCapacity int `json:"concurrent_capacity"`
		} `json:"resilience"`
	} `json:"data"`
}

func TestControlEndpoints_DeploymentsAndCooldownAudit(t *testing.T) {
	mux, client, auditStore := newControlTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/control/deployments", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /control/deployments status = %d", rec.Code)
	}

	var deployments deploymentListResponse
	if err := json.NewDecoder(rec.Body).Decode(&deployments); err != nil {
		t.Fatalf("decode deployments: %v", err)
	}
	if len(deployments.Data) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments.Data))
	}
	deploymentID := deployments.Data[0].Deployment.ID
	if deploymentID == "" {
		t.Fatal("expected deployment id to be set")
	}

	body := []byte(`{"deployment_id":"` + deploymentID + `","cooldown_seconds":30}`)
	cooldownReq := httptest.NewRequest(http.MethodPost, "/control/deployments/cooldown", bytes.NewReader(body))
	cooldownReq.RemoteAddr = "127.0.0.1:1234"
	cooldownReq = addTestAuthContext(cooldownReq)
	cooldownRec := httptest.NewRecorder()
	mux.ServeHTTP(cooldownRec, cooldownReq)
	if cooldownRec.Code != http.StatusOK {
		t.Fatalf("POST /control/deployments/cooldown status = %d", cooldownRec.Code)
	}

	stats := client.GetStats(deploymentID)
	if stats == nil || time.Now().After(stats.CooldownUntil) {
		t.Fatal("expected cooldown to be active")
	}

	logs, total, err := auditStore.ListAuditLogs(auth.AuditLogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total == 0 || len(logs) == 0 {
		t.Fatal("expected audit log entries for cooldown change")
	}
	if logs[0].ObjectID == "" {
		t.Fatal("expected audit log object id")
	}
}

func TestControlEndpoints_ProvidersAndConfigReload(t *testing.T) {
	mux, cfgManager, auditStore, _ := newControlTestServerWithConfig(t)

	req := httptest.NewRequest(http.MethodGet, "/control/providers", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /control/providers status = %d", rec.Code)
	}

	var providers providerListResponse
	if err := json.NewDecoder(rec.Body).Decode(&providers); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(providers.Data) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers.Data))
	}
	if providers.Data[0].Provider == "" {
		t.Fatal("expected provider name")
	}

	before := cfgManager.Status()
	if err := os.WriteFile(before.Path, []byte(controlConfig("9090")), 0644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	payload := []byte(`{"expected_checksum":"` + before.Checksum + `"}`)
	reloadReq := httptest.NewRequest(http.MethodPost, "/control/config/reload", bytes.NewReader(payload))
	reloadReq.RemoteAddr = "127.0.0.1:5678"
	reloadReq = addTestAuthContext(reloadReq)
	reloadRec := httptest.NewRecorder()
	mux.ServeHTTP(reloadRec, reloadReq)
	if reloadRec.Code != http.StatusOK {
		t.Fatalf("POST /control/config/reload status = %d", reloadRec.Code)
	}

	after := cfgManager.Status()
	if after.ReloadCount <= before.ReloadCount {
		t.Fatalf("expected reload count to increase from %d", before.ReloadCount)
	}
	if after.Checksum == before.Checksum {
		t.Fatal("expected checksum to change after reload")
	}

	logs, total, err := auditStore.ListAuditLogs(auth.AuditLogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total == 0 || len(logs) == 0 {
		t.Fatal("expected audit log entries for config reload")
	}
}

func TestControlEndpoints_ConfigReloadRejectsMismatch(t *testing.T) {
	mux, cfgManager, _, _ := newControlTestServerWithConfig(t)

	before := cfgManager.Status()
	if err := os.WriteFile(before.Path, []byte(controlConfig("9091")), 0644); err != nil {
		t.Fatalf("update config: %v", err)
	}

	payload := []byte(`{"expected_checksum":"mismatch"}`)
	reloadReq := httptest.NewRequest(http.MethodPost, "/control/config/reload", bytes.NewReader(payload))
	reloadRec := httptest.NewRecorder()
	mux.ServeHTTP(reloadRec, reloadReq)
	if reloadRec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", reloadRec.Code)
	}
}

func newControlTestServer(t *testing.T) (*http.ServeMux, *llmux.Client, auth.AuditLogStore) {
	t.Helper()
	mux, _, auditStore, client := newControlTestServerWithConfig(t)
	return mux, client, auditStore
}

func newControlTestServerWithConfig(t *testing.T) (*http.ServeMux, *config.Manager, auth.AuditLogStore, *llmux.Client) {
	t.Helper()
	cfgPath := writeControlConfig(t, controlConfig("8080"))
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfgManager, err := config.NewManager(cfgPath, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	mux := http.NewServeMux()
	auditStore := auth.NewMemoryAuditLogStore()
	client, err := newStubClient()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	swapper := NewClientSwapper(client)
	store := auth.NewMemoryStore()
	auditLogger := auth.NewAuditLogger(auditStore, true)
	handler := NewManagementHandler(store, auditStore, logger, swapper, cfgManager, auditLogger)
	handler.RegisterRoutes(mux)
	return mux, cfgManager, auditStore, client
}

func newStubClient() (*llmux.Client, error) {
	provider := &stubProvider{name: "stub", models: []string{"gpt-4"}}
	return llmux.New(llmux.WithProviderInstance(provider.name, provider, provider.models))
}

func addTestAuthContext(r *http.Request) *http.Request {
	authCtx := &auth.AuthContext{
		User: &auth.User{
			ID: "user-1",
		},
	}
	ctx := auth.WithAuthContext(r.Context(), authCtx)
	ctx = observability.ContextWithRequestID(ctx, "req-test")
	return r.WithContext(ctx)
}

func writeControlConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func controlConfig(port string) string {
	return strings.TrimSpace(`
server:
  port: ` + port + `
providers:
  - name: stub
    type: openai
    api_key: test-key
    models:
      - gpt-4
`)
}

func containsModel(models []string, model string) bool {
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}
