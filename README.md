# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?style=flat&logo=next.js)](https://nextjs.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

English | [Simplified Chinese](README_CN.md)

LLMux is a high-performance LLM gateway written in Go. It supports standalone
deployments and distributed, enterprise-grade governance with optional Postgres
and Redis. A Next.js dashboard provides management and analytics when enabled.



## Overview

- Unified OpenAI-compatible APIs: chat, responses, embeddings, models
- Multi-provider routing with six strategies (shuffle, round-robin, lowest-latency,
  least-busy, lowest-tpm-rpm, lowest-cost)
- Governance: multi-tenant auth, budgets, rate limits, audit logging
- Ops-friendly: Prometheus metrics, OpenTelemetry tracing, health checks
- Optional Next.js dashboard for management and analytics

## Performance: LLMux vs LiteLLM

We benchmarked LLMux (Go) against LiteLLM (Python) on identical hardware (4 CPU
cores) using a local mock server with fixed 50ms latency.

| Metric               | LLMux (Go)  | LiteLLM (Python) | Difference             |
| :------------------- | :---------- | :--------------- | :--------------------- |
| **Throughput (RPS)** | **1943.35** | **246.52**       | **~8x Faster**         |
| **Mean Latency**     | **51.29 ms**| **403.94 ms**    | **~8x Lower Overhead** |
| **P99 Latency**      | **91.71 ms**| **845.37 ms**    | **Stable vs Jittery**  |

Benchmark config: 10k requests, 100 concurrency, 4 CPU cores, 50ms backend latency.

## Quick Start

### Prerequisites

- Go 1.23+
- Node.js 18+ (dashboard)
- Optional: PostgreSQL for auth/usage tracking
- Optional: Redis for distributed routing + rate limiting

### Build and Run

```bash
git clone https://github.com/blueberrycongee/llmux.git
cd llmux

cp .env.example .env
# Edit .env with your API keys

make build
cp config/config.example.yaml config/config.yaml
./bin/llmux --config config/config.yaml
```

### Run Dashboard

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

## Configuration

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

### config.yaml (minimal)

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

### Deployment Modes

- `standalone`: in-memory state, intended for single-instance runs.
- `distributed`: requires PostgreSQL for auth/usage state and Redis for routing
  stats and rate limiting.
- `development`: allows in-memory state for multi-instance testing (not consistent).

## Routing Strategies

| Strategy         | Description                                                                 |
| ---------------- | --------------------------------------------------------------------------- |
| `simple-shuffle` | Random selection with optional weight/rpm/tpm weighting                     |
| `round-robin`    | Cycles through deployments, Redis-backed when distributed                   |
| `lowest-latency` | Selects deployment with lowest average latency (streaming-aware)            |
| `least-busy`     | Selects deployment with fewest active requests                              |
| `lowest-tpm-rpm` | Selects deployment with lowest TPM/RPM usage                                |
| `lowest-cost`    | Selects deployment with lowest cost per token                               |

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

Management endpoints are enabled when the database is configured.

Key categories:
- Keys: `/key/*`
- Users: `/user/*`
- Teams: `/team/*`
- Organizations: `/organization/*`
- Spend/usage: `/spend/*`, `/global/*`
- Audit: `/audit/*`
- Control: `/control/*`

## Operations and Observability

- Metrics: Prometheus at `metrics.path` (default `/metrics`)
- Tracing: OpenTelemetry exporter configuration via `tracing.*` settings
- Logs: structured JSON logs from the gateway and management APIs
- Auditing: append-only audit logs when the audit store is configured

## Production Notes

- Standalone mode is single-node and uses in-memory state only.
- Distributed mode requires Postgres for auth/usage and Redis for routing stats
  and rate limiting; missing dependencies degrade related features.
- `/v1/audio/*` and `/v1/batches` currently return `invalid_request_error` until
  provider support is implemented.

## Project Structure

```
llmux/
|-- cmd/server/           # Gateway entry point
|-- config/               # Configuration files
|-- internal/
|   |-- api/              # HTTP handlers & management endpoints
|   |-- auth/             # Authentication, authorization & stores
|   |-- cache/            # Response caching (local/redis/dual)
|   |-- config/           # Configuration loading
|   |-- metrics/          # Prometheus & OpenTelemetry
|   `-- router/           # Request routing strategies
|-- providers/            # LLM provider adapters
|-- pkg/
|   |-- types/            # Shared types
|   `-- errors/           # Error definitions
|-- ui/                   # Next.js Dashboard
|-- deploy/               # Deployment configs
|-- bench/                # Benchmark tools
`-- tests/                # Integration tests
```

## Developer Info

### Documentation

- `docs/DEVELOPMENT.md`
- `docs/PRODUCTION_TEST_GUIDE.md`
- `docs/runbooks/DISTRIBUTED_MODE.md`

### Development Commands

```bash
make test
make coverage
make lint
make fmt
make check
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

### License

MIT License - see [LICENSE](LICENSE)
