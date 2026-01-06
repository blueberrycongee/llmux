# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md) | 简体中文

高性能 LLM 网关，使用 Go 编写。支持多提供商智能路由、统一 API 接口和企业级特性。

## 🚀 性能对比: LLMux vs LiteLLM

我们进行了一次公平的正面基准测试，对比了 LLMux (Go) 和 LiteLLM (Python)。
两个网关都在相同的硬件（限制为 4 个 CPU 核心）上针对具有固定 50ms 延迟的本地 Mock Server 进行了测试。

| 指标 | 🚀 LLMux (Go) | 🐢 LiteLLM (Python) | 差异 |
| :--- | :--- | :--- | :--- |
| **吞吐量 (RPS)** | **1943.35** | **246.52** | **~8 倍更快** |
| **平均延迟** | **51.29 ms** | **403.94 ms** | **~8 倍更低开销** |
| **P99 延迟** | **91.71 ms** | **845.37 ms** | **稳定 vs 抖动** |

*测试配置: 10k 请求, 100 并发, 4 CPU 核心, 50ms 后端延迟。*

## 特性

- **统一 OpenAI 兼容 API** - 单一端点访问所有提供商
- **多提供商支持** - OpenAI、Anthropic Claude、Google Gemini、Azure OpenAI
- **6 种路由策略** - 随机、最低延迟、最少请求、最低使用率、最低成本、标签路由
- **流式响应** - 实时 SSE 流式传输
- **响应缓存** - 内存、Redis 或双层缓存
- **可观测性** - Prometheus 指标 + OpenTelemetry 链路追踪
- **多租户认证** - API 密钥、团队、用户、组织及预算管理
- **速率限制** - 支持按密钥的 TPM/RPM 限制，可细化到模型级别
- **生产就绪** - Docker、Kubernetes、Helm 部署配置

## 快速开始

### 环境要求

- Go 1.23+
- (可选) PostgreSQL 用于认证/使用量追踪
- (可选) Redis 用于分布式缓存

### 构建与运行

```bash
# 克隆
git clone https://github.com/blueberrycongee/llmux.git
cd llmux

# 配置
cp .env.example .env
# 编辑 .env 填入你的 API 密钥

# 构建
make build

# 运行
./bin/llmux --config config/config.yaml
```

### Docker

```bash
docker build -t llmux .
docker run -p 8080:8080 -v $(pwd)/config:/config llmux
```

## 配置

### 环境变量

```bash
# 提供商 API 密钥
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx
AZURE_OPENAI_API_KEY=xxx

# 数据库 (可选)
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis (可选)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=xxx
```

### config.yaml

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 120s

providers:
  - name: openai
    type: openai
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    models:
      - gpt-4o
      - gpt-4o-mini

  - name: anthropic
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    models:
      - claude-3-5-sonnet-20241022

routing:
  strategy: simple-shuffle  # 可选: lowest-latency, least-busy, lowest-tpm-rpm, lowest-cost, tag-based
  fallback_enabled: true
  retry_count: 3

cache:
  enabled: true
  type: local  # local, redis, dual
  ttl: 1h

metrics:
  enabled: true
  path: /metrics

tracing:
  enabled: false
  endpoint: localhost:4317
```

### OpenAI 兼容提供商

LLMux 支持任何 OpenAI 兼容的 API（硅基流动、Together AI 等）：

```yaml
providers:
  - name: siliconflow
    type: openai
    api_key: ${SILICONFLOW_API_KEY}
    base_url: https://api.siliconflow.cn/v1
    models:
      - deepseek-ai/DeepSeek-V3
```

## API 参考

### 聊天补全

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好！"}],
    "stream": false
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

启用数据库后，可使用完整的管理端点：

| 端点 | 方法 | 描述 |
|------|------|------|
| `/key/generate` | POST | 生成 API 密钥 |
| `/key/update` | POST | 更新 API 密钥 |
| `/key/delete` | POST | 删除 API 密钥 |
| `/key/info` | GET | 获取密钥信息 |
| `/key/list` | GET | 列出密钥 |
| `/team/new` | POST | 创建团队 |
| `/team/update` | POST | 更新团队 |
| `/team/delete` | POST | 删除团队 |
| `/user/new` | POST | 创建用户 |
| `/organization/new` | POST | 创建组织 |
| `/spend/logs` | GET | 获取消费日志 |
| `/global/activity` | GET | 全局活动指标 |

## 路由策略

| 策略 | 描述 |
|------|------|
| `simple-shuffle` | 随机选择，支持权重/rpm/tpm 加权 |
| `lowest-latency` | 选择平均延迟最低的部署（流式请求支持 TTFT） |
| `least-busy` | 选择当前活跃请求数最少的部署 |
| `lowest-tpm-rpm` | 选择 TPM/RPM 使用率最低的部署 |
| `lowest-cost` | 选择每 token 成本最低的部署 |
| `tag-based` | 根据请求标签过滤部署 |

## 部署

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

### Helm

```bash
helm install llmux deploy/helm/llmux
```

## 开发

```bash
# 运行测试
make test

# 覆盖率测试
make coverage

# 代码检查
make lint

# 格式化
make fmt

# 全部检查
make check
```

## 项目结构

```
├── cmd/server/        # 入口
├── config/            # 配置文件
├── internal/
│   ├── api/           # HTTP 处理器
│   ├── auth/          # 认证与授权
│   ├── cache/         # 响应缓存
│   ├── provider/      # LLM 提供商适配器
│   │   ├── openai/
│   │   ├── anthropic/
│   │   ├── azure/
│   │   └── gemini/
│   └── router/        # 请求路由策略
├── pkg/
│   ├── types/         # 共享类型
│   └── errors/        # 错误定义
└── deploy/            # 部署配置
    ├── k8s/
    └── helm/
```

## 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 贡献

详见 [CONTRIBUTING.md](CONTRIBUTING.md)
