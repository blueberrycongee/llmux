package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/providers/openai"
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
	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := int(status.Load())
		if current >= http.StatusBadRequest {
			http.Error(w, "fail", current)
			return
		}
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

	status.Store(http.StatusOK)
	prober.runOnce(context.Background())

	stats = client.GetStats("openai-gpt-4o")
	require.NotNil(t, stats)
	require.True(t, stats.CooldownUntil.IsZero())
}

func TestProber_RunOnce_DoesNotShortenExistingCooldown(t *testing.T) {
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

	existing := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	require.NoError(t, client.SetCooldown("openai-gpt-4o", existing))

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
	require.True(t, stats.CooldownUntil.Equal(existing))
}

func TestProber_RunOnce_DoesNotClearExternalCooldown(t *testing.T) {
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

	existing := time.Now().Add(5 * time.Minute).Truncate(time.Second)
	require.NoError(t, client.SetCooldown("openai-gpt-4o", existing))

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
	require.True(t, stats.CooldownUntil.Equal(existing))
}
