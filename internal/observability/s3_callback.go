// Package observability provides an S3 callback for logging LLM requests to AWS S3.
package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config contains configuration for S3 logging.
type S3Config struct {
	BucketName    string        // S3 bucket name
	Region        string        // AWS region
	AccessKeyID   string        // AWS access key (optional, uses default credentials if empty)
	SecretKey     string        // AWS secret key (optional)
	Endpoint      string        // Custom S3 endpoint (for MinIO, etc.)
	PathPrefix    string        // Prefix for S3 keys (e.g., "llmux/logs")
	FlushInterval time.Duration // Flush interval for batching
	BatchSize     int           // Max batch size before flush
	Compression   bool          // Enable gzip compression
}

// DefaultS3Config returns default configuration from environment.
func DefaultS3Config() S3Config {
	return S3Config{
		BucketName:    os.Getenv("S3_BUCKET_NAME"),
		Region:        os.Getenv("AWS_REGION"),
		AccessKeyID:   os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey:     os.Getenv("AWS_SECRET_ACCESS_KEY"),
		Endpoint:      os.Getenv("S3_ENDPOINT"),
		PathPrefix:    os.Getenv("S3_PATH_PREFIX"),
		FlushInterval: 10 * time.Second,
		BatchSize:     100,
		Compression:   true,
	}
}

// S3LogEntry represents a single log entry for S3.
type S3LogEntry struct {
	Timestamp        time.Time              `json:"timestamp"`
	RequestID        string                 `json:"request_id"`
	CallType         string                 `json:"call_type"`
	Status           string                 `json:"status"`
	Model            string                 `json:"model"`
	RequestedModel   string                 `json:"requested_model"`
	APIProvider      string                 `json:"api_provider"`
	APIBase          string                 `json:"api_base"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	TotalTokens      int                    `json:"total_tokens"`
	ResponseCost     float64                `json:"response_cost"`
	LatencyMs        int64                  `json:"latency_ms"`
	TTFTMs           *int64                 `json:"ttft_ms,omitempty"`
	Team             string                 `json:"team,omitempty"`
	User             string                 `json:"user,omitempty"`
	EndUser          string                 `json:"end_user,omitempty"`
	APIKeyAlias      string                 `json:"api_key_alias,omitempty"`
	CacheHit         bool                   `json:"cache_hit"`
	Error            string                 `json:"error,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// S3Callback implements Callback for S3 logging.
type S3Callback struct {
	config   S3Config
	client   *s3.Client
	logQueue []S3LogEntry
	mu       sync.Mutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewS3Callback creates a new S3 callback.
func NewS3Callback(cfg S3Config) (*S3Callback, error) {
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("s3: bucket_name is required")
	}

	// Build AWS config
	var awsCfg aws.Config
	var err error

	opts := []func(*config.LoadOptions) error{}

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err = config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	cb := &S3Callback{
		config:   cfg,
		client:   client,
		logQueue: make([]S3LogEntry, 0, cfg.BatchSize),
		stopCh:   make(chan struct{}),
	}

	// Start background flush goroutine
	cb.wg.Add(1)
	go cb.flushLoop()

	return cb, nil
}

// Name returns the callback name.
func (s *S3Callback) Name() string {
	return "s3"
}

// LogPreAPICall is a no-op for S3 (we log on completion).
func (s *S3Callback) LogPreAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogPostAPICall is a no-op for S3.
func (s *S3Callback) LogPostAPICall(ctx context.Context, payload *StandardLoggingPayload) error {
	return nil
}

// LogStreamEvent is a no-op for S3.
func (s *S3Callback) LogStreamEvent(ctx context.Context, payload *StandardLoggingPayload, chunk any) error {
	return nil
}

// LogSuccessEvent logs a successful request to S3.
func (s *S3Callback) LogSuccessEvent(ctx context.Context, payload *StandardLoggingPayload) error {
	entry := s.payloadToEntry(payload)
	entry.Status = "success"
	s.enqueue(entry)
	return nil
}

// LogFailureEvent logs a failed request to S3.
func (s *S3Callback) LogFailureEvent(ctx context.Context, payload *StandardLoggingPayload, err error) error {
	entry := s.payloadToEntry(payload)
	entry.Status = "failure"
	if err != nil {
		entry.Error = err.Error()
	} else if payload.ErrorStr != nil {
		entry.Error = *payload.ErrorStr
	}
	s.enqueue(entry)
	return nil
}

// LogFallbackEvent logs a fallback event.
func (s *S3Callback) LogFallbackEvent(ctx context.Context, originalModel, fallbackModel string, err error, success bool) error {
	entry := S3LogEntry{
		Timestamp:      time.Now(),
		CallType:       "fallback",
		Model:          fallbackModel,
		RequestedModel: originalModel,
	}
	if success {
		entry.Status = "fallback_success"
	} else {
		entry.Status = "fallback_failed"
	}
	if err != nil {
		entry.Error = err.Error()
	}
	s.enqueue(entry)
	return nil
}

// Shutdown flushes remaining logs and stops the callback.
func (s *S3Callback) Shutdown(ctx context.Context) error {
	close(s.stopCh)
	s.wg.Wait()
	return s.flush(ctx)
}

// payloadToEntry converts StandardLoggingPayload to S3LogEntry.
func (s *S3Callback) payloadToEntry(payload *StandardLoggingPayload) S3LogEntry {
	entry := S3LogEntry{
		Timestamp:        payload.EndTime,
		RequestID:        payload.RequestID,
		CallType:         string(payload.CallType),
		Model:            payload.Model,
		RequestedModel:   payload.RequestedModel,
		APIProvider:      payload.APIProvider,
		APIBase:          payload.APIBase,
		PromptTokens:     payload.PromptTokens,
		CompletionTokens: payload.CompletionTokens,
		TotalTokens:      payload.TotalTokens,
		ResponseCost:     payload.ResponseCost,
		LatencyMs:        payload.EndTime.Sub(payload.StartTime).Milliseconds(),
		Metadata:         payload.Metadata,
	}

	// Optional fields
	if payload.Team != nil {
		entry.Team = *payload.Team
	}
	if payload.User != nil {
		entry.User = *payload.User
	}
	if payload.EndUser != nil {
		entry.EndUser = *payload.EndUser
	}
	if payload.APIKeyAlias != nil {
		entry.APIKeyAlias = *payload.APIKeyAlias
	}
	if payload.CacheHit != nil {
		entry.CacheHit = *payload.CacheHit
	}
	if payload.CompletionStartTime != nil {
		ttft := payload.CompletionStartTime.Sub(payload.StartTime).Milliseconds()
		entry.TTFTMs = &ttft
	}

	return entry
}

// enqueue adds a log entry to the queue.
func (s *S3Callback) enqueue(entry S3LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logQueue = append(s.logQueue, entry)

	if len(s.logQueue) >= s.config.BatchSize {
		go s.flush(context.Background())
	}
}

// flushLoop periodically flushes logs.
func (s *S3Callback) flushLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.flush(context.Background())
		case <-s.stopCh:
			return
		}
	}
}

// flush uploads queued logs to S3.
func (s *S3Callback) flush(ctx context.Context) error {
	s.mu.Lock()
	if len(s.logQueue) == 0 {
		s.mu.Unlock()
		return nil
	}

	entries := s.logQueue
	s.logQueue = make([]S3LogEntry, 0, s.config.BatchSize)
	s.mu.Unlock()

	// Build JSONL content
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for i := range entries {
		if err := encoder.Encode(&entries[i]); err != nil {
			continue
		}
	}

	// Generate S3 key
	now := time.Now().UTC()
	key := s.generateKey(now)

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/x-ndjson"),
	})

	if err != nil {
		return fmt.Errorf("s3: failed to upload logs: %w", err)
	}

	return nil
}

// generateKey generates an S3 key with date partitioning.
func (s *S3Callback) generateKey(t time.Time) string {
	// Format: prefix/year=YYYY/month=MM/day=DD/hour=HH/logs_timestamp.jsonl
	datePrefix := fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d",
		t.Year(), t.Month(), t.Day(), t.Hour())

	filename := fmt.Sprintf("logs_%d.jsonl", t.UnixNano())

	if s.config.PathPrefix != "" {
		return path.Join(s.config.PathPrefix, datePrefix, filename)
	}
	return path.Join(datePrefix, filename)
}
