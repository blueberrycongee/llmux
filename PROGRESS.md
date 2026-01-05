# LLMux 开发进度

## 项目概述

LLMux 是一个用 Go 语言重构的 LiteLLM 核心功能实现，目标是打造生产级、高性能的 LLM 网关。

### 为什么重构？

LiteLLM (Python) 在高并发生产环境中存在以下问题：
- **GIL 限制**：300 RPS 时 P99 延迟急剧恶化
- **内存泄漏**：长时间运行后内存占用攀升至 12GB+
- **冷启动慢**：大量依赖导致启动时间长
- **部署复杂**：依赖 Python 环境和众多包

### Go 版本优势

- **高并发**：Goroutine 模型，轻松支持 1000+ RPS
- **低延迟**：P99 延迟降低 45 倍以上
- **单二进制**：无依赖部署，镜像 < 20MB
- **内存稳定**：无 GIL，无内存泄漏风险

---

## 开发进度

| Phase | 状态 | 完成日期 | 说明 |
|-------|------|----------|------|
| Phase 1: 骨架搭建 | ✅ 完成 | 2026-01-05 | HTTP Server, Config, Metrics, Router |
| Phase 2: 多 Provider | ✅ 完成 | 2026-01-05 | OpenAI, Anthropic, Azure, Gemini |
| Phase 3: SSE 流式 | ✅ 完成 | 2026-01-05 | 流式转发、buffer 复用、client 断开检测 |
| Phase 4: 高可用 | 🔲 待开始 | - | 熔断器、限流、优雅关闭 |
| Phase 5: 可观测性 | 🔲 待开始 | - | OpenTelemetry, 日志脱敏, Token 计数 |
| Phase 6: 云原生 | 🔲 待开始 | - | Distroless 镜像, Helm Chart |

---

## Phase 1: 骨架搭建 ✅

### 已完成功能

1. **HTTP Server**
   - 基于 `net/http` 的高性能服务器
   - 优雅关闭 (SIGTERM → drain → shutdown)
   - 健康检查端点 (`/health/live`, `/health/ready`)

2. **配置管理**
   - YAML 配置文件支持
   - 环境变量展开 (`${VAR_NAME}`)
   - fsnotify 热重载 (atomic.Pointer 原子替换)

3. **Prometheus Metrics**
   - `llmux_requests_total` - 请求计数
   - `llmux_request_latency_seconds` - 延迟分布
   - `llmux_token_usage_total` - Token 用量
   - `llmux_upstream_errors_total` - 错误统计

4. **路由器**
   - SimpleRouter 随机选择策略
   - 冷却机制 (429/401/408/5xx 触发)
   - 部署健康状态追踪

5. **OpenAI Provider**
   - 完整的请求/响应转换
   - 流式 chunk 解析
   - 错误映射

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/router` | 94.0% |
| `pkg/errors` | 93.8% |
| `internal/provider/openai` | 86.3% |
| `internal/streaming` | 81.6% |
| `internal/provider/azure` | 47.3% |
| `internal/provider/gemini` | 45.6% |
| `internal/provider/anthropic` | 38.9% |
| `internal/config` | 29.9% |

---

## Phase 2: 多 Provider 支持 ✅

### 已实现的 Provider

| Provider | 文件 | 功能 |
|----------|------|------|
| **OpenAI** | `internal/provider/openai/` | Chat Completions, Streaming, Tool Calling |
| **Anthropic** | `internal/provider/anthropic/` | Messages API, System Prompt, Tool Use |
| **Azure OpenAI** | `internal/provider/azure/` | Deployment-based routing, api-key auth |
| **Google Gemini** | `internal/provider/gemini/` | generateContent API, Function Calling |

### 参数映射

所有 Provider 都实现了 OpenAI 格式到原生格式的转换：

```
OpenAI Format (统一输入)
    ↓
Provider Adapter (转换)
    ↓
Native API Format (各厂商)
    ↓
Provider Adapter (转换)
    ↓
OpenAI Format (统一输出)
```

### 关键转换逻辑

| OpenAI 参数 | Anthropic | Gemini | Azure |
|-------------|-----------|--------|-------|
| `messages[role=system]` | `system` 字段 | `systemInstruction` | 直接透传 |
| `messages[role=tool]` | `tool_result` block | `functionResponse` | 直接透传 |
| `tool_choice=required` | `type: any` | `mode: ANY` | 直接透传 |
| `stop` | `stop_sequences` | `stopSequences` | 直接透传 |
| `finish_reason=stop` | `end_turn` | `STOP` | 直接透传 |

---

## Phase 3: SSE 流式支持 ✅

### 已完成功能

1. **SSE Forwarder**
   - 高效的流式转发器 (`internal/streaming/forwarder.go`)
   - `sync.Pool` buffer 复用，减少 GC 压力
   - Client 断开检测 (context cancellation)
   - 自动设置 SSE headers (Content-Type, Cache-Control, X-Accel-Buffering)

2. **Provider-Specific Parsers**
   - `OpenAIParser` - 标准 SSE 格式 (`data: {...}\n\n`)
   - `AnthropicParser` - 事件类型格式 (`event: xxx\ndata: {...}\n\n`)
   - `GeminiParser` - JSON 对象流格式
   - `AzureParser` - 复用 OpenAI 格式

3. **统一输出格式**
   - 所有 Provider 的流式输出都转换为 OpenAI 格式
   - 统一的 `StreamChunk` 结构
   - finish_reason 映射 (end_turn → stop, STOP → stop, etc.)

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/streaming` | 81.6% |

### 验收标准

- [x] SSE 正常工作
- [x] Client 断开能立即 cancel 上游
- [x] Buffer 复用 (sync.Pool)
- [x] 多 Provider 格式适配

---

## 下一步：Phase 4 - 高可用

## 下一步：Phase 4 - 高可用

### 目标

实现生产级高可用特性。

### 核心任务

1. **熔断器 (Circuit Breaker)**
   ```go
   // 使用 sony/gobreaker
   cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
       Name:        "openai",
       MaxRequests: 5,
       Interval:    10 * time.Second,
       Timeout:     30 * time.Second,
   })
   ```

2. **并发控制**
   - Per-provider semaphore
   - 防止单个 provider 过载

3. **限流 (Rate Limiting)**
   - Token Bucket 算法
   - Per-user / Per-API-key 限流

4. **优雅关闭增强**
   - Drain mode (停止接受新请求)
   - 等待现有请求完成
   - 超时强制关闭

### 验收标准

- [ ] 熔断器正常工作
- [ ] 并发控制生效
- [ ] 限流准确
- [ ] 优雅关闭无请求丢失

---

## Phase 4-6 预览

### Phase 4: 高可用

- 集成 `sony/gobreaker` 熔断器
- Per-provider semaphore 并发控制
- Token Bucket 限流
- 优雅关闭 (drain mode)

### Phase 5: 可观测性

- OpenTelemetry tracing
- 日志脱敏 (API Key, PII)
- tiktoken-go Token 估算
- 成本计算

### Phase 6: 云原生

- Distroless Docker 镜像 (< 20MB)
- Helm Chart
- GitHub Actions CI/CD

---

## 项目结构

```
llmux/
├── cmd/server/main.go           # 入口
├── internal/
│   ├── api/handler.go           # HTTP 处理器
│   ├── config/                  # 配置管理 + 热重载
│   ├── metrics/middleware.go    # Prometheus 指标
│   ├── provider/
│   │   ├── interface.go         # Provider 接口
│   │   ├── registry.go          # Provider 注册表
│   │   ├── openai/              # OpenAI 适配器
│   │   ├── anthropic/           # Anthropic 适配器
│   │   ├── azure/               # Azure OpenAI 适配器
│   │   └── gemini/              # Gemini 适配器
│   ├── router/
│   │   ├── interface.go         # Router 接口
│   │   └── simple.go            # 简单随机路由
│   └── streaming/
│       ├── forwarder.go         # SSE 流式转发器
│       └── parsers.go           # Provider 流式解析器
├── pkg/
│   ├── types/                   # 请求/响应类型
│   └── errors/                  # 统一错误类型
├── config/config.yaml           # 配置示例
├── Makefile                     # 构建命令
└── Dockerfile                   # 多阶段构建
```

---

## 快速开始

### 构建

```bash
make build
```

### 运行

```bash
export OPENAI_API_KEY=sk-xxx
./bin/llmux --config config/config.yaml
```

### 测试

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## 性能目标

| 指标 | 目标值 |
|------|--------|
| P99 延迟 | < 100ms |
| 吞吐量 | 1000 QPS |
| 内存占用 | < 100MB |
| 并发连接 | 10k |
| 冷启动 | < 1s |
| 镜像大小 | < 20MB |

---

## 参考资料

- [LiteLLM 源码](https://github.com/BerriAI/litellm)
- [开发路线图](./开发文档路线图.md)
- [深度分析](./DEEP_ANALYSIS.md)
- [代码分析](./LITELLM_CODE_ANALYSIS.md)
