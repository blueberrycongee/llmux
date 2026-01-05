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
| Phase 6: 云原生 | ✅ 完成 | 2026-01-05 | Distroless 镜像, Helm Chart, CI/CD |
| Phase 7: 认证与多租户 | ✅ 完成 | 2026-01-06 | API Key 认证, PostgreSQL, 多租户限流 |
| Phase 8: 缓存层 | ✅ 完成 | 2026-01-06 | 内存缓存, Redis, 双层缓存 |

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
| Distroless 镜像 | ✅ | ✅ | 安全加固，< 20MB |
| Helm Chart | ✅ | ✅ | HPA, Ingress, Security |
| CI/CD | ✅ | ✅ | GitHub Actions |
| API Key 认证 | ✅ | ✅ | SHA-256 哈希存储 |
| 多租户隔离 | ✅ | ✅ | 按 Key/Team 限流 |
| PostgreSQL 集成 | ✅ | ✅ | 用户/团队/用量存储 |
| 预算控制 | ✅ | ✅ | Key/Team 级别预算 |
| 响应缓存 | ✅ | ✅ | 内存/Redis/双层缓存 |
| 缓存控制 | ✅ | ✅ | TTL, no-cache, no-store |
| 命名空间隔离 | ✅ | ✅ | 多租户缓存隔离 |

### 🔲 未实现（下一阶段）

| Phase | 功能 | 优先级 | 说明 |
|-------|------|--------|------|
| 9 | Token 计数 | 🟡 中 | tiktoken-go 估算 |
| 9 | 成本计算 | 🟡 中 | 按 model 计费 |
| 10 | 更多 Provider | 🟡 中 | Bedrock, Cohere, Ollama |
| 11 | Embeddings API | 🟡 中 | 向量嵌入 |
| 12 | Admin UI | 🟢 低 | 管理界面 |

### 📊 数据库选型

选择 PostgreSQL 作为持久化数据库，原因：
- 与 LiteLLM 使用相同数据库，便于迁移和对比
- 成熟稳定，生态丰富
- 支持 JSON 字段，适合存储灵活配置
- 支持事务，保证数据一致性

### 📊 完成度

```
核心网关功能:  ~85%
LiteLLM 全功能: ~25-30%
生产就绪度:    ~75%
```

### 🔜 下一步优先级

1. **认证系统** - API Key 验证（需要数据库）
2. **Token 计数** - tiktoken-go 估算成本
3. **缓存层** - Redis 缓存

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

## Phase 6: 云原生 ✅

### 已完成功能

1. **Distroless 镜像**
   - 多阶段构建
   - gcr.io/distroless/static-debian12:nonroot
   - 无 shell，安全加固
   - 非 root 用户运行

2. **Helm Chart**
   - Deployment, Service, ConfigMap
   - HPA 自动扩缩容
   - Ingress 配置
   - ServiceAccount
   - 安全上下文（只读文件系统，无特权）

3. **K8s Manifests**
   - 原生 YAML 示例
   - Secret 管理
   - 健康检查配置

4. **CI/CD**
   - GitHub Actions
   - 自动 lint、test、build
   - 多平台镜像构建 (amd64/arm64)
   - 自动发布 Release

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
├── deploy/                      # 部署文件
│   ├── helm/llmux/              # Helm Chart
│   └── k8s/                     # K8s Manifests
├── .github/workflows/           # CI/CD
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

### 本地运行

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

### Docker

```bash
docker run -d -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  ghcr.io/blueberrycongee/llmux:latest
```

### Kubernetes (Helm)

```bash
helm install llmux ./deploy/helm/llmux \
  --namespace llmux --create-namespace \
  --set providers[0].name=openai \
  --set providers[0].secretName=openai-credentials
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


---

## Phase 7: 认证与多租户 ✅

### 已完成功能

1. **API Key 认证**
   - SHA-256 哈希存储（不存明文）
   - Bearer Token 格式支持
   - Key 前缀显示（用于识别）
   - 过期时间检查
   - 预算限制检查

2. **多租户隔离**
   - Team/User 层级结构
   - 按 API Key 隔离限流
   - 按 Team 隔离预算
   - 用量统计持久化

3. **PostgreSQL 集成**
   - 完整的数据库 Schema
   - 连接池管理
   - 分区表（usage_logs 按月分区）
   - 自动更新 updated_at 触发器

4. **内存存储**
   - 用于测试和单实例部署
   - 完整实现 Store 接口
   - 线程安全

5. **多租户限流器**
   - 按租户隔离的 Token Bucket
   - 支持自定义速率
   - 自动清理不活跃租户
   - HTTP 中间件集成

### 数据库 Schema

```sql
-- 核心表
teams          -- 团队/组织
users          -- 用户
api_keys       -- API 密钥
usage_logs     -- 用量日志（按月分区）

-- 视图
api_key_usage_summary  -- API Key 用量汇总
team_usage_summary     -- Team 用量汇总
```

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/auth` | 60.9% |

### 配置示例

```yaml
# 启用认证
auth:
  enabled: true
  skip_paths:
    - /health/live
    - /health/ready
    - /metrics

# 启用数据库
database:
  enabled: true
  host: localhost
  port: 5432
  user: llmux
  password: ${DB_PASSWORD}
  database: llmux
```

### 文件结构

```
internal/auth/
├── types.go           # APIKey, Team, User, UsageLog 类型定义
├── hasher.go          # API Key 生成和哈希
├── store.go           # Store 接口定义
├── postgres.go        # PostgreSQL 实现
├── memory.go          # 内存存储实现
├── middleware.go      # HTTP 认证中间件
├── ratelimiter.go     # 多租户限流器
└── migrations/
    └── 001_init.sql   # 数据库迁移脚本
```


---

## Phase 8: 缓存层 ✅

### 已完成功能

1. **多后端缓存支持**
   - 内存缓存 (InMemoryCache)：LRU + TTL 混合淘汰
   - Redis 缓存 (RedisCache)：支持单节点/集群/Sentinel
   - 双层缓存 (DualCache)：内存 + Redis 两级缓存

2. **缓存键生成**
   - SHA-256 哈希确保唯一性
   - 支持命名空间隔离（多租户）
   - 基于请求参数（model, messages, temperature 等）

3. **缓存控制**
   - TTL 配置（默认/自定义）
   - no-cache：跳过缓存读取
   - no-store：跳过缓存写入
   - max-age：缓存有效期检查

4. **双层缓存策略**
   - 写入：同时写入内存和 Redis
   - 读取：先查内存，未命中再查 Redis
   - 回填：Redis 命中后自动回填内存
   - 节流：批量查询时限制 Redis 访问频率

5. **Redis 高级特性**
   - Pipeline 批量写入
   - MGET 批量读取
   - 原子递增 (INCRBY)
   - SetNX 分布式锁支持

### 设计参考

参考 LiteLLM 的缓存架构：
- `BaseCache` 抽象接口
- `DualCache` 双层缓存模式
- 缓存键哈希策略
- 批量操作优化

### 测试覆盖率

| 模块 | 覆盖率 |
|------|--------|
| `internal/cache` | 52.5% |

### 配置示例

```yaml
# 启用缓存
cache:
  enabled: true
  type: dual               # local, redis, dual
  namespace: llmux         # 命名空间隔离
  ttl: 1h                  # 默认 TTL

  # 内存缓存配置
  memory:
    max_size: 1000         # 最大条目数
    default_ttl: 10m       # 内存缓存 TTL
    max_item_size: 1048576 # 单条最大 1MB

  # Redis 配置
  redis:
    addr: localhost:6379
    password: ${REDIS_PASSWORD}
    db: 0
    pool_size: 10
```

### 文件结构

```
internal/cache/
├── types.go           # Cache 接口、CacheControl、CacheStats 类型
├── keygen.go          # 缓存键生成器（SHA-256）
├── memory.go          # 内存缓存（LRU + TTL）
├── redis.go           # Redis 缓存（支持集群/Sentinel）
├── dual.go            # 双层缓存（内存 + Redis）
├── handler.go         # 高级缓存处理器（业务层）
└── factory.go         # 缓存工厂函数
```

### 使用示例

```go
// 创建缓存
cfg := cache.Config{
    Enabled:   true,
    Type:      cache.CacheTypeDual,
    Namespace: "llmux",
    TTL:       time.Hour,
}
handler, _ := cache.NewCacheHandler(cfg)

// 获取缓存
cached, _ := handler.GetCachedResponse(ctx, req, nil)
if cached != nil {
    return cached.Response // 缓存命中
}

// 存储缓存
handler.SetCachedResponse(ctx, req, response, &cache.CacheControl{
    TTL:       30 * time.Minute,
    Namespace: "tenant-123",
})
```
