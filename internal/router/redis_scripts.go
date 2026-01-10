package router

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
	//   KEYS[4] - usage hash key (e.g., "llmux:router:stats:deployment-1:usage:2026-01-10-16-52")
	//
	// Args:
	//   ARGV[1] - latency value in milliseconds (float)
	//   ARGV[2] - ttft value in milliseconds (float, 0 if not streaming)
	//   ARGV[3] - total tokens (integer)
	//   ARGV[4] - max latency list size (integer, default 10)
	//   ARGV[5] - usage TTL in seconds (integer, default 120)
	//   ARGV[6] - current timestamp (for last_request_time)
	//
	// Returns:
	//   "OK" on success
	recordSuccessScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local usage_key = KEYS[4]

local latency = tonumber(ARGV[1])
local ttft = tonumber(ARGV[2])
local tokens = tonumber(ARGV[3])
local max_size = tonumber(ARGV[4])
local usage_ttl = tonumber(ARGV[5])
local now = tonumber(ARGV[6])

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

return redis.status_reply("OK")
`

	// recordFailureScript atomically records a failed request.
	//
	// Keys:
	//   KEYS[1] - counters hash key
	//   KEYS[2] - latency list key (for penalty latency on timeout)
	//
	// Args:
	//   ARGV[1] - current timestamp
	//   ARGV[2] - is timeout error (1 or 0)
	//   ARGV[3] - max latency list size
	//
	// Returns:
	//   "OK" on success
	recordFailureScript = `
local counters_key = KEYS[1]
local latency_key = KEYS[2]

local now = tonumber(ARGV[1])
local is_timeout = tonumber(ARGV[2])
local max_size = tonumber(ARGV[3])

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
	// Args:
	//   ARGV[1] - current minute key (e.g., "2026-01-10-16-52")
	//
	// Returns:
	//   Array of 4 elements:
	//     [1] - latency list (array of floats)
	//     [2] - ttft list (array of floats)
	//     [3] - counters hash (flat array: key1, val1, key2, val2, ...)
	//     [4] - usage hash for current minute (flat array)
	getStatsScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local usage_prefix = KEYS[4]

local current_minute = ARGV[1]
local usage_key = usage_prefix .. current_minute

local result = {}
result[1] = redis.call('LRANGE', latency_key, 0, -1)
result[2] = redis.call('LRANGE', ttft_key, 0, -1)
result[3] = redis.call('HGETALL', counters_key)
result[4] = redis.call('HGETALL', usage_key)

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
	//
	// Returns:
	//   Number of keys deleted
	deleteStatsScript = `
local latency_key = KEYS[1]
local ttft_key = KEYS[2]
local counters_key = KEYS[3]
local cooldown_key = KEYS[4]
local usage_pattern = KEYS[5]

local deleted = 0

-- Delete fixed keys
deleted = deleted + redis.call('DEL', latency_key, ttft_key, counters_key, cooldown_key)

-- Delete usage keys (pattern match)
local cursor = "0"
repeat
    local result = redis.call('SCAN', cursor, 'MATCH', usage_pattern, 'COUNT', 100)
    cursor = result[1]
    local keys = result[2]
    
    if #keys > 0 then
        deleted = deleted + redis.call('DEL', unpack(keys))
    end
until cursor == "0"

return deleted
`
)
