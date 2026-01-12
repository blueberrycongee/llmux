# LLMux Governance Gateway Roadmap

Document created: 2026-01-12T20:18:08Z
Last updated: 2026-01-12T20:18:08Z

## Goals
- Deliver an enterprise-grade governance gateway that supports both microservice and monolith modes.
- Keep the design pragmatic: high performance, ops-friendly, and not over-engineered.
- Ensure compatibility with key OpenAI-style surfaces and critical LiteLLM parameters.

## Execution Protocol
- Use atomic, TDD-driven tasks with explicit edge-case coverage.
- Run full lint and CI in WSL after each task, then self-review, commit, and push.
- Track completion with timestamps in this document as tasks are finished.

## Phases

### Phase 0 - Baseline and Interface Freeze
Status: not started
Completed at (UTC):

Deliverables:
- Define gateway spec, error codes, request lifecycle, and governance extension points.
- Publish the "distributed mode" runbook and minimal viable configuration templates.

### Phase 1 - Externalized State and Adapters
Status: not started
Completed at (UTC):

Deliverables:
- Abstract state interfaces: rate limits, budgets, RR and stats, sessions, audits.
- Provide Redis/Postgres adapters for distributed mode and in-memory adapters for monolith.
- Replace all local maps with interface-backed implementations.

### Phase 2 - Governance Kernel
Status: not started
Completed at (UTC):

Deliverables:
- Unify auth, tenant, budget, quota, and audit into a decision engine.
- PreHook intercept + PostHook accounting; async accounting with idempotent writes.
- Support hot reload of governance config.

### Phase 3 - Routing and Resilience
Status: not started
Completed at (UTC):

Deliverables:
- Standardize routing strategies (weighted, least-conn, latency-aware).
- Add jittered retries with caps and full timeouts; circuit-breaker and isolation pods.
- Make fallback policies explicit and observable.

### Phase 4 - Ops and Observability
Status: not started
Completed at (UTC):

Deliverables:
- Trace ID, structured logs, unified metrics, and circuit-break event metrics.
- Control plane APIs with audit logs, plus safe rollout/gray config.

### Phase 5 - Compatibility and Developer Experience
Status: not started
Completed at (UTC):

Deliverables:
- Align with LiteLLM key params and OpenAI-compatible surfaces
  (chat, responses, embeddings, audio, batch).
- Normalize config and CLI, and deliver high-quality docs and examples.

## Priority Refactor Items

### P0 - State Interfaces
Status: not started
Completed at (UTC):

- Move RR, rate limiting, budgets, stats, and sessions to external storage adapters.
- Provide local fallback for monolith mode.

### P0 - Unified Governance
Status: not started
Completed at (UTC):

- Consolidate "evaluate -> decide -> account" pipeline.
- Keep governance logic out of handlers and routers.

### P1 - Routing Strategies
Status: not started
Completed at (UTC):

- Add optional consistent RR (Redis counters), weighted least latency, and configurable
  multi-fallback with degradation.

### P1 - Resilience Policies
Status: not started
Completed at (UTC):

- Standardize timeouts, jittered exponential backoff, retry caps, circuit breaking,
  and isolation strategies.

### P1 - Observability and Audit
Status: not started
Completed at (UTC):

- Add trace IDs, structured logs, metrics, and audit/charge tracing.

### P2 - Compatibility
Status: not started
Completed at (UTC):

- Stabilize parameter mapping and align streaming vs non-streaming behavior.

## Selective References to Bifrost (No Direct Copy)
- Dual interception points (transport interceptor + pre/post hooks).
- Provider-level queueing and concurrency isolation.
- Jittered backoff with caps.
- Governance that can inject fallbacks pre-request.

## Task Log
| ID | Task | Status | Completed at (UTC) | Notes |
| --- | --- | --- | --- | --- |
| 000 | Roadmap document created | done | 2026-01-12T20:18:08Z | Initial version |
