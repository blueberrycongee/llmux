# Contributing to LLMux

Thank you for your interest in contributing to LLMux! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/llmux.git`
3. Create a branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `make test`
6. Commit your changes
7. Push and create a Pull Request

## Development Setup

```bash
# Install Go 1.23+
# https://go.dev/doc/install

# Clone and setup
git clone https://github.com/blueberrycongee/llmux.git
cd llmux
go mod download

# Run tests
make test

# Build
make build

# Run locally
export OPENAI_API_KEY=sk-xxx
./bin/llmux --config config/config.yaml
```

## Code Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing

### Comments

- All exported functions, types, and constants must have GoDoc comments
- Comments should be in English
- Explain "why", not "what"

```go
// Router dispatches requests to LLM providers based on model and load balancing strategy.
type Router struct {
    providers map[string]Provider
}

// Pick selects the best available deployment for the given model.
// Returns ErrNoDeployment if no deployment is available.
func (r *Router) Pick(ctx context.Context, model string) (*Deployment, error) {
    // ...
}
```

### Testing

- Write tests for all new functionality
- Use table-driven tests where appropriate
- Aim for >80% coverage on core logic
- Run `go test -race ./...` to check for race conditions

```go
func TestRouter_Pick(t *testing.T) {
    tests := []struct {
        name    string
        model   string
        want    string
        wantErr bool
    }{
        {"valid model", "gpt-4o", "openai", false},
        {"unknown model", "unknown", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add Anthropic provider support
fix: handle streaming timeout correctly
docs: update configuration examples
test: add router unit tests
refactor: simplify error handling
chore: update dependencies
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add tests for new functionality
4. Keep PRs focused and small
5. Respond to review feedback promptly

## Project Structure

```
llmux/
├── cmd/server/          # Application entry point
├── internal/            # Private application code
│   ├── api/             # HTTP handlers
│   ├── config/          # Configuration
│   ├── metrics/         # Prometheus metrics
│   ├── observability/   # Tracing, logging
│   ├── provider/        # LLM providers
│   ├── resilience/      # Circuit breaker, rate limiter
│   ├── router/          # Request routing
│   └── streaming/       # SSE streaming
├── pkg/                 # Public libraries
│   ├── types/           # Shared types
│   └── errors/          # Error definitions
├── deploy/              # Deployment files
└── config/              # Configuration examples
```

## Adding a New Provider

1. Create `internal/provider/newprovider/newprovider.go`
2. Implement the `Provider` interface
3. Add factory function to registry in `main.go`
4. Add tests in `internal/provider/newprovider/newprovider_test.go`
5. Update configuration documentation

## Questions?

Open an issue for questions or discussions.
