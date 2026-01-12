package llmux

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryBackoff_Capped(t *testing.T) {
	client := newRetryTestClient(t)
	client.config.RetryBackoff = 100 * time.Millisecond
	client.config.RetryMaxBackoff = 150 * time.Millisecond
	client.config.RetryJitter = 0
	client.backoffRand = rand.New(rand.NewSource(1))

	got := client.retryBackoff(3)
	require.Equal(t, 150*time.Millisecond, got)
}

func TestRetryBackoff_JitterRange(t *testing.T) {
	client := newRetryTestClient(t)
	client.config.RetryBackoff = time.Second
	client.config.RetryMaxBackoff = 0
	client.config.RetryJitter = 0.2
	client.backoffRand = rand.New(rand.NewSource(1))

	got := client.retryBackoff(2)
	min := 1600 * time.Millisecond
	max := 2400 * time.Millisecond
	if got < min || got > max {
		t.Fatalf("backoff = %v, want between %v and %v", got, min, max)
	}
}

func newRetryTestClient(t *testing.T) *Client {
	t.Helper()

	provider := &httpMockProvider{
		name:    "primary",
		models:  []string{"test-model"},
		baseURL: "http://example.invalid",
	}

	client, err := New(
		WithProviderInstance("primary", provider, []string{"test-model"}),
		withTestPricing(t, "test-model"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}
