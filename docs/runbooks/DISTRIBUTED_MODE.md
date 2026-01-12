# Distributed Mode Runbook

Last updated (UTC): 2026-01-12T21:33:13Z

## Purpose
This runbook enables multi-instance LLMux deployments with shared state via Postgres and Redis.

## Prerequisites
- PostgreSQL 14+ (for API keys, usage, and audit state).
- Redis 6+ (for routing stats and distributed rate limiting).
- Network access from LLMux instances to both Postgres and Redis.

## Bootstrap Checklist
1) Apply database schema.
2) Configure distributed mode and storage backends.
3) Validate health endpoints and metrics.

## Database Schema
LLMux ships SQL migrations in `internal/auth/migrations/`.

Apply the baseline schema (pick one file and keep it consistent across environments):

```
psql "$DATABASE_URL" -f internal/auth/migrations/001_init.sql
```

If you need the full multi-tenant schema parity, use:

```
psql "$DATABASE_URL" -f internal/auth/migrations/002_full_schema.sql
```

## Configuration Steps
1) Set `deployment.mode=distributed`.
2) Enable `database.enabled` with Postgres credentials.
3) Enable `routing.distributed` and `rate_limit.distributed` with Redis credentials.

Minimum templates:
- `docs/templates/config.distributed.min.yaml`
- `docs/templates/config.standalone.min.yaml`

## Startup
```
LLMUX_CONFIG=config/config.yaml ./llmux
```

## Smoke Checks
- `GET /health/ready` returns `{"status":"ok"}`.
- `GET /metrics` exposes metrics without errors.
- Logs include successful Redis and Postgres initialization.

## Operational Notes
- If Redis is unavailable, routing stats fall back to local stats.
- Round-robin counters use Redis when distributed routing is enabled; fallback is local.
- Governance idempotency uses Redis in distributed mode when configured; otherwise it falls back to memory.
- Governance config hot reload is supported for runtime policy changes.
- If Postgres is unavailable and auth is enabled, startup will fail in distributed mode.
- Use `server.admin_port` if you need a separate admin plane port.

## Rollback
Switch `deployment.mode` to `standalone` and disable distributed storage settings,
then redeploy.
