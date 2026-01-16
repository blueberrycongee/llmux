# Resilience Package

High-availability patterns for the LLM gateway.

## Components

| Component         | Status               | Description                               |
| ----------------- | -------------------- | ----------------------------------------- |
| `RateLimiter`     | ✅ **ACTIVE**         | Token bucket rate limiting                |
| `RedisLimiter`    | ✅ **ACTIVE**         | Distributed rate limiting via Redis       |
| `Semaphore`       | ✅ **ACTIVE**         | Concurrency control                       |
| `AdaptiveLimiter` | ✅ **ACTIVE**         | Netflix-style adaptive concurrency limits |
| `CircuitBreaker`  | ⚠️ **NOT INTEGRATED** | Traditional circuit breaker pattern       |
| `Manager`         | ⚠️ **PARTIAL**        | Only RateLimiter/Semaphore used           |

## Adaptive Concurrency Limiter

Inspired by [Netflix Concurrency Limits](https://github.com/Netflix/concurrency-limits), the `AdaptiveLimiter` automatically adjusts the maximum concurrency based on latency (RTT) jitter.

### Key Features
- **Gradient Algorithm**: Dynamically adjusts the limit using the ratio of `minRTT` to `avgRTT`.
- **Automatic Protection**: When the backend slows down (e.g., due to queuing or load), the limiter automatically reduces the concurrency limit to prevent cascading failures.
- **Self-Healing**: As latency improves, it gradually increases the limit to maximize throughput.
- **minRTT Aging**: Periodically resets the baseline minimum RTT to adapt to changing network conditions or backend performance characteristics.

### Usage
```go
limiter := resilience.NewAdaptiveLimiter(minLimit, maxLimit)

if limiter.TryAcquire() {
    start := time.Now()
    // Perform request
    err := doRequest()
    limiter.Release(time.Since(start))
} else {
    // Return 429 Too Many Requests
}
```

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
