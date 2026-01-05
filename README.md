# LiteLLM Go Proxy

一个使用 Go 语言编写的 LLM API 代理服务，旨在解决 litellm 的一些痛点问题。

## 项目背景

[litellm](https://github.com/BerriAI/litellm) 是一个流行的 LLM API 代理工具，但存在以下问题：

1. **性能问题** - Python 实现在高并发场景下性能受限
2. **部署复杂** - 依赖 Python 环境，部署和维护成本较高
3. **资源占用** - 内存占用较大，冷启动时间长
4. **稳定性** - 在长时间运行时可能出现内存泄漏等问题

## 项目目标

使用 Go 语言重新实现核心功能，提供：

- **高性能** - 利用 Go 的并发模型，支持高并发请求
- **易部署** - 单一二进制文件，无需额外运行时依赖
- **低资源** - 更低的内存占用和更快的启动速度
- **高稳定** - Go 的内存管理更加可靠

## 核心功能

### 1. 多模型支持
- OpenAI (GPT-3.5, GPT-4, GPT-4o 等)
- Anthropic (Claude 系列)
- Azure OpenAI
- Google Gemini
- 本地模型 (Ollama, vLLM 等)

### 2. 统一 API 接口
- 兼容 OpenAI API 格式
- 支持 Chat Completions
- 支持 Embeddings
- 支持流式响应 (SSE)

### 3. 负载均衡与路由
- 多后端负载均衡
- 基于模型的智能路由
- 故障转移 (Fallback)
- 请求重试机制

### 4. 认证与限流
- API Key 管理
- 请求速率限制
- 用量统计与配额

### 5. 可观测性
- 结构化日志
- Prometheus 指标
- 请求追踪

## 技术架构

```
┌─────────────────────────────────────────────────────────┐
│                      Client                              │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                   API Gateway                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   Auth      │  │  Rate Limit │  │   Logging   │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                   Router / Load Balancer                 │
└─────────────────────────────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │  OpenAI  │    │ Anthropic│    │  Azure   │
    └──────────┘    └──────────┘    └──────────┘
```

## 项目结构

```
.
├── cmd/
│   └── server/          # 主程序入口
├── internal/
│   ├── api/             # HTTP 处理器
│   ├── config/          # 配置管理
│   ├── middleware/      # 中间件 (认证、限流、日志)
│   ├── provider/        # LLM 提供商适配器
│   │   ├── openai/
│   │   ├── anthropic/
│   │   ├── azure/
│   │   └── gemini/
│   ├── router/          # 路由与负载均衡
│   └── model/           # 数据模型
├── pkg/
│   └── client/          # 可复用的客户端库
├── config/
│   └── config.yaml      # 配置文件示例
├── go.mod
├── go.sum
└── README.md
```

## 配置示例

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 60s

providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    models:
      - gpt-4o
      - gpt-4-turbo
      - gpt-3.5-turbo

  - name: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com
    models:
      - claude-3-opus
      - claude-3-sonnet

routing:
  default_provider: openai
  fallback_enabled: true
  retry_count: 3

rate_limit:
  enabled: true
  requests_per_minute: 60

logging:
  level: info
  format: json
```

## 快速开始

### 安装

```bash
go install github.com/yourname/litellm-go@latest
```

### 运行

```bash
# 使用配置文件
litellm-go --config config.yaml

# 或使用环境变量
export OPENAI_API_KEY=sk-xxx
litellm-go
```

### 使用

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 代码质量标准

本项目对标 GitHub 最高标准的开源项目，严格遵循以下规范：

### 代码规范

- **Go 官方规范** - 严格遵循 [Effective Go](https://go.dev/doc/effective_go) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- **代码格式化** - 使用 `gofmt` / `goimports` 统一代码风格
- **静态检查** - 使用 `golangci-lint` 进行全面的静态分析
- **零容忍屎山** - 拒绝复杂嵌套、超长函数、魔法数字、重复代码

### 注释规范

- **全英文注释** - 所有代码注释、文档注释均使用英文
- **GoDoc 标准** - 导出的函数、类型、常量必须有符合 GoDoc 规范的注释
- **清晰表达意图** - 注释解释 "为什么" 而非 "是什么"

```go
// Router dispatches incoming requests to the appropriate LLM provider
// based on the requested model and current load balancing strategy.
type Router struct {
    // providers holds all registered LLM provider instances
    providers map[string]Provider
    
    // strategy determines how requests are distributed across providers
    strategy LoadBalanceStrategy
}

// Route selects the best available provider for the given model.
// It returns ErrNoAvailableProvider if all providers are unavailable.
func (r *Router) Route(ctx context.Context, model string) (Provider, error) {
    // ...
}
```

### 测试标准

- **测试覆盖率** - 核心逻辑覆盖率 > 80%，整体覆盖率 > 70%
- **单元测试** - 每个 package 必须有对应的 `_test.go` 文件
- **表驱动测试** - 使用 table-driven tests 提高测试可读性
- **Mock 与集成测试** - 关键路径需要有集成测试覆盖

```bash
# 运行测试并查看覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 项目质量工具

| 工具 | 用途 |
|------|------|
| `golangci-lint` | 静态代码分析 |
| `go test -race` | 竞态检测 |
| `go vet` | 代码检查 |
| `govulncheck` | 安全漏洞扫描 |

### CI/CD 检查项

每次 PR 必须通过以下检查：

- [ ] `golangci-lint run` 无错误
- [ ] `go test -race ./...` 全部通过
- [ ] 测试覆盖率不低于基准线
- [ ] 无安全漏洞警告

## 架构设计理念

### 一、极致稳定性 (Extreme Stability)

**目标：做到"挂不掉的网关"**，即使上游 OpenAI 崩了，你的网关也不能崩；即使流量突增 10 倍，也不能 OOM。

#### 1. 熔断与隔离 (Circuit Breaking & Bulkheading)

Python 项目常用简单的 Retry，这在生产环境是灾难（会导致级联故障）。Go 有更优雅的模式。

**痛点**：当某个上游（如 GPT-4）响应变慢时，所有 Goroutine 都阻塞在那里等待，耗尽连接池，导致健康的 Gemini 请求也进不来。

**Go 方案**：
- 引入 `sony/gobreaker` 或自行实现滑动窗口熔断器
- **舱壁模式 (Bulkhead Pattern)**：为每个 Provider 分配独立的 Semaphore（信号量）或 Worker Pool
  - 例如：OpenAI 限制并发 1000，Claude 限制并发 500
  - OpenAI 挂了，Claude 的通道依然畅通

#### 2. 确定性的资源管理 (Deterministic Resource Management)

**痛点**：Python 的内存回收不可控，高并发下容易 OOM。

**Go 方案**：
- **Bounded Concurrency**：必须有全局和分租户的 `WeightedSemaphore`。不仅仅限制 QPS，更要限制 In-flight Token 预估总量
- **Zero-Allocation Pipeline**：对于 SSE 流式转发，使用 `sync.Pool` 复用 `[]byte` buffer
- **Context 强绑定**：

```go
// 伪代码：一旦 Client 断开，立即取消上游请求，节省 Token
go func() {
    select {
    case <-clientCtx.Done():
        upstreamCancel() // 毫秒级切断，帮老板省钱
    case <-upstreamResult:
        // ...
    }
}()
```

---

### 二、极致云原生 (True Cloud-Native)

**目标：让 K8s 运维人员爱死这个项目**。它应该像一个标准的 K8s 组件一样行为。

#### 1. Sidecar 友好型架构

**痛点**：LiteLLM 镜像太大（1GB+），无法作为 Sidecar 注入到业务 Pod 中。

**Go 方案**：
- **Distroless/Scratch 镜像**：最终产物只有 binary，体积控制在 20MB 以内
- **Unix Domain Socket (UDS)**：支持通过 UDS 通信。当作为 Sidecar 部署时，业务容器通过 localhost 的 socket 文件调用网关，性能比 TCP 高 30% 且无需经过网络栈

#### 2. 配置热重载 (Hot Reload) 与动态发现

**痛点**：改个 API Key 就要重启服务，导致短暂断连。

**Go 方案**：
- 利用 `fsnotify` 监听 ConfigMap 挂载目录
- 文件变更时，**原子替换 (Atomic Swap)** 内存中的配置指针，实现 Zero-Downtime Reconfiguration
- **Operator 模式**：未来可以写一个 K8s Operator，通过 CRD (`kind: LLMRoute`) 来管理路由规则，而不是改配置文件

#### 3. 探针与优雅关闭 (Probes & Graceful Shutdown)

**Go 方案**：
- 区分 `/health/live` (进程还在吗) 和 `/health/ready` (能处理请求吗？比如是否还没加载完配置)
- **Drain Mode**：接收到 `SIGTERM` 后，停止接收新请求，但等待当前处理中的流式请求完成（设置最长等待时间，例如 60s），再退出进程

---

### 三、极致可观测性 (Deep Observability)

**目标：让其成为 AI 流量的"显微镜"**。这是企业买单的核心功能。

#### 1. 结构化与动态日志

**Go 方案**：
- 使用 `slog` (Go 1.21+) 或 `uber-go/zap`
- **Redaction (脱敏)**：实现一个高性能的 `LogFieldMarshaler`，自动把 API Key、Prompt 中的 PII 信息 mask 掉，防止日志泄露隐私

#### 2. 全链路追踪 (Distributed Tracing)

**痛点**：用户问"为什么这个请求慢？"，LiteLLM 很难回答是网络慢、OpenAI 慢还是网关慢。

**Go 方案**：
- 集成 **OpenTelemetry (OTel) SDK**
- 在 Span 中注入关键 Metadata：
  - `gen_ai.system` (openai/anthropic)
  - `gen_ai.request.model`
  - `gen_ai.usage.input_tokens`
- **关键点**：Trace ID 必须能从 HTTP Header 透传，串联起 `客户端 -> 网关 -> 上游` 的完整链路

#### 3. 成本归因指标 (Cost Attribution Metrics)

**Go 方案**：
- 直接暴露 **Prometheus Metrics**
- 不要只给简单的 QPS，高价值指标：

```prometheus
llm_token_usage_total{team="marketing", model="gpt-4"}  # Counter
llm_request_latency_seconds_bucket                       # Histogram
llm_upstream_error_total{error_type="429"}               # Counter
```

- **Token 计算**：集成 `pkoukk/tiktoken-go`，在网关层估算 Token 消耗（虽然不是 100% 准确，但对实时监控足够了），不用完全依赖上游返回

---

## 开发计划 (MVP 优先级)

既然我们明确了定位，MVP (Minimum Viable Product) 就不应该只是"能跑通"，而是要体现"架构美感"。

### Phase 1: The Strong Skeleton (骨架)

- [ ] **Transport 层**：基于 `fasthttp` 或标准库 `net/http` 调优的 Server
- [ ] **Observability 层**：先做 Metrics 和 Logging。代码里要是没有 Metrics 埋点，Review 时直接打回
- [ ] **Config 层**：实现基于 `fsnotify` 的热加载

### Phase 2: The Logic (血肉)

- [ ] **Adapter 生成器**：写个脚本，把 LiteLLM 的 Python 逻辑喂给 LLM，让它生成 Go 的 Adapter struct
- [ ] **Router**：简单的 Round-Robin
- [ ] 更多提供商支持 (Anthropic, Azure, Gemini)
- [ ] 故障转移 (Fallback)

### Phase 3: The Shield (铠甲)

- [ ] **Circuit Breaker**：集成熔断器 (`sony/gobreaker`)
- [ ] **Rate Limiter**：基于 Token Bucket 的流控
- [ ] 认证与 API Key 管理
- [ ] Docker 镜像 (Distroless, < 20MB)

## Contributing

欢迎贡献代码！请确保：

1. 代码通过所有 lint 检查
2. 新功能有对应的测试用例
3. 注释使用英文，符合 GoDoc 规范
4. Commit message 遵循 [Conventional Commits](https://www.conventionalcommits.org/)

## License

MIT License
