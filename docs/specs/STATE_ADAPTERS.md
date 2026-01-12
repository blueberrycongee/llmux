# State Interfaces and Backends

Last updated (UTC): 2026-01-12T20:46:53Z

This document summarizes the stateful interfaces and their supported backends.

## Routing Stats
- Interface: `pkg/router.StatsStore`
- Backends: in-memory (`routers/MemoryStatsStore`), Redis (`routers/RedisStatsStore`)
- Purpose: latency, active requests, cooldowns, TPM/RPM usage

## Round-Robin Counters
- Interface: `pkg/router.RoundRobinStore`
- Backends: in-memory (`routers/MemoryRoundRobinStore`), Redis (`routers/RedisRoundRobinStore`)
- Purpose: consistent round-robin selection across instances

## Rate Limiting
- Interface: `internal/resilience.DistributedLimiter`
- Backends: Redis (`internal/resilience/RedisLimiter`)
- Local fallback: `internal/auth.TenantRateLimiter` for in-process token buckets

## Budgets, Usage, and Tenants
- Interface: `internal/auth.Store`
- Backends: in-memory (`internal/auth.MemoryStore`), Postgres (`internal/auth.PostgresStore`)
- Purpose: API keys, teams, users, orgs, budgets, usage logs

## Audit Logs
- Interface: `internal/auth.AuditLogStore`
- Backends: in-memory (`internal/auth.MemoryAuditLogStore`), Postgres (`internal/auth.PostgresAuditLogStore`)
- Purpose: compliance event logging and audit reporting

## Sessions
- Current status: no session state is stored by default.
- Guidance: use OIDC claims and `AuthContext` when session-like context is required.
