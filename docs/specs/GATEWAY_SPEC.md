# LLMux Gateway Specification (Baseline)

Version: v0
Last updated (UTC): 2026-01-12T20:26:44Z

## Scope
This spec defines the gateway contract for request lifecycle, error semantics, and governance
extension points. It is a baseline for both standalone and distributed deployments.

## Supported HTTP Surfaces (current)
- POST `/v1/chat/completions`
- POST `/v1/completions`
- POST `/v1/embeddings`
- GET `/v1/models`
- GET `/health/ready`
- GET `/health/live`
- GET `/metrics`

## Error Response Contract
All errors return JSON in an OpenAI-compatible envelope:

```
{
  "error": {
    "message": "human-readable string",
    "type": "rate_limit_error",
    "code": "optional_machine_code"
  }
}
```

### Error Type Registry
Type values are stable and map to HTTP status codes:
- `authentication_error` -> 401
- `rate_limit_error` -> 429
- `invalid_request_error` -> 400
- `not_found_error` -> 404
- `timeout_error` -> 408
- `service_unavailable_error` -> 503
- `internal_error` -> 500
- `context_length_exceeded` -> 400
- `content_policy_violation` -> 400

### Optional Error Code
`code` is optional and reserved for machine-level classification. If not set, it is
omitted or null. Preferred values are lowercase snake_case and stable across versions.

Reserved baseline codes:
- `rate_limit_exceeded`
- `budget_exceeded`
- `provider_unavailable`
- `provider_timeout`
- `invalid_payload`
- `model_not_allowed`
- `stream_unsupported`

## Request Lifecycle
The gateway follows a deterministic lifecycle for both streaming and non-streaming requests.

1) **Ingress**
   - Enforce max body size.
   - Parse JSON; reject invalid payloads.

2) **Validation**
   - Validate required fields and semantic constraints.
   - Normalize request fields for provider compatibility.

3) **Governance Evaluate**
   - Auth (API keys, OIDC) and tenant lookup.
   - Rate limit and budget evaluation.
   - Policy evaluation (model allowlist, org/team constraints).

4) **Route Selection**
   - Select deployment based on routing strategy and request context.
   - Provide provider + deployment metadata for accounting and observability.

5) **Upstream Execution**
   - Build provider request with per-deployment timeout.
   - Send request via shared HTTP client.

6) **Response Handling**
   - If streaming, forward SSE chunks with provider-specific parsing.
   - If non-streaming, parse provider response and map to OpenAI format.

7) **Governance Account**
   - Record usage, spend, and audit log.
   - Apply async accounting where possible with idempotent writes.

8) **Observability**
   - Emit metrics, logs, and traces for pre/post events and failures.

## Governance Extension Points
Extension points are designed to keep governance logic out of handlers and routers.

### Pre-Request Hooks
Run before routing. Use for validation, auth, budgeting, and policy gates.

### Route Hooks
Executed during routing decisions to allow governance to influence deployment selection.

### Post-Response Hooks
Run after response or stream completion. Use for accounting and audit writes.

### Stream Hooks
Optional per-chunk hooks for stream governance or observability. Should be lightweight.

### Transport Interceptors
Optional hooks to wrap outbound requests and inbound responses for tracing, retry control,
or response shaping.

## Compatibility Guarantees
- Streaming and non-streaming behaviors are consistent for the same request.
- Error types are stable across providers; provider-specific errors are normalized.
- Distributed mode does not change API semantics, only state backends and routing behavior.

## Non-Goals (v0)
- This spec does not mandate a single storage backend.
- This spec does not define UI or control plane APIs.
- This spec does not redefine OpenAI semantics beyond compatibility needs.
