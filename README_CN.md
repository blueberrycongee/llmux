# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat&logo=next.js)](https://nextjs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | 简体中文

LLMux 是一个使用 Go 编写的高性能 LLM 网关。它支持单机部署与分布式企业治理（可选 Postgres + Redis）。
当启用 UI 时，Next.js 控制台提供管理与分析能力。

## 概览

- 统一 OpenAI 兼容 API：chat、responses、embeddings、models
- 多 Provider 路由（shuffle、round-robin、lowest-latency、least-busy、lowest-tpm-rpm、lowest-cost）
- 治理能力：多租户认证、预算、限流、审计
- 运维友好：Prometheus 指标、OpenTelemetry、健康检查
- 可选 Next.js 控制台（管理 + 分析）

## 性能对比：LLMux vs LiteLLM

在相同硬件（4 CPU 核）上，使用固定 50ms 后端延迟的本地 Mock Server 对比：

| 指标               | LLMux (Go)  | LiteLLM (Python) | 差异                 |
| :----------------- | :---------- | :--------------- | :------------------- |
| **吞吐（RPS）**     | **1943.35** | **246.52**       | **约 8x 更快**       |
| **平均延迟**        | **51.29 ms**| **403.94 ms**    | **约 8x 更低开销**   |
| **P99 延迟**        | **91.71 ms**| **845.37 ms**    | **更稳定**           |

基准配置：10k 请求、100 并发、4 CPU 核、后端固定 50ms 延迟。

## 快速开始

### 依赖

- Go 1.23+
- Node.js 18+（控制台）
- 可选：PostgreSQL（认证/用量/审计）
- 可选：Redis（分布式路由统计/分布式限流）

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

# Database（可选，启用企业功能）
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis（可选，用于分布式缓存/路由/限流）
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=xxx

# Dashboard
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### config.yaml（最小示例）

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

- `standalone`：单机内存状态，适用于单实例运行。
- `distributed`：需要 Postgres（认证/用量）+ Redis（路由统计/限流等）。
- `development`：允许多实例的内存状态测试（不保证一致性）。

## 路由策略

| 策略             | 说明 |
| ---------------- | ---- |
| `simple-shuffle` | 随机选择，可结合权重/TPM/RPM |
| `round-robin`    | 轮询；分布式模式下可用 Redis 计数 |
| `lowest-latency` | 选择平均延迟最低的部署（支持流式 TTFT） |
| `least-busy`     | 选择当前活跃请求最少的部署 |
| `lowest-tpm-rpm` | 选择 TPM/RPM 使用最低的部署 |
| `lowest-cost`    | 选择 token 成本最低的部署 |

## API 参考

### Chat Completions

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}],
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

当数据库配置启用后，会开放完整的管理端点。

分类示例：
- Keys：`/key/*`
- Users：`/user/*`
- Teams：`/team/*`
- Organizations：`/organization/*`
- Spend/Usage：`/spend/*`、`/global/*`
- Audit：`/audit/*`
- Control：`/control/*`

## 运维与可观测性

- 指标：Prometheus（默认 `GET /metrics`）
- 追踪：OpenTelemetry（`tracing.*` 配置）
- 日志：结构化 JSON 或 text（`logging.*` 配置）
- 审计：当审计存储启用后写入审计日志

## 生产注意事项

- standalone 为单机内存状态。
- distributed 依赖 Postgres/Redis；缺失会影响对应能力（或按配置 fail-fast）。
- `/v1/audio/*` 与 `/v1/batches` 目前会返回 `invalid_request_error`（待补齐 provider 支持）。

## 项目结构

```
llmux/
|-- cmd/server/           # 网关入口
|-- config/               # 配置文件模板
|-- internal/
|   |-- api/              # HTTP handlers & 管理端点
|   |-- auth/             # 认证、授权与存储
|   |-- cache/            # 响应缓存（local/redis/dual）
|   |-- config/           # 配置加载
|   |-- metrics/          # Prometheus & OpenTelemetry
|   `-- router/           # 路由策略
|-- providers/            # Provider 适配器
|-- pkg/
|   |-- types/            # 共享类型
|   `-- errors/           # 错误定义
|-- ui/                   # Next.js 控制台
|-- deploy/               # 部署配置
|-- bench/                # 基准测试工具
`-- tests/                # 集成测试
```

## 开发命令

### 文档

- `docs/DEVELOPMENT.md`
- `docs/PRODUCTION_TEST_GUIDE.md`
- `docs/runbooks/DISTRIBUTED_MODE.md`

```bash
make test
make coverage
make lint
make fmt
make check
```

## 贡献

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## License

MIT License - 详见 [LICENSE](LICENSE)
