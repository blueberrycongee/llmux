# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat&logo=next.js)](https://nextjs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

English | [简体中文](README_CN.md)

**LLMux** is a high-performance LLM Gateway written in Go, featuring an enterprise-grade Web Dashboard built with Next.js. Route requests across multiple LLM providers with intelligent load balancing, unified API, and comprehensive management capabilities.

<p align="center">
  <img src="docs/architecture.png" alt="LLMux Architecture" width="700">
</p>

## 🚀 Performance: LLMux vs LiteLLM

We conducted a fair, head-to-head benchmark comparing LLMux (Go) against LiteLLM (Python).
Both gateways were tested on identical hardware (limited to 4 CPU cores) against a local Mock Server with fixed 50ms latency.

| Metric               | 🚀 LLMux (Go) | 🐢 LiteLLM (Python) | Difference             |
| :------------------- | :----------- | :----------------- | :--------------------- |
| **Throughput (RPS)** | **1943.35**  | **246.52**         | **~8x Faster**         |
| **Mean Latency**     | **51.29 ms** | **403.94 ms**      | **~8x Lower Overhead** |
| **P99 Latency**      | **91.71 ms** | **845.37 ms**      | **Stable vs Jittery**  |

*Benchmark Config: 10k requests, 100 concurrency, 4 CPU cores, 50ms backend latency.*

## ✨ Features

### Core Gateway
- **Unified OpenAI-Compatible API** - Single endpoint for all providers
- **Advanced Memory System** - Long-term memory with Mem0 architecture, smart ingestion, and hybrid retrieval
- **Multi-Provider Support** - OpenAI, Anthropic Claude, Google Gemini, Azure OpenAI, and any OpenAI-compatible API
- **6 Routing Strategies** - simple-shuffle, lowest-latency, least-busy, lowest-tpm-rpm, lowest-cost, tag-based
- **Streaming Support** - Real-time SSE streaming with proper forwarding
- **Response Caching** - In-memory, Redis, or dual-layer caching
- **Observability** - Prometheus metrics + OpenTelemetry tracing

### Enterprise Features
- **Multi-Tenant Authentication** - API keys, teams, users, organizations with hierarchical permissions
- **Budget Management** - Per-key, per-user, per-team budget limits with automatic reset
- **Rate Limiting** - TPM/RPM limits with model-level granularity
- **SSO/OIDC Integration** - Enterprise single sign-on with JWT team synchronization
- **Invitation System** - Self-service team/organization joining via invitation links
- **Audit Logging** - Complete audit trail for compliance

### Web Dashboard (New!)
- **Modern UI** - Built with Next.js 14, shadcn/ui, and Tremor charts
- **Real-time Analytics** - Request volume, token usage, cost tracking, model distribution
- **Resource Management** - Full CRUD for API keys, users, teams, and organizations
- **Responsive Design** - Works on desktop, tablet, and mobile

## 🏁 Quick Start

### Prerequisites

- Go 1.23+
- Node.js 18+ (for dashboard)
- (Optional) PostgreSQL for auth/usage tracking
- (Optional) Redis for distributed caching

### Build & Run

```bash
# Clone
git clone https://github.com/blueberrycongee/llmux.git
cd llmux

# Configure
cp .env.example .env
# Edit .env with your API keys

# Build Gateway
make build

# Run Gateway
./bin/llmux --config config/config.yaml
```

### Run Dashboard

```bash
cd ui

# Install dependencies
npm install

# Start development server
npm run dev

# Open http://localhost:3000
```

### Docker

```bash
# Build & run gateway
docker build -t llmux .
docker run -p 8080:8080 -v $(pwd)/config:/config llmux
```

## ⚙️ Configuration

### Environment Variables

```bash
# Provider API Keys
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx
AZURE_OPENAI_API_KEY=xxx

# Database (optional, enables enterprise features)
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis (optional, for distributed caching)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=xxx

# Dashboard
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
  strategy: simple-shuffle  # or: lowest-latency, least-busy, lowest-tpm-rpm, lowest-cost, tag-based
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

### OpenAI-Compatible Providers

LLMux works with any OpenAI-compatible API (SiliconFlow, Together AI, Groq, etc.):

```yaml
providers:
  - name: siliconflow
    type: openai
    api_key: ${SILICONFLOW_API_KEY}
    base_url: https://api.siliconflow.cn/v1
    models:
      - deepseek-ai/DeepSeek-V3
```

## 📡 API Reference

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

### List Models

```bash
curl http://localhost:8080/v1/models
```

### Health Check

```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

## 🔧 Management API

When database is enabled, full management endpoints are available:

### Key Management
| Endpoint        | Method | Description               |
| --------------- | ------ | ------------------------- |
| `/key/generate` | POST   | Generate API key          |
| `/key/update`   | POST   | Update API key            |
| `/key/delete`   | POST   | Delete API keys           |
| `/key/info`     | GET    | Get key info              |
| `/key/list`     | GET    | List keys with pagination |
| `/key/block`    | POST   | Block an API key          |
| `/key/unblock`  | POST   | Unblock an API key        |

### User Management
| Endpoint       | Method | Description                  |
| -------------- | ------ | ---------------------------- |
| `/user/new`    | POST   | Create user                  |
| `/user/update` | POST   | Update user                  |
| `/user/delete` | POST   | Delete users                 |
| `/user/info`   | GET    | Get user info                |
| `/user/list`   | GET    | List users (supports search) |

### Team & Organization
| Endpoint                | Method | Description               |
| ----------------------- | ------ | ------------------------- |
| `/team/new`             | POST   | Create team               |
| `/team/update`          | POST   | Update team               |
| `/team/member_add`      | POST   | Add member to team        |
| `/organization/new`     | POST   | Create organization       |
| `/organization/members` | GET    | List organization members |

### Analytics
| Endpoint               | Method | Description             |
| ---------------------- | ------ | ----------------------- |
| `/spend/logs`          | GET    | Get spend logs          |
| `/spend/keys`          | GET    | Spend by API keys       |
| `/spend/teams`         | GET    | Spend by teams          |
| `/global/activity`     | GET    | Global activity metrics |
| `/global/spend/models` | GET    | Spend by models         |
| `/audit/logs`          | GET    | Audit logs              |

## 🛤️ Routing Strategies

| Strategy         | Description                                                                 |
| ---------------- | --------------------------------------------------------------------------- |
| `simple-shuffle` | Random selection with optional weight/rpm/tpm weighting                     |
| `lowest-latency` | Select deployment with lowest average latency (supports TTFT for streaming) |
| `least-busy`     | Select deployment with fewest active requests                               |
| `lowest-tpm-rpm` | Select deployment with lowest TPM/RPM usage                                 |
| `lowest-cost`    | Select deployment with lowest cost per token                                |
| `tag-based`      | Filter deployments by request tags                                          |

## 🚢 Deployment

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

### Helm

```bash
helm install llmux deploy/helm/llmux
```

## 🛠️ Development

### Gateway (Go)

```bash
# Run tests
make test

# Run with coverage
make coverage

# Lint
make lint

# Format
make fmt

# All checks
make check
```

### Dashboard (Next.js)

```bash
cd ui

# Run tests
npm run test

# Run E2E tests
npm run test:e2e

# Lint
npm run lint
```

## 📚 Documentation

- **[Architecture Overview](.agent/docs/architecture/overview.md)**
- **[Advanced Memory System](internal/memory/README.md)**
- **[Plugin System](.agent/docs/architecture/plugin_system.md)**
- **[Developer Guide](.agent/docs/development/codebase_overview.md)**
- **[CI/CD Guide](.agent/docs/development/ci_guide.md)**
- **[Testing Guide](.agent/docs/development/testing.md)**

## 📁 Project Structure

```
llmux/
├── cmd/server/           # Gateway entry point
├── config/               # Configuration files
├── internal/
│   ├── api/              # HTTP handlers & management endpoints
│   ├── auth/             # Authentication, authorization & stores
│   ├── cache/            # Response caching (local/redis/dual)
│   ├── config/           # Configuration loading
│   ├── metrics/          # Prometheus & OpenTelemetry
│   └── router/           # Request routing strategies
├── providers/            # LLM provider adapters
│   ├── openai/
│   ├── anthropic/
│   ├── azure/
│   └── gemini/
├── pkg/
│   ├── types/            # Shared types
│   └── errors/           # Error definitions
├── ui/                   # Next.js Dashboard
│   ├── src/
│   │   ├── app/          # App Router pages
│   │   ├── components/   # React components
│   │   ├── hooks/        # Custom React hooks
│   │   ├── lib/          # API client & utilities
│   │   └── types/        # TypeScript types
│   └── e2e/              # Playwright E2E tests
├── deploy/               # Deployment configs
│   ├── k8s/
│   └── helm/
├── bench/                # Benchmark tools
└── tests/                # Integration tests
```

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

1. Fork the repository
2. Clone your fork
3. Create a feature branch
4. Make your changes
5. Run tests (`make check` for Go, `npm run test:all` for UI)
6. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE)

## 🙏 Acknowledgments

- Inspired by [LiteLLM](https://github.com/BerriAI/litellm) for the proxy pattern
- UI components from [shadcn/ui](https://ui.shadcn.com/)
- Charts powered by [Tremor](https://tremor.so/)
