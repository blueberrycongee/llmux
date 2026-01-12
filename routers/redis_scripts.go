package routers

// This file contains Lua scripts for atomic Redis operations on router statistics.
// The scripts ensure consistency in distributed deployments where multiple LLMux
// instances share the same Redis backend.
//
// Design inspired by the atomic pattern used in internal/resilience/redis_limiter.go
// for distributed rate limiting.

const (
	// recordSuccessScript atomically records a successful request with all metrics.
	//
	// Keys:
	//   KEYS[1] - latency list key (e.g., "llmux:router:stats:deployment-1:latency")
	//   KEYS[2] - ttft list key (e.g., "llmux:router:stats:deployment-1:ttft")
	//   KEYS[3] - counters hash key (e.g., "llmux:router:stats:deployment-1:counters")
	//   KEYS[4] - usage hash key prefix (e.g., "llmux:router:stats:deployment-1:usage:")
	//   KEYS[5] - success bucket key prefix (e.g., "llmux:router:stats:deployment-1:successes:")
	//
	// Args:
	//   ARGV[1] - latency value in milliseconds (float)
	//   ARGV[2] - ttft value in milliseconds (float, 0 if not streaming)
	//   ARGV[3] - total tokens (integer)
	//   ARGV[4] - max latency list size (integer, default 10)
	//   ARGV[5] - usage TTL in seconds (integer, default 120)
	//   ARGV[6] - bucket TTL in seconds (integer)
	//   ARGV[7] - bucket size in seconds (integer)
	//
	// Returns:
	//   "OK" on success
	recordSuccessScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local usage_prefix = KEYS[4]
local success_prefix = KEYS[5]

local latency = tonumber(ARGV[1])
local ttft = tonumber(ARGV[2])
local tokens = tonumber(ARGV[3])
local max_size = tonumber(ARGV[4])
local usage_ttl = tonumber(ARGV[5])
local bucket_ttl = tonumber(ARGV[6])
local bucket_seconds = tonumber(ARGV[7])

local time_data = redis.call('TIME')
local now = tonumber(time_data[1])

local minute_bucket = math.floor(now / 60)
local usage_key = usage_prefix .. minute_bucket
local bucket = math.floor(now / bucket_seconds)
local success_key = success_prefix .. bucket

-- 1. Update latency history (rolling window)
redis.call('LPUSH', latency_key, latency)
redis.call('LTRIM', latency_key, 0, max_size - 1)
redis.call('EXPIRE', latency_key, 3600)  -- 1 hour TTL

-- 2. Update TTFT history if provided (for streaming requests)
if ttft and ttft > 0 then
    redis.call('LPUSH', ttft_key, ttft)
    redis.call('LTRIM', ttft_key, 0, max_size - 1)
    redis.call('EXPIRE', ttft_key, 3600)
end

-- 3. Update counters atomically
redis.call('HINCRBY', counters_key, 'total_requests', 1)
redis.call('HINCRBY', counters_key, 'success_count', 1)
redis.call('HSET', counters_key, 'last_request_time', now)
redis.call('EXPIRE', counters_key, 3600)

-- 4. Update TPM/RPM for current minute
redis.call('HINCRBY', usage_key, 'tpm', tokens)
redis.call('HINCRBY', usage_key, 'rpm', 1)
redis.call('EXPIRE', usage_key, usage_ttl)

-- 5. Track per-minute successes (sliding window)
redis.call('INCRBY', success_key, 1)
redis.call('EXPIRE', success_key, bucket_ttl)

return redis.status_reply("OK")
`

	// recordFailureScript atomically records a failed request.
	//
	// Keys:
	//   KEYS[1] - counters hash key
	//   KEYS[2] - latency list key (for penalty latency on timeout)
	//   KEYS[3] - cooldown key
	//   KEYS[4] - success bucket key prefix (e.g., "llmux:router:stats:deployment-1:successes:")
	//   KEYS[5] - failure bucket key prefix (e.g., "llmux:router:stats:deployment-1:failures:")
	//
	// Args:
	//   ARGV[1] - is timeout error (1 or 0)
	//   ARGV[2] - max latency list size
	//   ARGV[3] - window size (N)
	//   ARGV[4] - bucket TTL in seconds
	//   ARGV[5] - failure threshold percent (float)
	//   ARGV[6] - min requests for threshold
	//   ARGV[7] - cooldown seconds
	//   ARGV[8] - immediate cooldown on 429 (1 or 0)
	//   ARGV[9] - status code (int)
	//   ARGV[10] - single deployment min requests
	//   ARGV[11] - is single deployment (1 or 0)
	//   ARGV[12] - cooldown TTL seconds
	//   ARGV[13] - bucket size in seconds
	//
	// Returns:
	//   "OK" on success
	recordFailureScript = `
local counters_key = KEYS[1]
local latency_key = KEYS[2]
local cooldown_key = KEYS[3]
local success_prefix = KEYS[4]
local failure_prefix = KEYS[5]

local is_timeout = tonumber(ARGV[1])
local max_size = tonumber(ARGV[2])
local window_size = tonumber(ARGV[3])
local bucket_ttl = tonumber(ARGV[4])
local failure_threshold = tonumber(ARGV[5])
local min_requests = tonumber(ARGV[6])
local cooldown_seconds = tonumber(ARGV[7])
local immediate_on_429 = tonumber(ARGV[8])
local status_code = tonumber(ARGV[9])
local single_deploy_min_requests = tonumber(ARGV[10])
local is_single_deployment = tonumber(ARGV[11])
local cooldown_ttl = tonumber(ARGV[12])
local bucket_seconds = tonumber(ARGV[13])

local time_data = redis.call('TIME')
local now = tonumber(time_data[1])

-- 1. Update failure counters
redis.call('HINCRBY', counters_key, 'total_requests', 1)
redis.call('HINCRBY', counters_key, 'failure_count', 1)
redis.call('HSET', counters_key, 'last_request_time', now)
redis.call('EXPIRE', counters_key, 3600)

-- 2. Add penalty latency for timeout errors (helps lowest-latency routing)
if is_timeout == 1 then
    redis.call('LPUSH', latency_key, 1000000)  -- 1000s penalty
    redis.call('LTRIM', latency_key, 0, max_size - 1)
    redis.call('EXPIRE', latency_key, 3600)
end

-- 3. Track per-minute failures (sliding window)
local current_bucket = math.floor(now / bucket_seconds)
local current_failure_key = failure_prefix .. current_bucket
redis.call('INCRBY', current_failure_key, 1)
redis.call('EXPIRE', current_failure_key, bucket_ttl)

-- 4. Aggregate window totals
local total_success = 0
local total_failure = 0
for i = 0, window_size - 1 do
    local bucket = current_bucket - i
    local success_val = redis.call('GET', success_prefix .. bucket)
    if success_val then
        total_success = total_success + tonumber(success_val)
    end
    local failure_val = redis.call('GET', failure_prefix .. bucket)
    if failure_val then
        total_failure = total_failure + tonumber(failure_val)
    end
end

local total_requests = total_success + total_failure
local should_cooldown = false

if status_code == 401 or status_code == 404 or status_code == 408 then
    should_cooldown = true
end

if status_code == 429 and immediate_on_429 == 1 and is_single_deployment == 0 then
    should_cooldown = true
end

if not should_cooldown then
    if is_single_deployment == 1 then
        if total_requests >= single_deploy_min_requests and total_failure == total_requests and total_requests > 0 then
            should_cooldown = true
        end
    else
        if total_requests >= min_requests and total_requests > 0 then
            local failure_rate = total_failure / total_requests
            if failure_rate > failure_threshold then
                should_cooldown = true
            end
        end
    end
end

if should_cooldown and cooldown_seconds and cooldown_seconds > 0 then
    redis.call('SET', cooldown_key, now + cooldown_seconds)
    redis.call('EXPIRE', cooldown_key, cooldown_ttl)
end

return redis.status_reply("OK")
`

	// incrementActiveRequestsScript atomically increments active request count.
	//
	// Keys:
	//   KEYS[1] - counters hash key
	//
	// Returns:
	//   The new active_requests count
	incrementActiveRequestsScript = `
local counters_key = KEYS[1]
local result = redis.call('HINCRBY', counters_key, 'active_requests', 1)
redis.call('EXPIRE', counters_key, 3600)
return result
`

	// decrementActiveRequestsScript atomically decrements active request count.
	//
	// Keys:
	//   KEYS[1] - counters hash key
	//
	// Returns:
	//   The new active_requests count (minimum 0)
	decrementActiveRequestsScript = `
local counters_key = KEYS[1]
local current = redis.call('HGET', counters_key, 'active_requests')

if current and tonumber(current) > 0 then
    local result = redis.call('HINCRBY', counters_key, 'active_requests', -1)
    redis.call('EXPIRE', counters_key, 3600)
    return result
end

return 0
`

	// getStatsScript retrieves all stats in a single round-trip.
	//
	// Keys:
	//   KEYS[1] - latency list key
	//   KEYS[2] - ttft list key
	//   KEYS[3] - counters hash key
	//   KEYS[4] - usage key prefix (without minute suffix)
	//
	// Returns:
	//   Array of 4 elements:
	//     [1] - latency list (array of floats)
	//     [2] - ttft list (array of floats)
	//     [3] - counters hash (flat array: key1, val1, key2, val2, ...)
	//     [4] - usage hash for current minute (flat array)
	//     [5] - current minute key (UTC unix minute bucket)
	getStatsScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local usage_prefix = KEYS[4]

local time_data = redis.call('TIME')
local now = tonumber(time_data[1])
local current_minute = math.floor(now / 60)
local usage_key = usage_prefix .. current_minute

local result = {}
result[1] = redis.call('LRANGE', latency_key, 0, -1)
result[2] = redis.call('LRANGE', ttft_key, 0, -1)
result[3] = redis.call('HGETALL', counters_key)
result[4] = redis.call('HGETALL', usage_key)
result[5] = tostring(current_minute)

return result
`

	// setCooldownScript sets a cooldown expiration time.
	//
	// Keys:
	//   KEYS[1] - cooldown key
	//
	// Args:
	//   ARGV[1] - cooldown until timestamp (unix seconds)
	//   ARGV[2] - TTL in seconds
	//
	// Returns:
	//   "OK"
	setCooldownScript = `
local cooldown_key = KEYS[1]
local until_timestamp = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

redis.call('SET', cooldown_key, until_timestamp)
redis.call('EXPIRE', cooldown_key, ttl)

return redis.status_reply("OK")
`

	// deleteStatsScript removes all stats for a deployment.
	//
	// Keys:
	//   KEYS[1] - latency key
	//   KEYS[2] - ttft key
	//   KEYS[3] - counters key
	//   KEYS[4] - cooldown key
	//   KEYS[5] - usage key pattern (for SCAN)
	//   KEYS[6] - success bucket key pattern (for SCAN)
	//   KEYS[7] - failure bucket key pattern (for SCAN)
	//
	// Returns:
	//   Number of keys deleted
	deleteStatsScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local cooldown_key = KEYS[4]
local usage_pattern = KEYS[5]
local success_pattern = KEYS[6]
local failure_pattern = KEYS[7]

local deleted = 0

-- Delete fixed keys
deleted = deleted + redis.call('DEL', latency_key, ttft_key, counters_key, cooldown_key)

local function delete_by_pattern(pattern)
    local cursor = "0"
    repeat
        local result = redis.call('SCAN', cursor, 'MATCH', pattern, 'COUNT', 100)
        cursor = result[1]
        local keys = result[2]

        if #keys > 0 then
            deleted = deleted + redis.call('DEL', unpack(keys))
        end
    until cursor == "0"
end

-- Delete usage keys (pattern match)
delete_by_pattern(usage_pattern)
delete_by_pattern(success_pattern)
delete_by_pattern(failure_pattern)

return deleted
`
)
