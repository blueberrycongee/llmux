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
| Phase 4: 高可用 | ✅ 完成 | 2026-01-05 | 熔断器、限流、并发控制 |
| Phase 5: 可观测性 | ✅ 完成 | 2026-01-05 | OpenTelemetry, 日志脱敏, Request ID |
| Phase 6: 云原生 | 🔲 待开始 | - | Distroless 镜像, Helm Chart |

---

## 与 LiteLLM 功能对比

### ✅ 已实现

| 功能 | LiteLLM | LLMux | 说明 |
|------|---------|-------|------|
| OpenAI 适配 | ✅ | ✅ | Chat Completions |
| Anthropic 适配 | ✅ | ✅ | Messages API |
| Azure OpenAI 适配 | ✅ | ✅ | Deployment routing |
| Gemini 适配 | ✅ | ✅ | generateContent |
| Tool Calling | ✅ | ✅ | 跨 Provider 转换 |
| SSE 流式 | ✅ | ✅ | Buffer 复用 |
| 配置热重载 | ✅ | ✅ | fsnotify |
| Prometheus 指标 | ✅ | ✅ | 请求/延迟/Token |
| 熔断器 | ✅ | ✅ | 自研实现 |
| 限流 | ✅ | ✅ | Token Bucket |
| 并发控制 | ✅ | ✅ | Semaphore |
| OpenTelemetry | ✅ | ✅ | OTLP gRPC 导出 |
| 日志脱敏 | ✅ | ✅ | API Key/PII 自动 mask |
| Request ID | ✅ | ✅ | 请求关联追踪 |

### 🔲 未实现

| 功能 | 优先级 | 说明 |
|------|--------|------|
| 认证系统 | 高 | API Key 验证 |
| 缓存层 | 高 | Redis 缓存 |
| Token 计数 | 中 | tiktoken-go 估算 |
| 数据库持久化 | 中 | PostgreSQL（与 LiteLLM 一致） |
| 预算管理 | 中 | 按用户/团队限额 |
| 更多 Provider | 中 | Bedrock, Cohere 等 |
| Embeddings API | 中 | 向量嵌入 |
| Admin UI | 低 | 管理界面 |

### 📊 数据库选型

选择 PostgreSQL 作为持久化数据库，原因：
- 与 LiteLLM 使用相同数据库，便于迁移和对比
- 成熟稳定，生态丰富
- 支持 JSON 字段，适合存储灵活配置
- 支持事务，保证数据一致性

### 📊 完成度

```
核心网关功能:  ~80%
LiteLLM 全功能: ~20-25%
生产就绪度:    ~60%
```

### 🔜 下一步优先级

1. **Phase 6: 云原生** - Distroless 镜像, Helm Chart
2. **认证系统** - API Key 验证（需要数据库）
3. **Token 计数** - tiktoken-go 估算成本

---

## Phase 1: 骨架搭建 ✅

### 已完成功能

1. **HTTP Server** - 基于 `net/http`，优雅关闭
2. **配置管理** - YAML + 环境变量 + 热重载
3. **Prometheus Metrics** - 请求/延迟/Token/错误
4. **路由器** - 随机选择 + 冷却机制
5. **OpenAI Provider** - 请求/响应转换

---

## Phase 2: 多 Provider 支持 ✅

| Provider | 功能 |
|----------|------|
| OpenAI | Chat, Streaming, Tools |
| Anthropic | Messages API, System, Tools |
| Azure | Deployment routing |
| Gemini | generateContent, Functions |

---

## Phase 3: SSE 流式支持 ✅

- SSE Forwarder + `sync.Pool` buffer 复用
- Client 断开检测 (context cancellation)
- 多 Provider 格式适配 (OpenAI/Anthropic/Gemini)

---

## Phase 4: 高可用 ✅

### 已完成功能

1. **熔断器 (Circuit Breaker)**
   - 三态: Closed → Open → Half-Open
   - 可配置阈值和超时
   - 状态变更回调

2. **限流器 (Rate Limiter)**
   - Token Bucket 算法
   - 支持突发流量
   - 动态调整速率

3. **并发控制 (Semaphore)**
   - Context-aware 阻塞
   - 超时取消支持
   - 公平唤醒

4. **统一管理器 (Manager)**
   - 按 Provider/Deployment 隔离
   - 懒加载组件
   - 统计信息查询

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/resilience` | 97.6% |

---

## Phase 5: 可观测性 ✅

### 已完成功能

1. **OpenTelemetry Tracing**
   - OTLP gRPC 导出器
   - LLM 专用 Span 属性 (gen_ai.*)
   - 可配置采样率
   - Jaeger/Tempo/Zipkin 兼容

2. **日志脱敏 (Redactor)**
   - API Key 自动 mask (OpenAI/Anthropic/Google)
   - Bearer Token 脱敏
   - 邮箱、电话、信用卡、SSN 等 PII 保护
   - HTTP Header 敏感字段过滤

3. **Request ID**
   - 自动生成唯一请求 ID
   - 支持传入 X-Request-ID 头
   - Context 传递，日志关联

4. **结构化日志**
   - slog 封装，支持 JSON/Text 格式
   - 自动脱敏输出
   - Request ID 自动注入

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/observability` | 82.0% |

---

## 下一步：Phase 6 - 云原生

### 目标

实现生产级部署能力。

### 核心任务

1. **Distroless 镜像**
   - 多阶段构建
   - 镜像 < 20MB
   - 无 shell，安全加固

2. **Helm Chart**
   - Deployment, Service, ConfigMap
   - HPA 自动扩缩容
   - Ingress 配置

3. **CI/CD**
   - GitHub Actions
   - 自动测试、构建、推送

---

## 项目结构

```
llmux/
├── cmd/server/main.go           # 入口
├── internal/
│   ├── api/handler.go           # HTTP 处理器
│   ├── config/                  # 配置管理 + 热重载
│   ├── metrics/middleware.go    # Prometheus 指标
│   ├── provider/                # Provider 适配器
│   │   ├── openai/
│   │   ├── anthropic/
│   │   ├── azure/
│   │   └── gemini/
│   ├── resilience/              # 高可用组件
│   │   ├── circuitbreaker.go
│   │   ├── ratelimiter.go
│   │   ├── semaphore.go
│   │   └── manager.go
│   ├── observability/           # 可观测性
│   │   ├── tracing.go
│   │   ├── redact.go
│   │   ├── requestid.go
│   │   └── logger.go
│   ├── router/                  # 路由器
│   └── streaming/               # SSE 流式
├── pkg/
│   ├── types/                   # 请求/响应类型
│   └── errors/                  # 统一错误类型
├── config/config.yaml           # 配置示例
├── Makefile
└── Dockerfile
```

---

## 测试覆盖率汇总

| 模块 | 覆盖率 |
|------|--------|
| `internal/resilience` | 97.6% |
| `internal/router` | 94.0% |
| `pkg/errors` | 93.8% |
| `internal/provider/openai` | 86.3% |
| `internal/observability` | 82.0% |
| `internal/streaming` | 81.6% |
| `internal/provider/azure` | 47.3% |
| `internal/provider/gemini` | 44.8% |
| `internal/config` | 38.0% |
| `internal/provider/anthropic` | 38.3% |

---

## 快速开始

```bash
# 构建
make build

# 运行
export OPENAI_API_KEY=sk-xxx
./bin/llmux --config config/config.yaml

# 测试
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello!"}]}'
```

---

## 性能目标

| 指标 | 目标值 |
|------|--------|
| P99 延迟 | < 100ms |
| 吞吐量 | 1000 QPS |
| 内存占用 | < 100MB |
| 冷启动 | < 1s |
| 镜像大小 | < 20MB |
