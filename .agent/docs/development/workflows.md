# LLMux Development Workflows

Common tasks and how to accomplish them.

---

## 🔧 Local Development Setup

### Start Backend + Frontend

```bash
# Terminal 1: Backend
cd llmux
./llmux.exe --config config/config.dev.yaml

# Terminal 2: Frontend
cd llmux/ui
npm run dev
```

**Access:**
- Gateway API: http://localhost:8080
- Dashboard: http://localhost:3000

---

## ✅ CI/Lint Workflow

### Full CI Check (Backend)

```bash
# One command to rule them all
gofmt -w . && go vet ./... && golangci-lint run ./... && go test ./... -v && go build -o llmux.exe ./cmd/server
```

### Individual Steps

| Step   | Command                              | Purpose          |
| ------ | ------------------------------------ | ---------------- |
| Format | `gofmt -w .`                         | Fix code style   |
| Vet    | `go vet ./...`                       | Static analysis  |
| Lint   | `golangci-lint run ./...`            | Advanced linting |
| Test   | `go test ./... -v`                   | Run all tests    |
| Build  | `go build -o llmux.exe ./cmd/server` | Compile          |

### Frontend CI

```bash
cd ui
npm run lint     # ESLint
npm run test     # Vitest unit tests
npm run build    # Production build
```

---

## 🐛 Fixing Common Lint Errors

### govet: shadow
Variable shadowing - rename the inner variable:
```go
// Bad
err := doSomething()
if err := doAnother(); err != nil { } // shadows outer err

// Good
err := doSomething()
if err2 := doAnother(); err2 != nil { }
```

### errcheck
Always handle errors:
```go
// Bad
doSomething()

// Good
if err := doSomething(); err != nil {
    return err
}
// Or explicitly ignore
_ = doSomething()
```

---

## 🧪 Testing Workflow

### Run Backend Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/auth/... -v

# E2E tests
go test ./tests/e2e/... -v
```

### Run Frontend Tests

```bash
cd ui

# Unit tests
npm run test

# E2E tests (requires Playwright)
npm run test:e2e
```

### Create Test Data via API

```bash
# Use the test script
powershell -ExecutionPolicy Bypass -File scripts/test_api.ps1
```

---

## 📝 Adding Features

### Add New Backend Endpoint

1. **Create handler** in `internal/api/xxx_endpoints.go`:
```go
func (h *ManagementHandler) NewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

2. **Register route** in the handler's `RegisterRoutes`:
```go
func (h *ManagementHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("POST /new/endpoint", h.NewEndpoint)
}
```

3. **Test** with curl or add E2E test

### Add New Frontend Page

1. **Create page**: `ui/src/app/(dashboard)/new-page/page.tsx`
2. **Create hook**: `ui/src/hooks/use-new-data.ts`
3. **Add API method**: `ui/src/lib/api/client.ts`
4. **Add nav link**: `ui/src/components/dashboard-layout.tsx`

---

## 🚀 Deployment Workflow

### Build Production Binary

```bash
# With embedded UI
cd ui && npm run build
# Copy ui/out/* to cmd/server/ui_assets/
go build -o llmux ./cmd/server
```

### Docker Build

```bash
docker build -t llmux:latest .
```

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

---

## 🔑 Config Management

### Development Config
- File: `config/config.dev.yaml`
- Features: Auth disabled, in-memory store

### Production Config
- File: `config/config.yaml`
- Features: Auth enabled, PostgreSQL, Redis cache

### Environment Variables
- `OPENAI_API_KEY` - OpenAI API key
- `ANTHROPIC_API_KEY` - Anthropic API key
- `DB_HOST`, `DB_USER`, `DB_PASSWORD` - Database
- `REDIS_ADDR` - Redis address

---

## 📊 Monitoring

### Prometheus Metrics
```bash
curl http://localhost:8080/metrics
```

### Health Checks
```bash
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

---

## 🆘 Troubleshooting

### Dashboard shows empty data
1. Check backend is running: `curl http://localhost:8080/health/live`
2. Check CORS: Look for CORS errors in browser console
3. Create test data: Run `scripts/test_api.ps1`

### Tests fail with "ui_assets not found"
```bash
# Create placeholder
echo "" > cmd/server/ui_assets/.gitkeep
```

### Lint fails with shadow errors
Rename the shadowed variable to a unique name.
