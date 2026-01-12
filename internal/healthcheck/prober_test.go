package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/providers/openai"
	"github.com/stretchr/testify/require"
)

func TestProber_RunOnce_FailureSetsCooldown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	prov := openai.New(
		openai.WithBaseURL(server.URL),
		openai.WithModels("gpt-4o"),
	)
	client, err := llmux.New(
		llmux.WithProviderInstance("openai", prov, []string{"gpt-4o"}),
		llmux.WithCooldown(2*time.Minute),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	prober := NewProber(
		Config{
			Enabled:        true,
			Interval:       time.Second,
			Timeout:        time.Second,
			CooldownPeriod: 30 * time.Second,
		},
		StaticClientProvider{Client: client},
		nil,
	)

	prober.runOnce(context.Background())

	stats := client.GetStats("openai-gpt-4o")
	require.NotNil(t, stats)
	require.True(t, stats.CooldownUntil.After(time.Now()))
}

func TestProber_RunOnce_SuccessClearsCooldown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prov := openai.New(
		openai.WithBaseURL(server.URL),
		openai.WithModels("gpt-4o"),
	)
	client, err := llmux.New(
		llmux.WithProviderInstance("openai", prov, []string{"gpt-4o"}),
		llmux.WithCooldown(2*time.Minute),
	)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	err = client.SetCooldown("openai-gpt-4o", time.Now().Add(5*time.Minute))
	require.NoError(t, err)

	prober := NewProber(
		Config{
			Enabled:        true,
			Interval:       time.Second,
			Timeout:        time.Second,
			CooldownPeriod: 30 * time.Second,
		},
		StaticClientProvider{Client: client},
		nil,
	)

	prober.runOnce(context.Background())

	stats := client.GetStats("openai-gpt-4o")
	require.NotNil(t, stats)
	require.True(t, stats.CooldownUntil.IsZero())
}
