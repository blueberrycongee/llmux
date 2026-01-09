# Distributed Rate Limiting Design (Litellm V3 Alignment)

## Progress
- [ ] **Phase 1: Interface Definition** (In Progress)
  - [x] Define `DistributedLimiter` interface and types
- [ ] **Phase 2: Redis Implementation**
  - [ ] Implement Lua script wrapper
  - [ ] Unit tests with `miniredis`
- [ ] **Phase 3: Integration**
  - [ ] Integrate into `internal/auth`


## 1. Overview
This document outlines the design for implementing distributed rate limiting in `llmux`, strictly aligning with `litellm`'s "Robust/Experimental" (V3) architecture. The goal is to achieve feature parity and reliability by adopting their proven Lua-script-based approach.

## 2. Core Architecture

### 2.1 Algorithm: Atomic Fixed Window
We will replicate `litellm`'s `BATCH_RATE_LIMITER_SCRIPT`. This uses a **Fixed Window** algorithm implemented in Lua to ensure atomicity across distributed instances.

**Why Fixed Window?**
- **Simplicity**: Easy to implement and debug.
- **Performance**: Requires minimal Redis memory (just counters) and operations.
- **Atomicity**: Lua script ensures "Check-and-Increment" happens in a single step.
- **Litellm Alignment**: This is exactly what `litellm` uses in production for high-scale deployments.

### 2.2 Redis Lua Script
The core logic resides in a Lua script that handles multiple limits (RPM, TPM) in a single round-trip.

```lua
-- BATCH_RATE_LIMITER_SCRIPT (Ported from litellm)
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
```

### 2.3 Key Management & Redis Cluster Support
To support Redis Cluster, we must ensure all keys related to a specific limit map to the same hash slot.

- **Strategy**: Use Hash Tags `{...}`.
- **Format**: `{{api_key}:{model}}:window`
- **Logic**: By enclosing the "identity" part in curly braces `{}`, Redis guarantees that the window key and the counter key are stored on the same node, allowing the Lua script to execute.

### 2.4 Fallback Mechanism
If Redis is unavailable, `llmux` will degrade gracefully:
1.  **Log Error**: Record the Redis failure.
2.  **In-Memory Fallback**: Switch to a local in-memory rate limiter (using `sync.Map` or `golang.org/x/time/rate`).
3.  **Fail-Open/Closed**: Configurable, but defaulting to **Fail-Open** (allow request) or **Best-Effort** (local limit) is usually preferred over blocking all traffic.

## 3. Go Implementation Plan

### 3.1 Interface Definition (`internal/resilience`)

We will define a `DistributedLimiter` interface that supports batch checking, mirroring `litellm`'s capability to check RPM and TPM simultaneously.

```go
package resilience

import (
    "context"
    "time"
)

// LimitType defines what we are limiting (Requests, Tokens, etc.)
type LimitType string

const (
    LimitTypeRequests LimitType = "requests" // RPM
    LimitTypeTokens   LimitType = "tokens"   // TPM
)

// Descriptor defines a specific limit rule
type Descriptor struct {
    Key        string        // e.g., "api-key-123"
    Value      string        // e.g., "model-gpt4"
    Limit      int64         // The limit threshold (e.g., 100)
    Type       LimitType     // RPM or TPM
    Window     time.Duration // Window size (default 1m)
}

// LimitResult contains the result of a check
type LimitResult struct {
    Allowed     bool
    Current     int64
    Remaining   int64
    ResetAt     int64 // Timestamp when window resets
    Error       error
}

type DistributedLimiter interface {
    // CheckAllow atomically checks and increments limits for multiple descriptors.
    // Returns a list of results corresponding to the input descriptors.
    CheckAllow(ctx context.Context, descriptors []Descriptor) ([]LimitResult, error)
}
```

### 3.2 Redis Implementation (`internal/resilience/redis_limiter.go`)

- **Dependencies**: `github.com/redis/go-redis/v9`
- **Struct**: `RedisLimiter`
- **Logic**:
    1.  Prepare keys using Hash Tags.
    2.  Load Lua script (using `Script.Run`).
    3.  Parse results.
    4.  Handle Redis errors (trigger fallback).

### 3.3 Integration Point (`internal/auth`)

The existing `TenantRateLimiter` in `internal/auth` will be updated or wrapped to use `DistributedLimiter`.

- **Current**: In-memory `map[string]*rate.Limiter`.
- **New**: 
    - Initialize `RedisLimiter` if Redis config is present.
    - In `RateLimitMiddleware`, construct `Descriptor` list (Global, Team, User, Model).
    - Call `CheckAllow`.
    - If allowed, proceed. If not, return 429.

## 4. Configuration

New configuration options in `config.yaml`:

```yaml
redis:
  address: "localhost:6379"
  password: ""
  db: 0
  cluster_mode: false

ratelimit:
  enabled: true
  fallback_strategy: "local" # or "fail-open", "block"
```

## 5. Migration Steps

1.  **Add Dependencies**: `go get github.com/redis/go-redis/v9`
2.  **Create Interface**: Define `DistributedLimiter` in `internal/resilience`.
3.  **Implement Redis Limiter**: Port the Lua script and Go wrapper.
4.  **Unit Tests**: Use `miniredis` to verify Lua script logic.
5.  **Integration**: Wire into `internal/auth/ratelimiter.go`.
