# LiteLLM 代码深度分析：翻译 vs 重写

## 一、可以快速翻译的部分（照抄逻辑）

这些代码逻辑简单、模式固定，可以让 AI 直接从 Python 翻译成 Go：

### 1. 类型定义 (Types)

| 文件 | 内容 | 翻译难度 | 说明 |
|------|------|----------|------|
| types/utils.py | ModelResponse, Usage, Choices | ⭐ | 纯数据结构，直接转 struct |
| types/llms/openai.py | OpenAI 请求/响应类型 | ⭐ | TypedDict → Go struct |
| types/llms/anthropic.py | Anthropic 类型 | ⭐ | 同上 |
| types/router.py | 路由相关类型 | ⭐ | 同上 |

**Python 示例**：

```python
class ModelResponse:
    id: str
    object: str = "chat.completion"
    created: int
    model: str
    choices: List[Choices]
    usage: Usage
```

**Go 翻译**：

```go
type ModelResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   *Usage   `json:"usage,omitempty"`
}
```

### 2. 异常/错误定义 (Exceptions)

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| exceptions.py | 统一异常类型 | ⭐ |

**Python**：

```python
class RateLimitError(openai.RateLimitError):
    def __init__(self, message, llm_provider, model, ...):
        self.status_code = 429
        self.message = f"litellm.RateLimitError: {message}"
        self.llm_provider = llm_provider
        self.model = model
```

**Go 翻译**：

```go
type RateLimitError struct {
    StatusCode int
    Message    string
    Provider   string
    Model      string
    Retries    int
    MaxRetries int
}

func (e *RateLimitError) Error() string {
    return fmt.Sprintf("RateLimitError: %s (provider=%s, model=%s)", 
        e.Message, e.Provider, e.Model)
}
```

### 3. 参数映射逻辑 (Parameter Mapping)

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| llms/anthropic/chat/transformation.py 的 map_openai_params() | OpenAI → Anthropic 参数转换 | ⭐⭐ |
| llms/openai/chat/gpt_transformation.py 的 map_openai_params() | OpenAI 参数过滤 | ⭐ |

**Python**：

```python
def _map_tool_choice(self, tool_choice, parallel_tool_use):
    if tool_choice == "auto":
        return {"type": "auto"}
    elif tool_choice == "required":
        return {"type": "any"}
    elif tool_choice == "none":
        return {"type": "none"}
    elif isinstance(tool_choice, dict):
        return {"type": "tool", "name": tool_choice["function"]["name"]}
```

**Go 翻译**：

```go
func (c *AnthropicConfig) MapToolChoice(toolChoice any) map[string]any {
    switch v := toolChoice.(type) {
    case string:
        switch v {
        case "auto":
            return map[string]any{"type": "auto"}
        case "required":
            return map[string]any{"type": "any"}
        case "none":
            return map[string]any{"type": "none"}
        }
    case map[string]any:
        if fn, ok := v["function"].(map[string]any); ok {
            return map[string]any{"type": "tool", "name": fn["name"]}
        }
    }
    return nil
}
```

### 4. 请求/响应转换 (Transform Request/Response)

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| llms/*/chat/transformation.py 的 transform_request() | 构建请求体 | ⭐⭐ |
| llms/*/chat/transformation.py 的 transform_response() | 解析响应体 | ⭐⭐ |

**Python**：

```python
def transform_request(self, model, messages, optional_params, ...):
    return {
        "model": model,
        "messages": messages,
        **optional_params,
    }
```

**Go 翻译**：

```go
func (c *OpenAIConfig) TransformRequest(model string, messages []Message, params map[string]any) map[string]any {
    req := map[string]any{
        "model":    model,
        "messages": messages,
    }
    for k, v := range params {
        req[k] = v
    }
    return req
}
```

### 5. 价格表和模型信息

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| model_prices_and_context_window.json | 价格、token 限制 | ⭐ 直接复制 |

**直接嵌入 Go**：

```go
//go:embed model_prices.json
var modelPricesJSON []byte

var ModelPrices map[string]ModelInfo

func init() {
    json.Unmarshal(modelPricesJSON, &ModelPrices)
}
```

### 6. 路由策略的选择逻辑

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| router_strategy/simple_shuffle.py | 随机选择 | ⭐ |
| router_strategy/lowest_latency.py 的 _get_available_deployments() | 最低延迟选择 | ⭐⭐ |

**Python**：

```python
def _get_available_deployments(self, model_group, healthy_deployments, ...):
    # 按延迟排序
    sorted_deployments = sorted(potential_deployments, key=lambda x: x[1])
    lowest_latency = sorted_deployments[0][1]
    # 在 buffer 范围内随机选
    valid = [x for x in sorted_deployments if x[1] <= lowest_latency + buffer]
    return random.choice(valid)[0]
```

**Go 翻译**：

```go
func (r *LowestLatencyRouter) Pick(deployments []*Deployment) *Deployment {
    sort.Slice(deployments, func(i, j int) bool {
        return deployments[i].AvgLatency < deployments[j].AvgLatency
    })
    lowest := deployments[0].AvgLatency
    buffer := r.LatencyBuffer * lowest
    
    var valid []*Deployment
    for _, d := range deployments {
        if d.AvgLatency <= lowest+buffer {
            valid = append(valid, d)
        }
    }
    return valid[rand.Intn(len(valid))]
}
```

### 7. 冷却判断逻辑

| 文件 | 内容 | 翻译难度 |
|------|------|----------|
| router_utils/cooldown_handlers.py 的 _is_cooldown_required() | 判断是否需要冷却 | ⭐⭐ |
| router_utils/cooldown_handlers.py 的 _should_cooldown_deployment() | 判断是否触发冷却 | ⭐⭐ |

**Python**：

```python
def _is_cooldown_required(model_id, exception_status):
    if exception_status >= 400 and exception_status < 500:
        if exception_status == 429:  # Rate Limit
            return True
        elif exception_status == 401:  # Auth Error
            return True
        elif exception_status == 408:  # Timeout
            return True
        else:
            return False  # 其他 4xx 不冷却
    return True  # 5xx 都冷却
```

**Go 翻译**：

```go
func IsCooldownRequired(statusCode int) bool {
    if statusCode >= 400 && statusCode < 500 {
        switch statusCode {
        case 429, 401, 408, 404:
            return true
        default:
            return false
        }
    }
    return true // 5xx 都冷却
}
```

## 二、需要用 Go 特性重写的部分

这些代码依赖 Python 特性或性能敏感，必须用 Go 的方式重新实现：

### 1. 🔥 SSE 流式转发（最复杂）

**Python 实现问题**：
- 使用 async for + yield 的 generator 模式
- 依赖 Python 的 asyncio 事件循环
- 内存管理不可控

**Go 重写要点**：

```go
type SSEForwarder struct {
    upstream   io.ReadCloser
    downstream http.ResponseWriter
    clientCtx  context.Context
    bufferPool *sync.Pool  // 复用 buffer
}

func (f *SSEForwarder) Forward() error {
    buf := f.bufferPool.Get().([]byte)
    defer f.bufferPool.Put(buf)
    
    reader := bufio.NewReader(f.upstream)
    flusher, _ := f.downstream.(http.Flusher)
    
    for {
        select {
        case <-f.clientCtx.Done():
            // 🔥 关键：client 断开立即取消上游
            return f.clientCtx.Err()
        default:
            line, err := reader.ReadBytes('\n')
            if err == io.EOF {
                return nil
            }
            if err != nil {
                return err
            }
            
            // 写入下游
            f.downstream.Write(line)
            flusher.Flush()
        }
    }
}
```

**关键差异**：

| Python | Go |
|--------|-----|
| async for chunk in stream | for { reader.ReadBytes('\n') } |
| GC 自动回收 | sync.Pool 手动复用 buffer |
| asyncio cancel | context.Context cancel |
| 无背压控制 | 可以加 chan 做背压 |

### 2. 🔥 HTTP Client 连接池

**Python 实现**：
- 使用 httpx.AsyncClient 或 aiohttp
- 连接池配置分散在多处
- SSL 配置复杂

**Go 重写**：

```go
type ProviderClient struct {
    client    *http.Client
    transport *http.Transport
    semaphore chan struct{}  // 并发控制
}

func NewProviderClient(maxConns int, timeout time.Duration) *ProviderClient {
    transport := &http.Transport{
        MaxIdleConns:        maxConns,
        MaxIdleConnsPerHost: maxConns,
        IdleConnTimeout:     90 * time.Second,
        // 🔥 关键：连接复用
        DisableKeepAlives:   false,
    }
    
    return &ProviderClient{
        client: &http.Client{
            Transport: transport,
            Timeout:   timeout,
        },
        semaphore: make(chan struct{}, maxConns),
    }
}

func (c *ProviderClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    // 获取 permit
    select {
    case c.semaphore <- struct{}{}:
        defer func() { <-c.semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    
    return c.client.Do(req.WithContext(ctx))
}
```

### 3. 🔥 熔断器 (Circuit Breaker)

**Python 实现**：
- LiteLLM 用简单的 cooldown cache（不是真正的熔断器）
- 基于时间窗口的失败计数

**Go 重写（使用 gobreaker）**：

```go
import "github.com/sony/gobreaker"

type ProviderBreaker struct {
    breakers map[string]*gobreaker.CircuitBreaker  // per-provider
    mu       sync.RWMutex
}

func NewProviderBreaker() *ProviderBreaker {
    return &ProviderBreaker{
        breakers: make(map[string]*gobreaker.CircuitBreaker),
    }
}

func (pb *ProviderBreaker) GetBreaker(providerID string) *gobreaker.CircuitBreaker {
    pb.mu.RLock()
    if cb, ok := pb.breakers[providerID]; ok {
        pb.mu.RUnlock()
        return cb
    }
    pb.mu.RUnlock()
    
    pb.mu.Lock()
    defer pb.mu.Unlock()
    
    // Double check
    if cb, ok := pb.breakers[providerID]; ok {
        return cb
    }
    
    cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        providerID,
        MaxRequests: 3,                    // 半开状态最多放 3 个请求
        Interval:    60 * time.Second,     // 统计窗口
        Timeout:     30 * time.Second,     // 熔断后多久尝试恢复
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            // 🔥 关键：失败率 > 50% 且请求数 > 10 才熔断
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 10 && failureRatio >= 0.5
        },
    })
    
    pb.breakers[providerID] = cb
    return cb
}
```

### 4. 🔥 配置热重载

**Python 实现**：
- LiteLLM 需要重启才能更新配置
- 没有真正的热重载

**Go 重写**：

```go
type ConfigManager struct {
    config  atomic.Pointer[Config]
    watcher *fsnotify.Watcher
}

func (cm *ConfigManager) Watch(ctx context.Context, path string) error {
    watcher, _ := fsnotify.NewWatcher()
    cm.watcher = watcher
    watcher.Add(path)
    
    // 防抖
    debounce := time.NewTimer(0)
    <-debounce.C
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                // 防抖：500ms 内多次变更只触发一次
                debounce.Reset(500 * time.Millisecond)
            }
        case <-debounce.C:
            newConfig, err := LoadConfig(path)
            if err != nil {
                log.Error("reload failed, keeping old config", "err", err)
                continue
            }
            // 🔥 关键：原子替换
            cm.config.Store(newConfig)
            log.Info("config reloaded")
        }
    }
}

func (cm *ConfigManager) Get() *Config {
    return cm.config.Load()
}
```

### 5. 🔥 优雅关闭 (Graceful Shutdown)

**Python 实现**：
- 简单的 signal handler
- 没有 drain mode

**Go 重写**：

```go
type Server struct {
    httpServer   *http.Server
    activeConns  sync.WaitGroup
    shuttingDown atomic.Bool
}

func (s *Server) Shutdown(ctx context.Context) error {
    s.shuttingDown.Store(true)
    
    // 1. 停止接收新请求
    s.httpServer.SetKeepAlivesEnabled(false)
    
    // 2. 等待现有请求完成（最多 60s）
    done := make(chan struct{})
    go func() {
        s.activeConns.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        log.Info("all connections drained")
    case <-ctx.Done():
        log.Warn("drain timeout, forcing shutdown")
    }
    
    // 3. 关闭 HTTP server
    return s.httpServer.Shutdown(ctx)
}

// 中间件：追踪活跃连接
func (s *Server) TrackConnection(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if s.shuttingDown.Load() {
            http.Error(w, "server shutting down", http.StatusServiceUnavailable)
            return
        }
        s.activeConns.Add(1)
        defer s.activeConns.Done()
        next.ServeHTTP(w, r)
    })
}
```

### 6. 🔥 Metrics 埋点

**Python 实现**：
- 分散在各处的 logging
- 没有结构化的 metrics

**Go 重写**：

```go
var (
    requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "llm_requests_total",
        Help: "Total LLM requests",
    }, []string{"provider", "model", "status"})
    
    requestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "llm_request_latency_seconds",
        Help:    "Request latency",
        Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
    }, []string{"provider", "model"})
    
    tokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "llm_token_usage_total",
        Help: "Token usage",
    }, []string{"provider", "model", "type"})  // type: input/output
)

// 中间件
func MetricsMiddleware(provider, model string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // 包装 ResponseWriter 捕获状态码
            wrapped := &statusRecorder{ResponseWriter: w, status: 200}
            next.ServeHTTP(wrapped, r)
            
            // 记录 metrics
            status := strconv.Itoa(wrapped.status)
            requestsTotal.WithLabelValues(provider, model, status).Inc()
            requestLatency.WithLabelValues(provider, model).Observe(time.Since(start).Seconds())
        })
    }
}
```

### 7. 🔥 并发控制 (Semaphore / Bulkhead)

**Python 实现**：
- 没有 per-provider 的并发限制
- 全局共享资源

**Go 重写**：

```go
type Bulkhead struct {
    semaphores map[string]chan struct{}
    mu         sync.RWMutex
}

func NewBulkhead(limits map[string]int) *Bulkhead {
    b := &Bulkhead{semaphores: make(map[string]chan struct{})}
    for provider, limit := range limits {
        b.semaphores[provider] = make(chan struct{}, limit)
    }
    return b
}

func (b *Bulkhead) Acquire(ctx context.Context, provider string) error {
    b.mu.RLock()
    sem, ok := b.semaphores[provider]
    b.mu.RUnlock()
    
    if !ok {
        return nil // 没有限制
    }
    
    select {
    case sem <- struct{}{}:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("bulkhead: %s is full", provider)
    }
}

func (b *Bulkhead) Release(provider string) {
    b.mu.RLock()
    sem, ok := b.semaphores[provider]
    b.mu.RUnlock()
    
    if ok {
        <-sem
    }
}
```

## 三、总结对照表

| 模块 | 翻译/重写 | 工作量 | 优先级 |
|------|----------|--------|--------|
| 类型定义 | 翻译 | ⭐ | P0 |
| 异常定义 | 翻译 | ⭐ | P0 |
| 参数映射 | 翻译 | ⭐⭐ | P1 |
| 请求转换 | 翻译 | ⭐⭐ | P1 |
| 响应转换 | 翻译 | ⭐⭐ | P1 |
| 价格表 | 直接复制 | ⭐ | P1 |
| 路由选择逻辑 | 翻译 | ⭐⭐ | P2 |
| 冷却判断逻辑 | 翻译 | ⭐⭐ | P2 |
| SSE 流式转发 | 重写 | ⭐⭐⭐⭐⭐ | P0 |
| HTTP Client | 重写 | ⭐⭐⭐ | P0 |
| 熔断器 | 重写 | ⭐⭐⭐ | P1 |
| 配置热重载 | 重写 | ⭐⭐⭐ | P1 |
| 优雅关闭 | 重写 | ⭐⭐ | P1 |
| Metrics | 重写 | ⭐⭐ | P0 |
| 并发控制 | 重写 | ⭐⭐⭐ | P1 |

## 四、建议的开发顺序

**Week 1: 骨架**
```
├── 定义 Go interface (Provider, Router)
├── 实现 HTTP Server
├── 实现 Metrics 中间件
└── 实现 OpenAI Provider (手写，作为模板)
```

**Week 2: AI 批量生成**
```
├── 让 AI 翻译所有类型定义
├── 让 AI 翻译 Anthropic/Azure/Gemini adapter
└── 人工 review
```

**Week 3: 流式 (最难)**
```
├── 实现 SSE 转发核心
├── sync.Pool buffer 复用
├── client 断开检测
└── 测试各种边界情况
```

**Week 4: 高可用**
```
├── 集成 gobreaker
├── 实现 Bulkhead
├── 实现配置热重载
└── 实现优雅关闭
```

**Week 5: 打磨**
```
├── 压测
├── 调参
├── Docker 镜像
└── 文档
```