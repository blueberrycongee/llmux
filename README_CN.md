# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat&logo=next.js)](https://nextjs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | 简体中文

**LLMux** 是一个使用 Go 编写的高性能 LLM 网关，配备基于 Next.js 构建的企业级 Web 控制台。支持多提供商智能路由、统一 API 接口和全面的资源管理能力。

<p align="center">
  <img src="docs/architecture.png" alt="LLMux 架构" width="700">
</p>

## 🚀 性能对比: LLMux vs LiteLLM

我们进行了一次公平的正面基准测试，对比了 LLMux (Go) 和 LiteLLM (Python)。
两个网关都在相同的硬件（限制为 4 个 CPU 核心）上针对具有固定 50ms 延迟的本地 Mock Server 进行了测试。

| 指标             | 🚀 LLMux (Go) | 🐢 LiteLLM (Python) | 差异              |
| :--------------- | :----------- | :----------------- | :---------------- |
| **吞吐量 (RPS)** | **1943.35**  | **246.52**         | **~8 倍更快**     |
| **平均延迟**     | **51.29 ms** | **403.94 ms**      | **~8 倍更低开销** |
| **P99 延迟**     | **91.71 ms** | **845.37 ms**      | **稳定 vs 抖动**  |

*测试配置: 10k 请求, 100 并发, 4 CPU 核心, 50ms 后端延迟。*

## ✨ 特性

### 核心网关
- **统一 OpenAI 兼容 API** - 单一端点访问所有提供商
- **[高级记忆系统](internal/memory/README_CN.md)** - 基于 Mem0 架构的长期记忆，支持智能摄入和混合检索
- **多提供商支持** - OpenAI、Anthropic Claude、Google Gemini、Azure OpenAI 及任意 OpenAI 兼容 API
- **6 种路由策略** - 随机轮询、最低延迟、最少请求、最低使用率、最低成本、标签路由
- **流式响应** - 实时 SSE 流式传输，正确转发
- **响应缓存** - 内存、Redis 或双层缓存
- **可观测性** - Prometheus 指标 + OpenTelemetry 链路追踪

### 企业级功能
- **多租户认证** - API 密钥、团队、用户、组织的层级权限管理
- **预算管理** - 按密钥、用户、团队的预算限制，支持自动重置
- **速率限制** - TPM/RPM 限制，可细化到模型级别
- **SSO/OIDC 集成** - 企业单点登录及 JWT 团队同步
- **邀请系统** - 通过邀请链接自助加入团队/组织
- **审计日志** - 完整的操作审计轨迹，满足合规要求

### Web 控制台（新功能！）
- **现代化 UI** - 基于 Next.js 14、shadcn/ui 和 Tremor 图表构建
- **实时分析** - 请求量、Token 使用量、成本追踪、模型分布
- **资源管理** - API 密钥、用户、团队、组织的完整增删改查
- **响应式设计** - 适配桌面端、平板和移动端

## 🏁 快速开始

### 环境要求

- Go 1.23+
- Node.js 18+（控制台）
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

# 构建网关
make build

# 运行网关
./bin/llmux --config config/config.yaml
```

### 运行控制台

```bash
cd ui

# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 打开 http://localhost:3000
```

### Docker

```bash
# 构建并运行网关
docker build -t llmux .
docker run -p 8080:8080 -v $(pwd)/config:/config llmux
```

## ⚙️ 配置

### 环境变量

```bash
# 提供商 API 密钥
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx
AZURE_OPENAI_API_KEY=xxx

# 数据库 (可选，启用企业功能)
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis (可选，用于分布式缓存)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=xxx

# 控制台
NEXT_PUBLIC_API_URL=http://localhost:8080
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

LLMux 支持任何 OpenAI 兼容的 API（硅基流动、Together AI、Groq 等）：

```yaml
providers:
  - name: siliconflow
    type: openai
    api_key: ${SILICONFLOW_API_KEY}
    base_url: https://api.siliconflow.cn/v1
    models:
      - deepseek-ai/DeepSeek-V3
```

## 📡 API 参考

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

## 🔧 管理 API

启用数据库后，可使用完整的管理端点：

### 密钥管理
| 端点            | 方法 | 描述             |
| --------------- | ---- | ---------------- |
| `/key/generate` | POST | 生成 API 密钥    |
| `/key/update`   | POST | 更新 API 密钥    |
| `/key/delete`   | POST | 删除 API 密钥    |
| `/key/info`     | GET  | 获取密钥信息     |
| `/key/list`     | GET  | 列出密钥（分页） |
| `/key/block`    | POST | 封禁密钥         |
| `/key/unblock`  | POST | 解封密钥         |

### 用户管理
| 端点           | 方法 | 描述                 |
| -------------- | ---- | -------------------- |
| `/user/new`    | POST | 创建用户             |
| `/user/update` | POST | 更新用户             |
| `/user/delete` | POST | 删除用户             |
| `/user/info`   | GET  | 获取用户信息         |
| `/user/list`   | GET  | 列出用户（支持搜索） |

### 团队与组织
| 端点                    | 方法 | 描述         |
| ----------------------- | ---- | ------------ |
| `/team/new`             | POST | 创建团队     |
| `/team/update`          | POST | 更新团队     |
| `/team/member_add`      | POST | 添加团队成员 |
| `/organization/new`     | POST | 创建组织     |
| `/organization/members` | GET  | 列出组织成员 |

### 数据分析
| 端点                   | 方法 | 描述           |
| ---------------------- | ---- | -------------- |
| `/spend/logs`          | GET  | 获取消费日志   |
| `/spend/keys`          | GET  | 按密钥统计消费 |
| `/spend/teams`         | GET  | 按团队统计消费 |
| `/global/activity`     | GET  | 全局活动指标   |
| `/global/spend/models` | GET  | 按模型统计消费 |
| `/audit/logs`          | GET  | 审计日志       |

## 🛤️ 路由策略

| 策略             | 描述                                        |
| ---------------- | ------------------------------------------- |
| `simple-shuffle` | 随机选择，支持权重/rpm/tpm 加权             |
| `lowest-latency` | 选择平均延迟最低的部署（流式请求支持 TTFT） |
| `least-busy`     | 选择当前活跃请求数最少的部署                |
| `lowest-tpm-rpm` | 选择 TPM/RPM 使用率最低的部署               |
| `lowest-cost`    | 选择每 token 成本最低的部署                 |
| `tag-based`      | 根据请求标签过滤部署                        |

## 🚢 部署

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

### Helm

```bash
helm install llmux deploy/helm/llmux
```

## 🛠️ 开发

### 网关 (Go)

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

### 控制台 (Next.js)

```bash
cd ui

# 运行单元测试
npm run test

# 运行 E2E 测试
npm run test:e2e

# 代码检查
npm run lint
```

## 📁 项目结构

```
llmux/
├── cmd/server/           # 网关入口
├── config/               # 配置文件
├── internal/
│   ├── api/              # HTTP 处理器 & 管理端点
│   ├── auth/             # 认证、授权 & 存储层
│   ├── cache/            # 响应缓存 (本地/redis/双层)
│   ├── config/           # 配置加载
│   ├── metrics/          # Prometheus & OpenTelemetry
│   └── router/           # 请求路由策略
├── providers/            # LLM 提供商适配器
│   ├── openai/
│   ├── anthropic/
│   ├── azure/
│   └── gemini/
├── pkg/
│   ├── types/            # 共享类型
│   └── errors/           # 错误定义
├── ui/                   # Next.js 控制台
│   ├── src/
│   │   ├── app/          # App Router 页面
│   │   ├── components/   # React 组件
│   │   ├── hooks/        # 自定义 React Hooks
│   │   ├── lib/          # API 客户端 & 工具库
│   │   └── types/        # TypeScript 类型
│   └── e2e/              # Playwright E2E 测试
├── deploy/               # 部署配置
│   ├── k8s/
│   └── helm/
├── bench/                # 基准测试工具
└── tests/                # 集成测试
```

## 🤝 贡献

欢迎贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解贡献指南。

### 开发环境设置

1. Fork 本仓库
2. 克隆你的 Fork
3. 创建特性分支
4. 进行修改
5. 运行测试（Go: `make check`，UI: `npm run test:all`）
6. 提交 Pull Request

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 🙏 致谢

- 代理模式灵感来自 [LiteLLM](https://github.com/BerriAI/litellm)
- UI 组件来自 [shadcn/ui](https://ui.shadcn.com/)
- 图表由 [Tremor](https://tremor.so/) 提供支持
