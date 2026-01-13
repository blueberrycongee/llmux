# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat&logo=next.js)](https://nextjs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | 简体中文

LLMux 是一个使用 Go 编写的高性能 LLM 网关。支持单体与分布式两种模式，
分布式治理需要 Postgres + Redis。可选 Next.js 控制台提供管理与分析能力。



## 概览

- 统一 OpenAI 兼容 API：chat、responses、embeddings、models
- 多提供商路由，六种策略（shuffle、round-robin、lowest-latency、
  least-busy、lowest-tpm-rpm、lowest-cost）
- 治理能力：多租户认证、预算、限流、审计
- 运维友好：Prometheus 指标、OpenTelemetry 追踪、健康检查
- 可选 Next.js 控制台用于管理与分析

## 性能对比：LLMux vs LiteLLM

在相同硬件（4 CPU 核）上，使用固定 50ms 延迟的本地 Mock Server 进行对比。

| 指标               | LLMux (Go)  | LiteLLM (Python) | 差异                 |
| :----------------- | :---------- | :--------------- | :------------------- |
| **吞吐量 (RPS)**   | **1943.35** | **246.52**       | **约 8x 更快**       |
| **平均延迟**       | **51.29 ms**| **403.94 ms**    | **约 8x 更低开销**   |
| **P99 延迟**       | **91.71 ms**| **845.37 ms**    | **更稳定、抖动更小** |

测试配置：10k 请求、100 并发、4 CPU 核、50ms 后端延迟。

## 快速开始

### 依赖

- Go 1.23+
- Node.js 18+（控制台）
- 可选：PostgreSQL（认证/用量）
- 可选：Redis（分布式路由与限流）

### 构建与运行

```bash
git clone https://github.com/blueberrycongee/llmux.git
cd llmux

cp .env.example .env
# 编辑 .env 填入 API 密钥

make build
cp config/config.example.yaml config/config.yaml
./bin/llmux --config config/config.yaml
```

### 启动控制台

```bash
cd ui
npm install
npm run dev
```

### Docker

```bash
docker build -t llmux .
docker run -p 8080:8080 -v $(pwd)/config:/config llmux
```

## 配置

### 环境变量

```bash
# Provider API Keys
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx
AZURE_OPENAI_API_KEY=xxx

# 数据库（可选，启用企业功能）
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis（可选，分布式缓存/路由/限流）
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=xxx

# 控制台
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### config.yaml（最小）

```yaml
server:
  port: 8080
  admin_port: 0
  read_timeout: 30s
  write_timeout: 120s

deployment:
  mode: standalone  # standalone, distributed, development

providers:
  - name: openai
    type: openai
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    models:
      - gpt-4o
      - gpt-4o-mini

routing:
  strategy: simple-shuffle
  fallback_enabled: true
  retry_count: 3
  distributed: false

metrics:
  enabled: true
  path: /metrics
```

### 部署模式

- `standalone`：单机内存态。
- `distributed`：需要 Postgres + Redis。
- `development`：多实例测试用（不保证一致性）。

## 路由策略

| 策略             | 说明                                                                 |
| ---------------- | -------------------------------------------------------------------- |
| `simple-shuffle` | 随机选择，可结合权重/TPM/RPM                                          |
| `round-robin`    | 轮询，分布式模式下可使用 Redis 计数                                  |
| `lowest-latency` | 选择平均延迟最低的部署（支持流式 TTFT）                              |
| `least-busy`     | 选择当前活跃请求最少的部署                                           |
| `lowest-tpm-rpm` | 选择 TPM/RPM 最低的部署                                              |
| `lowest-cost`    | 选择 token 成本最低的部署                                            |

## API 参考

### Chat Completions

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }'
```

### Responses

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4o",
    "input": "Hello!"
  }'
```

### 列出模型

```bash
curl http://localhost:8080/v1/models
```

### 健康检查

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

## 管理 API

启用数据库后可使用完整的管理端点。

分类示例：
- Keys：`/key/*`
- Users：`/user/*`
- Teams：`/team/*`
- Organizations：`/organization/*`
- Spend/Usage：`/spend/*`、`/global/*`
- Audit：`/audit/*`
- Control：`/control/*`

## 运维与观测

- 指标：Prometheus `metrics.path`（默认 `/metrics`）
- 追踪：OpenTelemetry（`tracing.*` 配置）
- 日志：结构化 JSON 日志
- 审计：审计存储启用时写入审计日志

## 生产注意事项

- standalone 为单机内存态。
- distributed 依赖 Postgres/Redis，缺失将影响相关功能。
- `/v1/audio/*` 与 `/v1/batches` 暂返回 `invalid_request_error`。

## 项目结构

```
llmux/
|-- cmd/server/           # 网关入口
|-- config/               # 配置文件
|-- internal/
|   |-- api/              # HTTP 处理器 & 管理端点
|   |-- auth/             # 认证、授权与存储
|   |-- cache/            # 缓存
|   |-- config/           # 配置加载
|   |-- metrics/          # Prometheus & OpenTelemetry
|   `-- router/           # 路由策略
|-- providers/            # Provider 适配
|-- pkg/
|   |-- types/            # 共享类型
|   `-- errors/           # 错误定义
|-- ui/                   # Next.js 控制台
|-- deploy/               # 部署配置
|-- bench/                # 基准测试
`-- tests/                # 集成测试
```

## 开发者信息

### 文档

- [Architecture Overview](.agent/docs/architecture/overview.md)
- [Plugin System](.agent/docs/architecture/plugin_system.md)
- [Developer Guide](.agent/docs/development/codebase_overview.md)
- [CI/CD Guide](.agent/docs/development/ci_guide.md)
- [Testing Guide](.agent/docs/development/testing.md)

### 开发命令

```bash
make test
make coverage
make lint
make fmt
make check
```

### 贡献

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

### License

MIT License - 见 [LICENSE](LICENSE)
