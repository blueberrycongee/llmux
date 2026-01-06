# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

English | [简体中文](README_CN.md)

A high-performance LLM Gateway written in Go. Route requests across multiple LLM providers with intelligent load balancing, unified API, and enterprise-grade features.

## Features

- **Unified OpenAI-Compatible API** - Single endpoint for all providers
- **Multi-Provider Support** - OpenAI, Anthropic Claude, Google Gemini, Azure OpenAI
- **6 Routing Strategies** - simple-shuffle, lowest-latency, least-busy, lowest-tpm-rpm, lowest-cost, tag-based
- **Streaming Support** - Real-time SSE streaming with proper forwarding
- **Response Caching** - In-memory, Redis, or dual-layer caching
- **Observability** - Prometheus metrics + OpenTelemetry tracing
- **Multi-Tenant Auth** - API keys, teams, users, organizations with budgets
- **Rate Limiting** - Per-key TPM/RPM limits with model-level granularity
- **Production Ready** - Docker, Kubernetes, Helm deployment configs

## Quick Start

### Prerequisites

- Go 1.23+
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

# Build
make build

# Run
./bin/llmux --config config/config.yaml
```

### Docker

```bash
docker build -t llmux .
docker run -p 8080:8080 -v $(pwd)/config:/config llmux
```

## Configuration

### Environment Variables

```bash
# Provider API Keys
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-ant-xxx
GOOGLE_API_KEY=xxx
AZURE_OPENAI_API_KEY=xxx

# Database (optional)
DB_HOST=localhost
DB_USER=llmux
DB_PASSWORD=xxx
DB_NAME=llmux

# Redis (optional)
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

LLMux works with any OpenAI-compatible API (SiliconFlow, Together AI, etc.):

```yaml
providers:
  - name: siliconflow
    type: openai
    api_key: ${SILICONFLOW_API_KEY}
    base_url: https://api.siliconflow.cn/v1
    models:
      - deepseek-ai/DeepSeek-V3
```

## API Reference

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

## Management API

When database is enabled, full management endpoints are available:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/key/generate` | POST | Generate API key |
| `/key/update` | POST | Update API key |
| `/key/delete` | POST | Delete API keys |
| `/key/info` | GET | Get key info |
| `/key/list` | GET | List keys |
| `/team/new` | POST | Create team |
| `/team/update` | POST | Update team |
| `/team/delete` | POST | Delete team |
| `/user/new` | POST | Create user |
| `/organization/new` | POST | Create organization |
| `/spend/logs` | GET | Get spend logs |
| `/global/activity` | GET | Global activity metrics |

## Routing Strategies

| Strategy | Description |
|----------|-------------|
| `simple-shuffle` | Random selection with optional weight/rpm/tpm weighting |
| `lowest-latency` | Select deployment with lowest average latency (supports TTFT for streaming) |
| `least-busy` | Select deployment with fewest active requests |
| `lowest-tpm-rpm` | Select deployment with lowest TPM/RPM usage |
| `lowest-cost` | Select deployment with lowest cost per token |
| `tag-based` | Filter deployments by request tags |

## Deployment

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

### Helm

```bash
helm install llmux deploy/helm/llmux
```

## Development

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

## Project Structure

```
├── cmd/server/        # Entry point
├── config/            # Configuration files
├── internal/
│   ├── api/           # HTTP handlers
│   ├── auth/          # Authentication & authorization
│   ├── cache/         # Response caching
│   ├── provider/      # LLM provider adapters
│   │   ├── openai/
│   │   ├── anthropic/
│   │   ├── azure/
│   │   └── gemini/
│   └── router/        # Request routing strategies
├── pkg/
│   ├── types/         # Shared types
│   └── errors/        # Error definitions
└── deploy/            # Deployment configs
    ├── k8s/
    └── helm/
```

## License

MIT License - see [LICENSE](LICENSE)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)
