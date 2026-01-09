# LLMux Plugin System

本文档描述 LLMux 的 Plugin 系统设计和使用方法。

## 概述

LLMux Plugin 系统提供了请求生命周期的完整控制能力，使 LLMux 从"仅观察"的 Callback 模式升级为"可拦截、可修改、可短路"的完整 Plugin 模式。

### 核心能力

| 能力 | 描述 |
|------|------|
| **请求拦截** | PreHook 在请求发送到 Provider 前执行 |
| **请求修改** | 可修改请求内容（如添加系统提示） |
| **短路返回** | 可直接返回响应或错误（如缓存命中、限流） |
| **响应修改** | PostHook 可修改响应内容 |
| **错误恢复** | PostHook 可从错误中恢复 |
| **错误转换** | PostHook 可将成功响应转为错误 |

### 执行顺序

```
请求 ──► PreHook (优先级升序) ──► Provider ──► PostHook (优先级降序) ──► 响应
                │                                    ▲
                └── 短路 ────────────────────────────┘
```

- **PreHook**: 按优先级数字升序执行（数字越小越先执行）
- **PostHook**: 按优先级数字降序执行（形成栈式调用，最后的 PreHook 对应最先的 PostHook）

## 快速开始

### 使用内置插件

```go
import (
    "github.com/blueberrycongee/llmux"
    "github.com/blueberrycongee/llmux/internal/plugin"
    "github.com/blueberrycongee/llmux/internal/plugin/builtin"
)

// 创建内置插件
loggingPlugin := builtin.NewLoggingPlugin(logger)
rateLimitPlugin := builtin.NewRateLimitPlugin(100.0, 50) // 100 req/s, burst 50
metricsPlugin := builtin.NewMetricsPlugin()

// 创建缓存插件
cacheBackend := builtin.NewMemoryCacheBackend()
cachePlugin := builtin.NewCachePlugin(cacheBackend)

// 创建 Client 并注册插件
client, err := llmux.New(
    llmux.WithProvider(llmux.ProviderConfig{
        Name:   "openai",
        Type:   "openai",
        APIKey: os.Getenv("OPENAI_API_KEY"),
        Models: []string{"gpt-4o"},
    }),
    llmux.WithPlugin(rateLimitPlugin),  // Priority: 5
    llmux.WithPlugin(cachePlugin),      // Priority: 10
    llmux.WithPlugin(metricsPlugin),    // Priority: 999
    llmux.WithPlugin(loggingPlugin),    // Priority: 1000
)
```

### 自定义插件

```go
import (
    "github.com/blueberrycongee/llmux/pkg/plugin"
    "github.com/blueberrycongee/llmux/pkg/types"
)

type MyPlugin struct{}

func (p *MyPlugin) Name() string     { return "my-plugin" }
func (p *MyPlugin) Priority() int    { return 50 }

func (p *MyPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
    // 在请求发送前执行
    // 可以修改请求、短路返回、或返回错误
    return req, nil, nil
}

func (p *MyPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
    // 在响应返回后执行
    // 可以修改响应、恢复错误、或转换错误
    return resp, err, nil
}

func (p *MyPlugin) Cleanup() error {
    // 清理资源
    return nil
}
```

## 内置插件

### LoggingPlugin

提供请求/响应的详细日志记录。

```go
loggingPlugin := builtin.NewLoggingPlugin(logger,
    builtin.WithLogRequestBody(true),   // 记录请求体
    builtin.WithLogResponseBody(true),  // 记录响应体
    builtin.WithLoggingPriority(1000),  // 自定义优先级
)
```

**默认优先级**: 1000 (最后执行 PreHook，最先执行 PostHook)

### RateLimitPlugin

实现令牌桶算法的请求限流。

```go
rateLimitPlugin := builtin.NewRateLimitPlugin(
    100.0,  // 每秒请求数
    50,     // 突发容量
    builtin.WithRateLimitKeyFunc(func(ctx *plugin.Context) string {
        // 自定义限流键（按用户/API Key 等）
        return ctx.Auth.APIKey.ID
    }),
)
```

**默认优先级**: 5 (最先执行，快速拒绝超限请求)

### CachePlugin

提供响应缓存，支持自定义后端。

```go
// 使用内存缓存
cacheBackend := builtin.NewMemoryCacheBackend(
    builtin.WithMemoryCacheMaxSize(10000),
    builtin.WithMemoryCacheCleanupInterval(5 * time.Minute),
)

cachePlugin := builtin.NewCachePlugin(cacheBackend,
    builtin.WithCacheTTL(time.Hour),
    builtin.WithCacheKeyFunc(func(req *types.ChatRequest) (string, error) {
        // 自定义缓存键生成
        return customKeyGeneration(req)
    }),
)
```

**默认优先级**: 10 (早期执行，缓存命中时短路)

### MetricsPlugin

收集请求指标，包括延迟、Token 使用量等。

```go
metricsPlugin := builtin.NewMetricsPlugin(
    builtin.WithMetricsCallback(func(m *builtin.RequestMetrics) {
        // 发送到 Prometheus、StatsD 等
        prometheus.ObserveLatency(m.LatencyMs)
    }),
)

// 获取指标快照
snapshot := metricsPlugin.GetSnapshot()
fmt.Printf("Total Requests: %d\n", snapshot.TotalRequests)
fmt.Printf("P99 Latency: %d ms\n", snapshot.P99LatencyMs)
```

**默认优先级**: 999 (接近最后执行，捕获最终状态)

## 短路机制

PreHook 可以返回 `*ShortCircuit` 来短路请求：

```go
func (p *MyPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
    // 返回缓存的响应
    if cachedResp := p.cache.Get(req); cachedResp != nil {
        return req, &plugin.ShortCircuit{
            Response: cachedResp,
            Metadata: map[string]any{"cache_hit": true},
        }, nil
    }
    
    // 返回错误（如限流）
    if p.rateLimiter.IsExceeded() {
        return req, &plugin.ShortCircuit{
            Error:         errors.New("rate limit exceeded"),
            AllowFallback: false, // 不允许 fallback
        }, nil
    }
    
    return req, nil, nil
}
```

## 错误恢复

PostHook 可以从错误中恢复：

```go
func (p *MyPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
    if err != nil {
        // 尝试从备份源恢复
        if fallbackResp := p.getFallback(ctx); fallbackResp != nil {
            return fallbackResp, nil, nil // 清除错误，返回备份响应
        }
    }
    return resp, err, nil
}
```

## 流式请求支持

实现 `StreamPlugin` 接口以支持流式请求：

```go
type MyStreamPlugin struct {
    MyPlugin
}

func (p *MyStreamPlugin) PreStreamHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.StreamShortCircuit, error) {
    // 流式请求前处理
    return req, nil, nil
}

func (p *MyStreamPlugin) OnStreamChunk(ctx *plugin.Context, chunk *types.StreamChunk) (*types.StreamChunk, error) {
    // 处理每个 chunk
    // 返回 nil 可过滤该 chunk
    return chunk, nil
}

func (p *MyStreamPlugin) PostStreamHook(ctx *plugin.Context, err error) error {
    // 流式请求完成后处理
    return nil
}
```

## Plugin Context

`plugin.Context` 提供了请求上下文信息和插件间共享数据的能力：

```go
func (p *MyPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
    // 访问请求信息
    fmt.Println("Request ID:", ctx.RequestID)
    fmt.Println("Model:", ctx.Model)
    fmt.Println("Provider:", ctx.Provider)
    fmt.Println("Is Streaming:", ctx.IsStreaming)
    
    // 访问认证信息
    if ctx.Auth != nil {
        fmt.Println("API Key ID:", ctx.Auth.APIKey.ID)
    }
    
    // 插件间共享数据
    ctx.Set("my_key", "my_value")
    
    return req, nil, nil
}

func (p *MyPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
    // 读取其他插件设置的数据
    if cacheHit := ctx.GetBool("cache_hit"); cacheHit {
        fmt.Println("Response was from cache")
    }
    
    // 读取 PreHook 设置的数据
    myValue := ctx.GetString("my_key")
    fmt.Println("My value:", myValue)
    
    return resp, err, nil
}
```

## 配置选项

```go
llmux.WithPluginConfig(plugin.PipelineConfig{
    PreHookTimeout:  5 * time.Second,   // PreHook 超时（默认 10s）
    PostHookTimeout: 5 * time.Second,   // PostHook 超时（默认 10s）
    PropagateErrors: false,             // 是否传播插件内部错误（默认 false）
    MaxPlugins:      100,               // 最大插件数量（默认 100）
})
```

## 最佳实践

1. **优先级规划**
   - 1-10: 高优先级（限流、认证等快速拒绝逻辑）
   - 10-100: 缓存、内容过滤等
   - 100-500: 业务逻辑插件
   - 500-1000: 日志、监控等观测性插件

2. **错误处理**
   - 插件内部错误（第三个返回值）不会中断请求，仅记录日志
   - 使用 `ShortCircuit.Error` 明确拒绝请求
   - 使用 PostHook 的第二个返回值覆盖响应错误

3. **性能考虑**
   - 插件使用对象池减少分配
   - 避免在热路径中进行阻塞操作
   - 使用异步操作处理非关键任务（如日志写入）

4. **测试**
   - 每个插件应有独立的单元测试
   - 测试短路场景、错误恢复场景
   - 测试并发安全性

## 与 Bifrost 对比

| 维度 | Bifrost Plugin | LLMux Plugin | 改进 |
|------|---------------|--------------|------|
| PreHook 签名 | `(ctx, req) → (req, sc, err)` | `(ctx, req) → (req, sc, err)` | ✅ 完全对齐 |
| PostHook 签名 | `(ctx, resp, err) → (resp, err, pluginErr)` | `(ctx, resp, err) → (resp, err, pluginErr)` | ✅ 完全对齐 |
| 执行顺序 | PreHook 正序, PostHook 逆序 | PreHook 正序, PostHook 逆序 | ✅ 完全对齐 |
| 超时控制 | 硬编码 10s | 可配置 | 🔧 改进 |
| 错误传递 | 静默吞掉 | 可配置 | 🔧 改进 |
| 优先级 | 注册顺序 | 显式 Priority() | 🔧 改进 |
| AllowFallback | *bool 三态 | bool 二态 | 🔧 简化 |
