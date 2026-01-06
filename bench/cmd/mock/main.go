// Package main provides the mock LLM server entry point.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blueberrycongee/llmux/bench/internal/mock"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	latency := flag.Duration("latency", 50*time.Millisecond, "Simulated API latency")
	flag.Parse()

	server := mock.NewServer()
	server.Latency = *latency

	addr := fmt.Sprintf(":%d", *port)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down mock server...")
		httpServer.Close()
	}()

	log.Printf("Mock LLM Server starting on %s", addr)
	log.Printf("  Latency: %v", *latency)
	log.Printf("  Endpoints:")
	log.Printf("    POST /v1/chat/completions")
	log.Printf("    GET  /v1/models")
	log.Printf("    GET  /health")

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
