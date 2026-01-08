// Package runner provides benchmark execution and result collection.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
)

// Config holds benchmark configuration.
type Config struct {
	Target      string        // Target URL
	Requests    int           // Total number of requests
	Concurrency int           // Number of concurrent workers
	Duration    time.Duration // Duration to run (0 = use Requests)
	Name        string        // Benchmark name
}

// Result holds benchmark results.
type Result struct {
	Name        string        `json:"name"`
	Target      string        `json:"target"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Requests    int           `json:"requests"`
	Concurrency int           `json:"concurrency"`

	// Performance metrics
	TotalRequests   int64         `json:"total_requests"`
	SuccessRequests int64         `json:"success_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	RPS             float64       `json:"rps"`
	LatencyMin      time.Duration `json:"latency_min"`
	LatencyMax      time.Duration `json:"latency_max"`
	LatencyMean     time.Duration `json:"latency_mean"`
	LatencyP50      time.Duration `json:"latency_p50"`
	LatencyP95      time.Duration `json:"latency_p95"`
	LatencyP99      time.Duration `json:"latency_p99"`

	// All latencies for percentile calculation
	Latencies []time.Duration `json:"-"`
}

// Runner executes benchmarks.
type Runner struct {
	client *http.Client
	config Config
}

// NewRunner creates a new benchmark runner.
func NewRunner(cfg Config) *Runner {
	return &Runner{
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Concurrency * 2,
				MaxIdleConnsPerHost: cfg.Concurrency * 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		config: cfg,
	}
}

// Run executes the benchmark and returns results.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	result := &Result{
		Name:        r.config.Name,
		Target:      r.config.Target,
		StartTime:   time.Now(),
		Requests:    r.config.Requests,
		Concurrency: r.config.Concurrency,
		Latencies:   make([]time.Duration, 0, r.config.Requests),
	}

	var (
		successCount atomic.Int64
		failedCount  atomic.Int64
		latencies    = make(chan time.Duration, r.config.Requests)
		wg           sync.WaitGroup
	)

	// Create request body
	reqBody := map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, this is a benchmark test."},
		},
		"stream": false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Worker function
	worker := func(requests <-chan struct{}) {
		defer wg.Done()
		for range requests {
			start := time.Now()
			err := r.sendRequest(ctx, bodyBytes)
			elapsed := time.Since(start)

			if err != nil {
				failedCount.Add(1)
			} else {
				successCount.Add(1)
				latencies <- elapsed
			}
		}
	}

	// Create request channel
	requests := make(chan struct{}, r.config.Requests)

	// Start workers
	for i := 0; i < r.config.Concurrency; i++ {
		wg.Add(1)
		go worker(requests)
	}

	// Send requests
sendLoop:
	for i := 0; i < r.config.Requests; i++ {
		select {
		case requests <- struct{}{}:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(requests)

	// Wait for workers
	wg.Wait()
	close(latencies)

	// Collect latencies
	for lat := range latencies {
		result.Latencies = append(result.Latencies, lat)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.TotalRequests = successCount.Load() + failedCount.Load()
	result.SuccessRequests = successCount.Load()
	result.FailedRequests = failedCount.Load()

	// Calculate metrics
	r.calculateMetrics(result)

	return result, nil
}

func (r *Runner) sendRequest(ctx context.Context, body []byte) error {
	url := r.config.Target + "/v1/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer benchmark-test-key")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body to completion
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (r *Runner) calculateMetrics(result *Result) {
	if len(result.Latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	sort.Slice(result.Latencies, func(i, j int) bool {
		return result.Latencies[i] < result.Latencies[j]
	})

	// Calculate basic stats
	result.LatencyMin = result.Latencies[0]
	result.LatencyMax = result.Latencies[len(result.Latencies)-1]

	var total time.Duration
	for _, lat := range result.Latencies {
		total += lat
	}
	result.LatencyMean = total / time.Duration(len(result.Latencies))

	// Calculate percentiles
	result.LatencyP50 = percentile(result.Latencies, 50)
	result.LatencyP95 = percentile(result.Latencies, 95)
	result.LatencyP99 = percentile(result.Latencies, 99)

	// Calculate RPS
	if result.Duration > 0 {
		result.RPS = float64(result.SuccessRequests) / result.Duration.Seconds()
	}
}

func percentile(latencies []time.Duration, p int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	idx := (len(latencies) * p) / 100
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}
	return latencies[idx]
}

// PrintResult prints the result in a human-readable format.
func (r *Runner) PrintResult(result *Result) {
	fmt.Println("\n========================================")
	fmt.Printf("Benchmark Results: %s\n", result.Name)
	fmt.Println("========================================")
	fmt.Printf("Target:       %s\n", result.Target)
	fmt.Printf("Duration:     %v\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("Concurrency:  %d\n", result.Concurrency)
	fmt.Println()
	fmt.Println("Requests:")
	fmt.Printf("  Total:      %d\n", result.TotalRequests)
	fmt.Printf("  Success:    %d\n", result.SuccessRequests)
	fmt.Printf("  Failed:     %d\n", result.FailedRequests)
	fmt.Printf("  RPS:        %.2f\n", result.RPS)
	fmt.Println()
	fmt.Println("Latency:")
	fmt.Printf("  Min:        %v\n", result.LatencyMin.Round(time.Microsecond))
	fmt.Printf("  Max:        %v\n", result.LatencyMax.Round(time.Microsecond))
	fmt.Printf("  Mean:       %v\n", result.LatencyMean.Round(time.Microsecond))
	fmt.Printf("  P50:        %v\n", result.LatencyP50.Round(time.Microsecond))
	fmt.Printf("  P95:        %v\n", result.LatencyP95.Round(time.Microsecond))
	fmt.Printf("  P99:        %v\n", result.LatencyP99.Round(time.Microsecond))
	fmt.Println("========================================")
}

// SaveResult saves the result to a JSON file.
func (r *Runner) SaveResult(result *Result, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

func writeFile(path string, data []byte) error {
	return nil // TODO: implement file writing
}
