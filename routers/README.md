# Routers

LLMux supports multiple routing strategies to balance load and ensure high availability across different LLM providers.

## Routing Strategies

- **Simple Shuffle**: Randomly selects from available deployments. Supports weighted selection.
- **Round Robin**: Selects deployments in a strict round-robin order.
- **Lowest Latency**: Selects the deployment with the lowest average latency or Time To First Token (TTFT). **Enhanced with EWMA and dynamic weighting.**
- **Least Busy**: Selects the deployment with the fewest active requests.
- **Lowest TPM/RPM**: Selects the deployment with the lowest token/request usage.
- **Lowest Cost**: Selects the deployment with the lowest cost per token.
- **Tag-Based**: Filters deployments based on request-level tags.

## EWMA (Exponentially Weighted Moving Average)

LLMux uses the EWMA algorithm to track the performance and quality of each deployment in real-time. Unlike a simple moving average, EWMA gives more weight to recent observations, allowing the router to adapt quickly to changes in provider performance or availability.

### How it works

EWMA is calculated using the following formula:
`Value_new = Alpha * Observation + (1 - Alpha) * Value_old`

Where `Alpha` is the smoothing factor (0 < Alpha <= 1). A higher Alpha makes the average more responsive to recent changes.

LLMux tracks EWMA for:
- **Latency**: Total request duration.
- **TTFT**: Time To First Token (for streaming requests).
- **Success Rate**: A moving average of successes (1.0) and failures (0.0).

### Configuration

You can configure the smoothing factor in the routing configuration:

```yaml
routing:
  strategy: lowest-latency
  ewma_alpha: 0.1  # Default value
```

## Dynamic Weighting in LatencyRouter

The `Lowest Latency` strategy uses EWMA values for dynamic feedback. It calculates a dynamic weight for all healthy candidates within a configurable latency buffer.

### Weight Calculation

For each candidate deployment, a dynamic weight is calculated:
`Weight = BaseWeight * (SuccessRate^2) / Latency`

- **BaseWeight**: The static weight configured for the deployment (defaults to 1.0).
- **SuccessRate**: The EWMA success rate (0.0 to 1.0). Squaring it penalizes providers with even small failure rates more heavily.
- **Latency**: The EWMA latency (or TTFT). Being in the denominator means lower latency significantly increases the probability of selection.

This approach ensures that traffic is automatically shifted away from providers that are slow or failing, even if they are still technically "healthy" and haven't triggered the circuit breaker yet.
