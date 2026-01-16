# Bug Fixes and Design Analysis Log

This document records the issues identified during the recent deep audit and distinguishes between "Design Trade-offs" and "Implementation Vulnerabilities."

## 🛠️ Implementation Vulnerabilities (Bugs & Flaws)
These are coding errors that, if left unfixed, could cause service crashes or resource leaks in production.

### 1. APIKey Concurrency Safety (Data Race)
- **Nature**: **Implementation Vulnerability**.
- **Issue**: `MemoryStore` returned shallow copies of structs, allowing multiple goroutines to concurrently read/write map fields (e.g., `ModelSpend`). This violates Go's concurrency safety and causes unpredictable crashes.
- **Fix**: Introduced a `Clone()` deep copy mechanism.
- **Risk**: 🚨 High

### 2. Streaming Context Leak
- **Nature**: **Implementation Vulnerability**.
- **Issue**: The `Forward()` method did not ensure the `cancel()` function was called, leading to lingering child contexts and memory/resource leaks after requests ended.
- **Fix**: Added `defer cancel()`.
- **Risk**: ⚠️ Medium

### 3. Missing Global Panic Recovery
- **Nature**: **Implementation Vulnerability / Stability Omission**.
- **Issue**: Lack of global panic recovery middleware meant that exceptions would hang requests without structured responses or complete logging.
- **Fix**: Added `recoveryMiddleware` at the entry point.
- **Risk**: ⚠️ Medium

## 📐 Design Trade-offs
These were initial choices made for development efficiency, performance, or simplicity, which have now been upgraded to match leading open-source standards.

### 1. From Hardcoded Auth to Casbin (RBAC Evolution)
- **Before**: Hardcoded role checks in middleware. **Pros**: Simple logic, rapid development. **Cons**: Requires recompilation for policy changes; lack of dynamism.
- **After**: Integrated **Casbin**. **Trade-off**: Slightly higher learning curve and configuration complexity, but gained industrial-grade dynamic access control.

### 2. From Static Load Balancing to EWMA Routing
- **Before**: Simple Round-Robin or static weights. **Pros**: Minimal CPU overhead, rock-solid logic. **Cons**: Incapable of sensing real-time performance jitters of upstream providers.
- **After**: Integrated **EWMA (Exponentially Weighted Moving Average)**. **Trade-off**: Incurred small memory overhead for statistics, but gained significant adaptive steering away from unstable providers.

### 3. From Top-1 Semantic Retrieval to Top-N Re-ranking
- **Before**: Retrieve only the single best vector match. **Pros**: Fastest performance, lowest latency. **Cons**: Prone to "hallucinated hits" where prompts are semantically similar but detail-divergent.
- **After**: Integrated **Top-N + Re-ranking**. **Trade-off**: Added time overhead for secondary filtering, but greatly improved cache accuracy and reliability.

### 4. From Fixed Rate Limiting to Adaptive Limiting
- **Before**: TPM/RPM limits based on fixed preset values. **Pros**: Fully predictable behavior. **Cons**: Unable to adjust throughput based on real-time back-end pressure.
- **After**: Integrated **Adaptive Limiter**. **Trade-off**: More complex logic, but achieved true system self-protection.

---
**Verification Status**: All fixes passed regression testing with the `-race` flag enabled.
