# LLMux

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/blueberrycongee/llmux/actions/workflows/ci.yaml/badge.svg)](https://github.com/blueberrycongee/llmux/actions)

High-performance LLM Gateway written in Go. A production-ready alternative to LiteLLM with better performance, lower resource usage, and cloud-native design.

## Why LLMux?

LiteLLM (Python) has known issues in high-concurrency production environments:

| Issue | LiteLLM (Python) | LLMux (Go) |
|-------|------------------|------------|
| GIL Bottleneck | P99 latency spikes at 300 RPS | Handles 1000+ RPS smoothly |
| Memory Leaks | 12GB+ after long runs | Stable ~100MB |
| Cold Start | Slow due to dependencies | < 1 second |
| Deployment | Complex Python environment | Single binary, < 20MB image |

## Features

- **Multi-Provider Support** - OpenAI, Anthropic, Azure OpenAI, Google Gemini
- **OpenAI-Compatible API** - Drop-in replacement for OpenAI SDK
- **SSE Streaming** - Efficient streaming with buffer pooling
- **High Availability** - Circuit breaker, rate limiting, concurrency control
- **Observability** - OpenTelemetry tracing, Prometheus metrics, log redaction
- **Cloud-Native** - Distroless image, Helm chart, HPA support

## Quick Start

### Binary

```bash
# Build
make build

# Run
export OPENAI_API_KEY=sk-xxx
./bin/llmux --config config/config.yaml
```

### Docker

```bash
docker run -d -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  ghcr.io/blueberrycongee/llmux:latest
```

### Kubernetes (Helm)

```bash
# Create secret
kubectl create secret generic openai-credentials \
  --from-literal=api-key=sk-xxx -n llmux

# Install
helm install llmux ./deploy/helm/llmux \
  --namespace llmux --create-namespace
```

## Usage

LLMux is fully compatible with OpenAI SDK:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

## Configuration

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
    models: [gpt-4o, gpt-4o-mini]
    timeout: 60s

  - name: anthropic
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    models: [claude-3-5-sonnet-20241022]

routing:
  strategy: simple-shuffle
  fallback_enabled: true
  retry_count: 3

metrics:
  enabled: true
  path: /metrics

tracing:
  enabled: false
  endpoint: localhost:4317
```

See [config/config.yaml](config/config.yaml) for full configuration options.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        Client                            │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                      LLMux Gateway                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Metrics  │  │ Tracing  │  │ Redactor │  │ ReqID   │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Circuit  │  │  Rate    │  │ Semaphore│              │
│  │ Breaker  │  │ Limiter  │  │          │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          ▼                ▼                ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │  OpenAI  │    │ Anthropic│    │  Gemini  │
    └──────────┘    └──────────┘    └──────────┘
```

## Project Structure

```
llmux/
├── cmd/server/              # Entry point
├── internal/
│   ├── api/                 # HTTP handlers
│   ├── config/              # Configuration management
│   ├── metrics/             # Prometheus metrics
│   ├── observability/       # Tracing, logging, redaction
│   ├── provider/            # LLM provider adapters
│   ├── resilience/          # Circuit breaker, rate limiter
│   ├── router/              # Request routing
│   └── streaming/           # SSE streaming
├── pkg/
│   ├── types/               # Request/response types
│   └── errors/              # Error definitions
├── deploy/
│   ├── helm/                # Helm chart
│   └── k8s/                 # Kubernetes manifests
└── config/                  # Configuration examples
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions (OpenAI compatible) |
| `/v1/models` | GET | List available models |
| `/health/live` | GET | Liveness probe |
| `/health/ready` | GET | Readiness probe |
| `/metrics` | GET | Prometheus metrics |

## Performance

| Metric | Target |
|--------|--------|
| P99 Latency | < 100ms (gateway overhead) |
| Throughput | 1000+ QPS |
| Memory | < 100MB |
| Cold Start | < 1s |
| Image Size | < 20MB |

## Development

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run with coverage
make test-coverage

# Lint
make lint

# Build
make build
```

## Roadmap

- [x] Multi-provider support (OpenAI, Anthropic, Azure, Gemini)
- [x] SSE streaming with buffer pooling
- [x] Circuit breaker, rate limiter, semaphore
- [x] OpenTelemetry tracing
- [x] Log redaction (API keys, PII)
- [x] Helm chart and CI/CD
- [ ] Authentication system
- [ ] Redis caching
- [ ] Token counting (tiktoken)
- [ ] Budget management
- [ ] Admin UI

## Contributing

Contributions are welcome! Please ensure:

1. Code passes `golangci-lint`
2. Tests are included for new features
3. Comments follow GoDoc conventions
4. Commits follow [Conventional Commits](https://www.conventionalcommits.org/)

## License

MIT License - see [LICENSE](LICENSE) for details.
