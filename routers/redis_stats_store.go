package routers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// RedisStatsStore implements StatsStore using Redis for distributed statistics.
// It uses Lua scripts to ensure atomic operations across multiple LLMux instances.
type RedisStatsStore struct {
	client             redis.UniversalClient
	keyPrefix          string
	maxLatencyListSize int
	usageTTL           time.Duration
	failureWindowMins  int
	failureBucketSecs  int
	failureThreshold   float64
	minRequests        int
	cooldownPeriod     time.Duration
	immediateOn429     bool
	singleDeployMinReq int

	// Precompiled Lua scripts
	recordSuccessScript      *redis.Script
	recordFailureScript      *redis.Script
	incrementActiveReqScript *redis.Script
	decrementActiveReqScript *redis.Script
	getStatsScript           *redis.Script
	setCooldownScript        *redis.Script
	deleteStatsScript        *redis.Script
}

// RedisStatsOption configures RedisStatsStore.
type RedisStatsOption func(*RedisStatsStore)

// WithKeyPrefix sets the Redis key prefix (default: "llmux:router:stats").
func WithKeyPrefix(prefix string) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.keyPrefix = prefix
	}
}

// WithMaxLatencySamples sets the maximum number of latency samples to keep (default: 10).
func WithMaxLatencySamples(size int) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.maxLatencyListSize = size
	}
}

// WithUsageTTL sets the TTL for per-minute usage stats (default: 120s).
func WithUsageTTL(ttl time.Duration) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.usageTTL = ttl
	}
}

// WithFailureWindowMinutes sets the sliding window size in minutes (default: 5).
func WithFailureWindowMinutes(minutes int) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.failureWindowMins = minutes
	}
}

// WithFailureBucketSeconds sets the bucket size in seconds (default: 60).
func WithFailureBucketSeconds(seconds int) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.failureBucketSecs = seconds
	}
}

// WithFailureThresholdPercent sets the failure rate threshold for cooldown.
func WithFailureThresholdPercent(threshold float64) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.failureThreshold = threshold
	}
}

// WithMinRequestsForThreshold sets the minimum requests before rate-based cooldown.
func WithMinRequestsForThreshold(minRequests int) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.minRequests = minRequests
	}
}

// WithCooldownPeriod sets the cooldown period for distributed stats.
func WithCooldownPeriod(period time.Duration) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.cooldownPeriod = period
	}
}

// WithImmediateCooldownOn429 toggles immediate cooldown for 429 responses.
func WithImmediateCooldownOn429(enabled bool) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.immediateOn429 = enabled
	}
}

// WithSingleDeploymentFailureThreshold sets the minimum requests before cooldown on single-deployment groups.
func WithSingleDeploymentFailureThreshold(minRequests int) RedisStatsOption {
	return func(r *RedisStatsStore) {
		r.singleDeployMinReq = minRequests
	}
}

// NewRedisStatsStore creates a new Redis-backed stats store.
func NewRedisStatsStore(client redis.UniversalClient, opts ...RedisStatsOption) *RedisStatsStore {
	defaults := router.DefaultConfig()
	store := &RedisStatsStore{
		client:             client,
		keyPrefix:          "llmux:router:stats",
		maxLatencyListSize: 10,
		usageTTL:           120 * time.Second,
		failureWindowMins:  defaultFailureWindowMinutes,
		failureBucketSecs:  defaultFailureBucketSeconds,
		failureThreshold:   defaults.FailureThresholdPercent,
		minRequests:        defaults.MinRequestsForThreshold,
		cooldownPeriod:     defaults.CooldownPeriod,
		immediateOn429:     defaults.ImmediateCooldownOn429,
		singleDeployMinReq: defaultSingleDeploymentFailureMinReq,
	}

	// Apply options
	for _, opt := range opts {
		opt(store)
	}

	// Precompile Lua scripts
	store.recordSuccessScript = redis.NewScript(recordSuccessScript)
	store.recordFailureScript = redis.NewScript(recordFailureScript)
	store.incrementActiveReqScript = redis.NewScript(incrementActiveRequestsScript)
	store.decrementActiveReqScript = redis.NewScript(decrementActiveRequestsScript)
	store.getStatsScript = redis.NewScript(getStatsScript)
	store.setCooldownScript = redis.NewScript(setCooldownScript)
	store.deleteStatsScript = redis.NewScript(deleteStatsScript)

	return store
}

// GetStats retrieves statistics for a deployment.
func (r *RedisStatsStore) GetStats(ctx context.Context, deploymentID string) (*DeploymentStats, error) {
	keys := []string{
		r.latencyKey(deploymentID),
		r.ttftKey(deploymentID),
		r.countersKey(deploymentID),
		r.usageKeyPrefix(deploymentID),
	}

	result, err := r.getStatsScript.Run(ctx, r.client, keys).Result()
	if err != nil {
		return nil, err
	}

	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 5 {
		return nil, fmt.Errorf("unexpected result format from getStatsScript")
	}

	currentMinute := parseString(resultSlice[4])
	stats := &DeploymentStats{
		MaxLatencyListSize: r.maxLatencyListSize,
		CurrentMinuteKey:   currentMinute,
	}

	// Parse latency history (result[0])
	if latencyList, ok := resultSlice[0].([]interface{}); ok {
		stats.LatencyHistory = make([]float64, 0, len(latencyList))
		for _, v := range latencyList {
			if f, err := parseFloat(v); err == nil {
				stats.LatencyHistory = append(stats.LatencyHistory, f)
			}
		}
	}

	// Parse TTFT history (result[1])
	if ttftList, ok := resultSlice[1].([]interface{}); ok {
		stats.TTFTHistory = make([]float64, 0, len(ttftList))
		for _, v := range ttftList {
			if f, err := parseFloat(v); err == nil {
				stats.TTFTHistory = append(stats.TTFTHistory, f)
			}
		}
	}

	// Track whether the counters hash exists in Redis (has any fields)
	// This is different from checking if the field VALUES are non-zero.
	// A hash with [active_requests 0] still EXISTS, it just has zero value.
	countersHashExists := false

	// Parse counters hash (result[2])
	// HGETALL returns an array of key-value pairs, or empty array if hash doesn't exist
	if resultSlice[2] != nil {
		if countersSlice, ok := resultSlice[2].([]interface{}); ok {
			// If HGETALL returned any fields, the hash exists
			countersHashExists = len(countersSlice) > 0

			if countersHashExists {
				countersMap := parseHashMap(countersSlice)

				stats.TotalRequests = parseInt64(countersMap["total_requests"])
				stats.SuccessCount = parseInt64(countersMap["success_count"])
				stats.FailureCount = parseInt64(countersMap["failure_count"])
				stats.ActiveRequests = parseInt64(countersMap["active_requests"])

				if lastReqTime := parseInt64(countersMap["last_request_time"]); lastReqTime > 0 {
					stats.LastRequestTime = time.Unix(lastReqTime, 0)
				}
			}
		}
	}

	// Parse usage hash (result[3])
	usageHashExists := false
	if usageSlice, ok := resultSlice[3].([]interface{}); ok && len(usageSlice) > 0 {
		usageHashExists = true
		usageMap := parseHashMap(usageSlice)
		stats.CurrentMinuteTPM = parseInt64(usageMap["tpm"])
		stats.CurrentMinuteRPM = parseInt64(usageMap["rpm"])
	}

	// Calculate average latency
	if len(stats.LatencyHistory) > 0 {
		var sum float64
		for _, lat := range stats.LatencyHistory {
			sum += lat
		}
		stats.AvgLatencyMs = sum / float64(len(stats.LatencyHistory))
	}

	// Calculate average TTFT
	if len(stats.TTFTHistory) > 0 {
		var sum float64
		for _, ttft := range stats.TTFTHistory {
			sum += ttft
		}
		stats.AvgTTFTMs = sum / float64(len(stats.TTFTHistory))
	}

	// Get cooldown status
	cooldownUntil, _ := r.GetCooldownUntil(ctx, deploymentID)
	stats.CooldownUntil = cooldownUntil

	// Determine if the deployment exists in Redis.
	// A deployment exists if ANY of the following is true:
	// - The counters hash has any fields (even if all values are 0)
	// - The latency/TTFT lists have any entries
	// - The usage hash has any fields
	// - A cooldown is set
	//
	// This is the industry-standard approach: check if the Redis key/hash EXISTS,
	// not whether the values are non-zero. A deployment with [active_requests=0]
	// still exists, it just has no active requests.
	hasLatencyData := len(stats.LatencyHistory) > 0
	hasTTFTData := len(stats.TTFTHistory) > 0
	hasCooldown := !stats.CooldownUntil.IsZero()

	deploymentExists := countersHashExists || hasLatencyData || hasTTFTData || usageHashExists || hasCooldown

	if !deploymentExists {
		return nil, ErrStatsNotFound
	}

	return stats, nil
}

// IncrementActiveRequests atomically increments the active request count.
func (r *RedisStatsStore) IncrementActiveRequests(ctx context.Context, deploymentID string) error {
	keys := []string{r.countersKey(deploymentID)}
	_, err := r.incrementActiveReqScript.Run(ctx, r.client, keys).Result()
	return err
}

// DecrementActiveRequests atomically decrements the active request count.
func (r *RedisStatsStore) DecrementActiveRequests(ctx context.Context, deploymentID string) error {
	keys := []string{r.countersKey(deploymentID)}
	_, err := r.decrementActiveReqScript.Run(ctx, r.client, keys).Result()
	return err
}

// RecordSuccess records a successful request with its metrics.
func (r *RedisStatsStore) RecordSuccess(ctx context.Context, deploymentID string, metrics *ResponseMetrics) error {
	keys := []string{
		r.latencyKey(deploymentID),
		r.ttftKey(deploymentID),
		r.countersKey(deploymentID),
		r.usageKeyPrefix(deploymentID),
		r.successKeyPrefix(deploymentID),
	}

	latencyMs := float64(metrics.Latency.Milliseconds())
	ttftMs := float64(0)
	if metrics.TimeToFirstToken > 0 {
		ttftMs = float64(metrics.TimeToFirstToken.Milliseconds())
	}

	args := []interface{}{
		latencyMs,
		ttftMs,
		metrics.TotalTokens,
		r.maxLatencyListSize,
		int(r.usageTTL.Seconds()),
		r.bucketTTLSeconds(),
		r.bucketSeconds(),
	}

	_, err := r.recordSuccessScript.Run(ctx, r.client, keys, args...).Result()
	return err
}

// RecordFailure records a failed request.
func (r *RedisStatsStore) RecordFailure(ctx context.Context, deploymentID string, err error) error {
	return r.RecordFailureWithOptions(ctx, deploymentID, err, failureRecordOptions{})
}

// RecordFailureWithOptions records a failed request with routing context.
func (r *RedisStatsStore) RecordFailureWithOptions(ctx context.Context, deploymentID string, err error, opts failureRecordOptions) error {
	windowSize := r.failureWindowSize()
	keys := []string{
		r.countersKey(deploymentID),
		r.latencyKey(deploymentID),
		r.cooldownKey(deploymentID),
		r.successKeyPrefix(deploymentID),
		r.failureKeyPrefix(deploymentID),
	}

	isTimeout := 0
	statusCode := 0
	if llmErr, ok := err.(*llmerrors.LLMError); ok {
		statusCode = llmErr.StatusCode
		// Timeout errors: 408 (Request Timeout) or 504 (Gateway Timeout)
		if llmErr.StatusCode == 408 || llmErr.StatusCode == 504 {
			isTimeout = 1
		}
	}

	args := []interface{}{
		isTimeout,
		r.maxLatencyListSize,
		windowSize,
		r.bucketTTLSeconds(),
		r.failureThreshold,
		r.minRequests,
		int(r.cooldownPeriod.Seconds()),
		boolToInt(r.immediateOn429),
		statusCode,
		r.singleDeployMinReq,
		boolToInt(opts.isSingleDeployment),
		r.cooldownTTLSeconds(),
		r.bucketSeconds(),
	}

	_, runErr := r.recordFailureScript.Run(ctx, r.client, keys, args...).Result()
	return runErr
}

// SetCooldown manually sets a cooldown period for a deployment.
func (r *RedisStatsStore) SetCooldown(ctx context.Context, deploymentID string, until time.Time) error {
	keys := []string{r.cooldownKey(deploymentID)}

	ttl := time.Until(until)
	if ttl <= 0 {
		// Already expired, delete the key
		return r.client.Del(ctx, keys[0]).Err()
	}

	args := []interface{}{
		until.Unix(),
		int(ttl.Seconds()) + 10, // Add 10s buffer to TTL
	}

	_, err := r.setCooldownScript.Run(ctx, r.client, keys, args...).Result()
	return err
}

// GetCooldownUntil returns the cooldown expiration time for a deployment.
func (r *RedisStatsStore) GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error) {
	key := r.cooldownKey(deploymentID)
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}

	timestamp, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(timestamp, 0), nil
}

// ListDeployments returns all deployment IDs that have stats recorded.
func (r *RedisStatsStore) ListDeployments(ctx context.Context) ([]string, error) {
	pattern := r.keyPrefix + ":*:counters"
	var deploymentIDs []string

	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Extract deployment ID from key
		// Format: "llmux:router:stats:{deploymentID}:counters"
		deploymentID := r.extractDeploymentID(key)
		if deploymentID != "" {
			deploymentIDs = append(deploymentIDs, deploymentID)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return deploymentIDs, nil
}

// DeleteStats removes all stats for a deployment.
func (r *RedisStatsStore) DeleteStats(ctx context.Context, deploymentID string) error {
	keys := []string{
		r.latencyKey(deploymentID),
		r.ttftKey(deploymentID),
		r.countersKey(deploymentID),
		r.cooldownKey(deploymentID),
		r.usageKeyPrefix(deploymentID) + "*", // Pattern for SCAN
		r.successKeyPrefix(deploymentID) + "*",
		r.failureKeyPrefix(deploymentID) + "*",
	}

	_, err := r.deleteStatsScript.Run(ctx, r.client, keys).Result()
	return err
}

// Close releases any resources held by the store.
func (r *RedisStatsStore) Close() error {
	// Redis client is shared, don't close it here
	return nil
}

// Key generation helpers

func (r *RedisStatsStore) latencyKey(deploymentID string) string {
	return fmt.Sprintf("%s:latency", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) ttftKey(deploymentID string) string {
	return fmt.Sprintf("%s:ttft", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) countersKey(deploymentID string) string {
	return fmt.Sprintf("%s:counters", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) cooldownKey(deploymentID string) string {
	return fmt.Sprintf("%s:cooldown", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) successKeyPrefix(deploymentID string) string {
	return fmt.Sprintf("%s:successes:", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) failureKeyPrefix(deploymentID string) string {
	return fmt.Sprintf("%s:failures:", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) successKey(deploymentID, minute string) string {
	return fmt.Sprintf("%s:successes:%s", r.deploymentKeyPrefix(deploymentID), minute)
}

func (r *RedisStatsStore) failureKey(deploymentID, minute string) string {
	return fmt.Sprintf("%s:failures:%s", r.deploymentKeyPrefix(deploymentID), minute)
}

func (r *RedisStatsStore) usageKeyPrefix(deploymentID string) string {
	return fmt.Sprintf("%s:usage:", r.deploymentKeyPrefix(deploymentID))
}

func (r *RedisStatsStore) usageKey(deploymentID, minute string) string {
	return fmt.Sprintf("%s:usage:%s", r.deploymentKeyPrefix(deploymentID), minute)
}

func (r *RedisStatsStore) extractDeploymentID(key string) string {
	// Extract from "prefix:{deploymentID}:counters"
	prefix := r.keyPrefix + ":"
	suffix := ":counters"

	if len(key) <= len(prefix)+len(suffix) {
		return ""
	}

	start := len(prefix)
	end := len(key) - len(suffix)

	if end <= start {
		return ""
	}

	deploymentID := key[start:end]
	if strings.HasPrefix(deploymentID, "{") && strings.HasSuffix(deploymentID, "}") {
		deploymentID = deploymentID[1 : len(deploymentID)-1]
	}
	return deploymentID
}

func (r *RedisStatsStore) deploymentKeyPrefix(deploymentID string) string {
	return fmt.Sprintf("%s:{%s}", r.keyPrefix, deploymentID)
}

func (r *RedisStatsStore) failureWindowSize() int {
	if r.failureWindowMins <= 0 {
		return defaultFailureWindowMinutes
	}
	return r.failureWindowMins
}

func (r *RedisStatsStore) bucketSeconds() int {
	if r.failureBucketSecs <= 0 {
		return defaultFailureBucketSeconds
	}
	return r.failureBucketSecs
}

func (r *RedisStatsStore) bucketTTLSeconds() int {
	windowSeconds := r.failureWindowSize() * r.bucketSeconds()
	return windowSeconds + r.bucketSeconds()
}

func (r *RedisStatsStore) cooldownTTLSeconds() int {
	cooldownSeconds := int(r.cooldownPeriod.Seconds())
	if cooldownSeconds <= 0 {
		return 0
	}
	return cooldownSeconds + 10
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// Parsing helpers

func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot parse float from %T", v)
	}
}

func parseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func parseString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatInt(int64(val), 10)
	default:
		return ""
	}
}

func parseHashMap(slice []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i < len(slice); i += 2 {
		if i+1 < len(slice) {
			key, ok1 := slice[i].(string)
			val := slice[i+1]
			if ok1 {
				m[key] = val
			}
		}
	}
	return m
}
