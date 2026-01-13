package routers

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
)

func TestRoundRobinRouter_Pick_RotatesInOrder(t *testing.T) {
	r := NewRoundRobinRouter()
	r.AddDeployment(&provider.Deployment{ID: "dep-a", ModelName: "gpt-4"})
	r.AddDeployment(&provider.Deployment{ID: "dep-b", ModelName: "gpt-4"})
	r.AddDeployment(&provider.Deployment{ID: "dep-c", ModelName: "gpt-4"})

	ctx := context.Background()
	picks := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		dep, err := r.Pick(ctx, "gpt-4")
		require.NoError(t, err)
		picks = append(picks, dep.ID)
	}

	assert.Equal(t, []string{"dep-a", "dep-b", "dep-c", "dep-a", "dep-b", "dep-c"}, picks)
}

func TestRoundRobinRouter_Pick_ConcurrentFairness(t *testing.T) {
	r := NewRoundRobinRouter()
	r.AddDeployment(&provider.Deployment{ID: "dep-a", ModelName: "gpt-4"})
	r.AddDeployment(&provider.Deployment{ID: "dep-b", ModelName: "gpt-4"})
	r.AddDeployment(&provider.Deployment{ID: "dep-c", ModelName: "gpt-4"})

	const goroutines = 30
	const picksPerGoroutine = 30
	total := goroutines * picksPerGoroutine

	counts := map[string]int{
		"dep-a": 0,
		"dep-b": 0,
		"dep-c": 0,
	}
	var mu sync.Mutex
	errCh := make(chan error, total)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < picksPerGoroutine; i++ {
				dep, err := r.Pick(ctx, "gpt-4")
				if err != nil {
					errCh <- err
					continue
				}
				mu.Lock()
				counts[dep.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	expected := total / 3
	assert.Equal(t, expected, counts["dep-a"])
	assert.Equal(t, expected, counts["dep-b"])
	assert.Equal(t, expected, counts["dep-c"])
}
