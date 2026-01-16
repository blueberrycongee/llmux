# Resilience 模块 (稳定性与韧性)

LLM 网关的高可用模式实现。

## 组件

| 组件 | 状态 | 描述 |
| ----------------- | -------------------- | ----------------------------------------- |
| `RateLimiter` | ✅ **活跃** | 令牌桶限流 |
| `RedisLimiter` | ✅ **活跃** | 基于 Redis 的分布式限流 |
| `Semaphore` | ✅ **活跃** | 并发控制 |
| `AdaptiveLimiter` | ✅ **活跃** | Netflix 风格的自适应并发限流 |
| `CircuitBreaker` | ⚠️ **未集成** | 传统的断路器模式 |
| `Manager` | ⚠️ **部分集成** | 仅使用了 RateLimiter/Semaphore |

## 自适应并发限流器 (Adaptive Concurrency Limiter)

受 [Netflix Concurrency Limits](https://github.com/Netflix/concurrency-limits) 启发，`AdaptiveLimiter` 根据延时 (RTT) 的抖动自动调节最大并发数。

### 关键特性
- **梯度算法 (Gradient Algorithm)**: 使用 `minRTT` 与 `avgRTT` 的比值动态调整限流值。
- **自动保护**: 当后端变慢（如由于排队或过载）时，限流器会自动降低并发上限，防止雪崩效应。
- **自愈**: 随着延时改善，它会逐渐增加限流值以最大化吞吐。
- **minRTT 老化**: 周期性重置基准最小 RTT，以适应网络状况或后端性能的变化。

### 使用示例
```go
limiter := resilience.NewAdaptiveLimiter(minLimit, maxLimit)

if limiter.TryAcquire() {
    start := time.Now()
    // 执行请求
    err := doRequest()
    limiter.Release(time.Since(start))
} else {
    // 返回 429 Too Many Requests
}
```

## 断路器状态说明

`CircuitBreaker` 实现提供了传统的断路器模式：
- 连续 N 次失败后从 Closed 转为 Open
- 包含用于恢复探测的 Half-open 状态
- 根据成功阈值逐渐恢复

**注意：此传统模式目前未在生产环境中使用。**

### 为什么不直接使用它？

路由器 (`routers/base.go`) 使用了**类 LiteLLM 风格的基于失败率的冷却机制**，这更适合 LLM API 场景：

| 特性 | 传统断路器 (CircuitBreaker) | 类 LiteLLM 冷却 (当前活跃) |
| ------------ | -------------------------- | ----------------------------------- |
| 触发条件 | N 次连续失败 | 失败率 > 50% (最少 5 次请求) |
| 429 处理 | 计为失败 | **立即进入冷却** |
| 半开启状态 | ✅ 支持 | ❌ 不支持 (基于时间恢复) |
| 适用场景 | 稳定的后端服务 | 突发性的 LLM API 错误 |

### 活跃实现

请参考 `routers/base.go`:
- `ReportFailure()` - 实现冷却逻辑
- `shouldCooldownByFailureRate()` - 计算失败率
- `IsCircuitOpen()` - 检查冷却状态
