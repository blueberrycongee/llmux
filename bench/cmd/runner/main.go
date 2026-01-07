// Package main provides the benchmark runner entry point.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/bench/internal/runner"
)

func main() {
	os.Exit(run())
}

func run() int {
	target := flag.String("target", "http://localhost:3000", "Target server URL")
	requests := flag.Int("requests", 1000, "Total number of requests")
	concurrency := flag.Int("concurrency", 100, "Number of concurrent workers")
	name := flag.String("name", "benchmark", "Benchmark name")
	output := flag.String("output", "bench/results", "Output directory for results")
	flag.Parse()

	cfg := runner.Config{
		Target:      *target,
		Requests:    *requests,
		Concurrency: *concurrency,
		Name:        *name,
	}

	log.Printf("Starting benchmark: %s", *name)
	log.Printf("  Target:      %s", *target)
	log.Printf("  Requests:    %d", *requests)
	log.Printf("  Concurrency: %d", *concurrency)

	r := runner.NewRunner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, runErr := r.Run(ctx)
	if runErr != nil {
		log.Printf("Benchmark failed: %v", runErr)
		return 1
	}

	// Print results
	r.PrintResult(result)

	// Save results
	if mkdirErr := os.MkdirAll(*output, 0o755); mkdirErr != nil {
		log.Printf("Warning: failed to create output directory: %v", mkdirErr)
	}

	filename := fmt.Sprintf("%s_%s.json", *name, time.Now().Format("20060102_150405"))
	resultPath := filepath.Join(*output, filename)

	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		log.Printf("Warning: failed to marshal results: %v", marshalErr)
		return 0
	}
	if writeErr := os.WriteFile(resultPath, data, 0o644); writeErr != nil {
		log.Printf("Warning: failed to save results: %v", writeErr)
	} else {
		log.Printf("Results saved to: %s", resultPath)
	}

	return 0
}
