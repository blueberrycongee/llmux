package resilience

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements DistributedLimiter using Redis and Lua scripts.
type RedisLimiter struct {
	client *redis.Client
	script *redis.Script
}

// NewRedisLimiter creates a new RedisLimiter instance.
func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	// BATCH_RATE_LIMITER_SCRIPT (Ported from litellm)
	luaScript := `
local results = {}
local now = tonumber(ARGV[1])        -- Current timestamp
local window_size = tonumber(ARGV[2]) -- Window size in seconds (default 60)

-- Iterate over keys (pairs of window_key, counter_key)
for i = 1, #KEYS, 2 do
    local window_key = KEYS[i]
    local counter_key = KEYS[i + 1]
    local increment_value = 1

    -- 1. Check window start time
    local window_start = redis.call('GET', window_key)
    
    -- 2. If window expired or doesn't exist, reset
    if not window_start or (now - tonumber(window_start)) >= window_size then
        redis.call('SET', window_key, tostring(now))
        redis.call('SET', counter_key, increment_value)
        redis.call('EXPIRE', window_key, window_size)
        redis.call('EXPIRE', counter_key, window_size)
        table.insert(results, tostring(now)) -- New window start
        table.insert(results, increment_value) -- New count (1)
    else
        -- 3. Window active: Increment counter
        local counter = redis.call('INCR', counter_key)
        -- Ensure TTL exists (defensive programming)
        if redis.call('TTL', counter_key) == -1 then
            redis.call('EXPIRE', counter_key, window_size)
        end
        table.insert(results, window_start) -- Existing window start
        table.insert(results, counter) -- Current count
    end
end

return results
`
	return &RedisLimiter{
		client: client,
		script: redis.NewScript(luaScript),
	}
}

// CheckAllow atomically checks and increments limits for multiple descriptors.
func (r *RedisLimiter) CheckAllow(ctx context.Context, descriptors []Descriptor) ([]LimitResult, error) {
	if len(descriptors) == 0 {
		return nil, nil
	}

	// Prepare keys and args
	// We assume all descriptors share the same window size for this batch call.
	now := time.Now().Unix()
	windowSize := int64(60) // Default
	if len(descriptors) > 0 {
		windowSize = int64(descriptors[0].Window.Seconds())
	}

	keys := make([]string, 0, len(descriptors)*2)
	for _, desc := range descriptors {
		// Format: {{api_key}:{model}}:window
		// Logic: By enclosing the "identity" part in curly braces {}, Redis guarantees that the window key and the counter key are stored on the same node.
		tag := fmt.Sprintf("{%s:%s}", desc.Key, desc.Value)

		// Append type to the key to avoid collision between RPM and TPM
		baseKey := fmt.Sprintf("%s:%s", tag, desc.Type)
		windowKey := baseKey + ":window"
		counterKey := baseKey + ":count"

		keys = append(keys, windowKey, counterKey)
	}

	args := []interface{}{now, windowSize}

	// Run script
	val, err := r.script.Run(ctx, r.client, keys, args...).Result()
	if err != nil {
		return nil, err
	}

	resultsSlice, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from redis script: %T", val)
	}

	if len(resultsSlice) != len(descriptors)*2 {
		return nil, fmt.Errorf("unexpected result length: got %d, want %d", len(resultsSlice), len(descriptors)*2)
	}

	limitResults := make([]LimitResult, len(descriptors))
	for i := 0; i < len(descriptors); i++ {
		countVal := resultsSlice[i*2+1]

		var current int64
		switch v := countVal.(type) {
		case int64:
			current = v
		case string:
			current, _ = strconv.ParseInt(v, 10, 64)
		case float64:
			current = int64(v)
		default:
			current, _ = strconv.ParseInt(fmt.Sprintf("%v", v), 10, 64)
		}

		desc := descriptors[i]
		allowed := current <= desc.Limit
		remaining := desc.Limit - current
		if remaining < 0 {
			remaining = 0
		}

		// Calculate ResetAt
		windowStartVal := resultsSlice[i*2]
		var windowStart int64
		switch v := windowStartVal.(type) {
		case string:
			windowStart, _ = strconv.ParseInt(v, 10, 64)
		case float64:
			windowStart = int64(v)
		case int64:
			windowStart = v
		}

		resetAt := windowStart + windowSize

		limitResults[i] = LimitResult{
			Allowed:   allowed,
			Current:   current,
			Remaining: remaining,
			ResetAt:   resetAt,
			Error:     nil,
		}
	}

	return limitResults, nil
}
