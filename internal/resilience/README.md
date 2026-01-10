# Resilience Package

High-availability patterns for the LLM gateway.

## Components

| Component        | Status               | Description                         |
| ---------------- | -------------------- | ----------------------------------- |
| `RateLimiter`    | ✅ **ACTIVE**         | Token bucket rate limiting          |
| `RedisLimiter`   | ✅ **ACTIVE**         | Distributed rate limiting via Redis |
| `Semaphore`      | ✅ **ACTIVE**         | Concurrency control                 |
| `CircuitBreaker` | ⚠️ **NOT INTEGRATED** | Traditional circuit breaker pattern |
| `Manager`        | ⚠️ **PARTIAL**        | Only RateLimiter/Semaphore used     |

## Circuit Breaker Status

The `CircuitBreaker` implementation provides a traditional circuit breaker pattern with:
- Closed → Open transition after N consecutive failures
- Half-open state for recovery probing  
- Gradual recovery with success threshold

**However, this is NOT used in production.**

### Why?

The router (`routers/base.go`) uses a **LiteLLM-style failure-rate based cooldown** instead, which is more suitable for LLM APIs:

| Feature      | Traditional CircuitBreaker | LiteLLM-style Cooldown (Active)     |
| ------------ | -------------------------- | ----------------------------------- |
| Trigger      | N consecutive failures     | Failure rate > 50% (min 5 requests) |
| 429 handling | Counted as failure         | **Immediate cooldown**              |
| Half-open    | ✅ Yes                      | ❌ No (time-based recovery)          |
| Best for     | Stable services            | Bursty LLM API errors               |

### Active Implementation

See `routers/base.go`:
- `ReportFailure()` - Implements cooldown logic
- `shouldCooldownByFailureRate()` - Failure rate calculation
- `IsCircuitOpen()` - Checks cooldown status

### Future Considerations

The traditional circuit breaker may be integrated if:
1. Half-open state probing is needed
2. More gradual recovery patterns are required
3. Provider-specific circuit breaker configurations are needed
