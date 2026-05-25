# LLMux Core Theory, Goals, and Experimental Data

## Purpose

This document extracts the project's core positioning, theoretical framework,
engineering goals, and currently documented experimental data from existing
repository materials. It is intended as a compact project-level summary rather
than a replacement for the original specs or thesis draft.

## Source Documents

- `README.md`
- `docs/ROADMAP.md`
- `docs/specs/GATEWAY_SPEC.md`
- `docs/specs/STATE_ADAPTERS.md`
- `routers/README.md`
- `bench/README.md`
- `docs/AGENT_HETEROGENEOUS_GATEWAY_THESIS_PAPER.md`

## 1. Project Positioning

LLMux has two closely related layers of positioning.

### 1.1 Base Product Positioning

At the base layer, LLMux is a high-performance LLM gateway written in Go. Its
core purpose is to pull model access, routing, governance, and observability up
into a unified platform layer instead of scattering those concerns across
business applications.

The gateway's stable product goals are:

- Provide OpenAI-compatible APIs such as chat, responses, embeddings, and models.
- Route traffic across multiple providers with unified strategy control.
- Support governance features such as auth, budgets, rate limits, and audit logs.
- Support both standalone deployment and distributed deployment.
- Provide an operations-friendly control plane with metrics, tracing, health
  checks, and dashboard visibility.

### 1.2 Research / Evolutionary Positioning

At the research layer, the project is evolving from a "multi-provider model
gateway" into a "heterogeneous Agent-oriented routing hub".

The thesis draft defines the extended problem as a multi-layer decision system:

- Team-level routing: decide whether a task should be handled by an Expert Agent Team.
- Agent-level routing: decide which lead Agent should own execution.
- Model-level scheduling: decide which candidate model should execute under
  runtime load and constraint signals.
- Tool-augmented execution: inject real MCP tools so Agents can perform external actions.

This means the project goal is no longer only "which provider should serve this
request", but increasingly "how should a semantically complex task be routed
across teams, agents, tools, and models under dynamic constraints".

## 2. Core Engineering Goals

According to `docs/ROADMAP.md`, the gateway's core engineering goals are:

- Deliver an enterprise-grade governance gateway that supports both microservice
  and monolith modes.
- Keep the design pragmatic: high performance, ops-friendly, and not
  over-engineered.
- Ensure compatibility with key OpenAI-style surfaces and critical LiteLLM parameters.

The roadmap also shows the implementation priorities that define the project's
core direction:

- Externalized state and adapters.
- Unified governance kernel.
- Routing and resilience standardization.
- Observability and control plane support.
- Compatibility and developer experience.

## 3. Core Theory and Principles

## 3.1 Gateway as a Platform Control Layer

The project treats the LLM gateway not as a thin proxy but as a platform control
layer. In this model, the gateway is responsible for:

- Protocol normalization.
- Governance evaluation.
- Route selection.
- Upstream execution.
- Usage accounting.
- Observability.

The request lifecycle defined in `docs/specs/GATEWAY_SPEC.md` is:

1. Ingress.
2. Validation.
3. Governance Evaluate.
4. Route Selection.
5. Upstream Execution.
6. Response Handling.
7. Governance Account.
8. Observability.

This lifecycle is a core project theory: handlers should not embed fragmented
policy logic; the gateway should provide a deterministic, reusable execution
pipeline.

## 3.2 Governance-Kernel Theory

The roadmap explicitly pushes governance out of handlers and routers into a
unified kernel. In practical terms, governance is modeled as:

- Pre-request evaluation: auth, tenant resolution, rate limit, budget, policy.
- Post-request accounting: usage logging, spend updates, audit logging.
- Async accounting with idempotent writes where appropriate.
- Runtime config update support.

This reflects the project's view that "governance" is not an add-on, but a
first-class execution stage.

## 3.3 State Externalization Theory

A key architectural principle is that stateful capabilities should be abstracted
 behind interfaces and moved out of process when needed.

The project documents the following state split:

- Routing stats: in-memory or Redis.
- Round-robin counters: in-memory or Redis.
- Rate limiting: local fallback or Redis-backed distributed limiter.
- Budgets / tenants / usage: in-memory or Postgres.
- Audit logs: in-memory or Postgres.

The theory behind this is that distributed deployment should not change API
semantics. It should only swap state backends and routing behavior.

## 3.4 Adaptive Routing Theory

At the gateway layer, LLMux uses runtime feedback rather than fixed dispatch.
The router README highlights three core ideas:

- Multiple routing strategies, including lowest latency, least busy, lowest
  TPM/RPM, and lowest cost.
- EWMA-based runtime estimation for latency, TTFT, and success rate.
- Dynamic weighting in latency routing:
  `Weight = BaseWeight * (SuccessRate^2) / Latency`

The project theory here is that routing should react to recent provider quality,
not just static configuration. This is why latency-aware routing uses EWMA and
success-rate penalties instead of simple averages or fixed weights.

## 3.5 Agent-Oriented Three-Layer Routing Theory

The thesis draft extends the gateway from provider routing to three-layer task
routing:

- Layer 1: Team routing with Team Registry + Team Memory.
- Layer 2: Agent routing with Agent Registry + Route Memory + tool bindings.
- Layer 3: Candidate-model scheduling with runtime RPM/TPM, cooldown, and
  predicted saturation.

The main theoretical additions are:

- Route Memory: historical path and outcome records become reusable routing knowledge.
- Team Memory: historical team collaboration becomes reusable team-level routing knowledge.
- Tool-Augmented Expert Agent: Agents are enhanced with real MCP-bound tools,
  not only prompt-defined roles.
- Predictive scheduling: avoid high-risk candidates before sending traffic,
  instead of only failing over after an error.

## 3.6 Optimization Targets in the Research Model

The thesis draft formalizes four optimization targets:

- Maximize request success rate.
- Minimize rate-limit collision probability.
- Minimize average end-to-end latency.
- Maximize model resource utilization under safety constraints.

This shows the project's extended scheduling theory is not only semantic; it is
explicitly framed as a constrained optimization problem across success,
stability, latency, and utilization.

## 4. Core Research Questions

The thesis draft defines four core research questions:

- RQ1: How to improve Agent routing rationality and consistency with request
  semantics and Route Memory?
- RQ2: How to introduce Expert Agent Teams and reuse Team Memory for complex tasks?
- RQ3: How to proactively schedule among multiple candidate models and tools
  under runtime constraints instead of only reacting after failure?
- RQ4: How to integrate Team routing, Agent routing, candidate-model scheduling,
  tool-augmented execution, and multi-tenant governance into one gateway prototype?

These RQs are the clearest statement of the project's "upper-layer" research goal.

## 5. Existing Experimental Data

The repository currently contains two different categories of experiment output:

- Quantitative benchmark data for the base gateway.
- Functional / prototype validation results for the Agent-routing architecture.

They should not be mixed together.

## 5.1 Quantitative Gateway Benchmark

The README documents a benchmark comparison between LLMux and LiteLLM on the
same hardware.

### Benchmark Setup

- Hardware: 4 CPU cores.
- Backend condition: local mock server with fixed 50 ms latency.
- Total requests: 10,000.
- Concurrency: 100.

### Reported Results

| Metric | LLMux (Go) | LiteLLM (Python) | Interpretation |
| --- | ---: | ---: | --- |
| Throughput (RPS) | 1943.35 | 246.52 | About 8x higher throughput |
| Mean latency | 51.29 ms | 403.94 ms | About 8x lower overhead |
| P99 latency | 91.71 ms | 845.37 ms | Much more stable tail latency |

### Extracted Conclusion

The documented benchmark supports the claim that the Go-based gateway has
significantly lower framework overhead than the Python comparison baseline under
the tested mock environment.

### Related Benchmark Metrics Supported by the Tooling

According to `bench/README.md`, the benchmark framework is designed to measure:

- RPS
- Latency P50
- Latency P95
- Latency P99
- Memory
- Errors

However, the main README currently only publishes RPS, mean latency, and P99
latency for the LLMux vs LiteLLM comparison.

## 5.2 Prototype / Functional Validation Results

The thesis draft does not present large-scale statistical experiment tables for
the Agent-routing subsystem. Instead, it reports functional verification of the
prototype.

### Validated Capabilities

The documented results claim the system can:

- Return `selected_team`, `team_participants`, `selected_agent`,
  `selected_candidate`, `candidate_failovers`, and `candidate_states` in the
  conversation interface.
- Return `participating_agents`, `steps`, and `recorded_memory` in the team
  rehearsal interface.
- Record `consulted` / `reused` Route Memory and Team Memory.
- Compute `selection_score` from recent usage and predicted pressure in the
  candidate-model layer.
- Skip high-risk `predicted_saturated` candidates before execution.
- Inject MCP tools so Expert Agents have real external execution capability.

### Extracted Conclusion

The project currently demonstrates feasibility and initial effectiveness of the
three-layer routing idea, but the evidence is still prototype-oriented rather
than a mature, statistically validated evaluation.

## 5.3 Current Evidence Boundary

The thesis draft is explicit about current limitations of the evidence:

- Validation is mainly prototype and functional testing.
- Candidate-model usage is still mainly local-memory based.
- The current scheduling score focuses on weight, RPM/TPM pressure, and cooldown.
- Latency, cost, and success-rate factors are not yet fully integrated into the
  candidate-model score.
- Team collaboration is still a lightweight mode, not a full autonomous
  parallel multi-agent execution framework.

This means the project already has a solid method prototype, but not yet a
fully quantified research evaluation for the higher-level Agent routing thesis.

## 6. Current Project Maturity Assessment

Based on the extracted materials, the project can be understood as follows:

- The "enterprise LLM gateway" layer is already relatively mature in concept:
  compatibility, governance, distributed state, routing, resilience, and
  observability all have concrete specs and roadmap completion records.
- The "Agent-oriented heterogeneous routing" layer is more innovative and is
  already implemented as a working prototype, but its evaluation is still in the
  functional-verification stage.

In other words, the repository already contains both:

- A practical gateway platform direction.
- A research-oriented intelligent routing direction.

## 7. Key Extracted Conclusions

- The fundamental goal of LLMux is to turn model access into governed,
  observable, routable infrastructure.
- The core engineering theory is a deterministic gateway pipeline:
  validate -> govern -> route -> execute -> account -> observe.
- The core routing theory is adaptive scheduling based on recent runtime state,
  not fixed dispatch.
- The project's major research extension is a three-layer routing architecture:
  Team -> Agent -> Candidate Model.
- The currently published quantitative evidence is strongest for gateway
  performance against LiteLLM.
- The currently published Agent-routing evidence is strongest as a functional
  prototype validation, not yet as large-scale quantitative research evidence.

## 8. Recommended Use of This Summary

This summary is suitable for:

- Project briefings.
- Thesis / report drafting.
- Architecture onboarding.
- Explaining the distinction between the gateway baseline and the Agent-routing extension.

For implementation details, read the original source documents listed at the top
of this file.
