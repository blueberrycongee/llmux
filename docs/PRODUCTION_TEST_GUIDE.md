# Production-Like Test Guide (Local)

This guide provides a practical, staged way to validate LLMux end-to-end.

## Levels

1. **Level 1: Single instance** (no external deps)
   - Focus: API compatibility, provider wiring, streaming, metrics
2. **Level 2: Single instance + Postgres + Redis** (docker compose)
   - Focus: distributed routing stats, rate limiting backends, governance stores
3. **Level 3: Multi-instance + load balancer**
   - Focus: HA behavior, distributed routing accuracy under load

## Stage 1: Single instance (fast sanity)

```bash
go build -o llmux ./cmd/server

export OPENAI_API_KEY="sk-your-key"
./llmux --config config/config.test.yaml
```

Smoke checks (another terminal):

```bash
curl -sf http://localhost:8080/health/ready >/dev/null
curl -sf http://localhost:8080/metrics | head

curl -sf http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}' | head
```

Notes:
- `config/config.test.yaml` keeps `auth/database/governance` disabled to avoid extra moving parts.

## Stage 2: docker compose (Postgres + Redis)

### 1) Start services

```bash
export OPENAI_API_KEY="sk-your-key"
docker compose -f docker-compose.test.yaml up -d
```

### 2) Apply migrations + seed a test API key

LLMux ships canonical Postgres migrations in `internal/auth/migrations/`.
This test setup applies them and then inserts a single API key:

- Test key: `sk-test-key-12345`
- Test key sha256: `db4aac95519f2890e4bd9f8860cceb0b452d35c5e809d69d44f234de3e7123d0`

```bash
docker compose -f docker-compose.test.yaml exec -T postgres \
  psql -U llmux -d llmux -f /workspace/scripts/init_db.sql
```

### 3) Run the API test suite

```bash
chmod +x scripts/test_production.sh scripts/test_redis.sh scripts/test_postgres.sh

export BASE_URL="http://localhost:8080"
export TEST_API_KEY="sk-test-key-12345"
./scripts/test_production.sh

# Optional: validate infra from your host (requires local clients)
export DB_HOST=localhost DB_PORT=5432 DB_USER=llmux DB_NAME=llmux DB_PASSWORD=llmux_test_pwd
./scripts/test_postgres.sh

export REDIS_HOST=localhost REDIS_PORT=6379
./scripts/test_redis.sh
```

Notes:
- Stage 2 uses `config/config.distributed.yaml` which enables `auth`, `database`, `routing.distributed`, and `rate_limit.distributed`.
- Because `auth.enabled=true`, requests to `/v1/*` require `Authorization: Bearer <key>`.
 - `governance` is disabled in the default distributed test config to avoid schema drift while the Postgres store is converging.

## Stage 3: Multi-instance (HA / LB)

The current `docker-compose.test.yaml` binds `llmux` to host port `8080`, so it cannot be scaled directly (port conflict).

To test multi-instance:
1) Remove the `ports:` mapping from the `llmux` service.
2) Add a load balancer (nginx/traefik) that exposes a single host port and proxies to the `llmux` service replicas.
3) Run:

```bash
docker compose -f docker-compose.test.yaml up -d --scale llmux=3
```

## Monitoring (optional)

Start Prometheus/Grafana profiles:

```bash
docker compose -f docker-compose.test.yaml --profile monitoring up -d
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)
