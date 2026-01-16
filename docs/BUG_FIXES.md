# Bug Fix Log

This document records the core bugs identified and fixed during the recent deep audit of the LLMux project.

## 1. APIKey Concurrency Safety (Data Race)
- **Description**: In `internal/auth/memory.go`, the `MemoryStore` query methods returned shallow copies of the `APIKey` struct. Since this struct contains map fields (like `ModelSpend`), concurrent read/write access to these maps by multiple goroutines triggered a data race and program crash.
- **Fix**:
  - Implemented `Clone()` deep copy methods for core structs including `APIKey`, `Team`, and `User`.
  - Enforced deep copying in `MemoryStore` before returning or storing objects to ensure memory data isolation.
- **Impacted Module**: `internal/auth`
- **Risk Level**: 🚨 High

## 2. Streaming Context Leak
- **Description**: The `Forward` method in `internal/streaming/forwarder.go` created a child context with a cancel function but did not ensure it was called upon method completion or exit.
- **Fix**: Added `defer f.cancel()` at the beginning of the `Forward()` method to ensure immediate resource release after the request lifecycle.
- **Impacted Module**: `internal/streaming`
- **Risk Level**: ⚠️ Medium

## 3. Global Panic Recovery Mechanism
- **Description**: The server middleware stack lacked panic capture logic. If a handler crashed, the request would hang without a standard JSON error response.
- **Fix**: Added `recoveryMiddleware` in `cmd/server/middleware.go` to capture panics, log errors, and return a standard 500 JSON error.
- **Impacted Module**: `cmd/server`
- **Risk Level**: ⚠️ Medium

## 4. Compilation and Compatibility Fix
- **Description**: Due to previous code refactoring, function signatures in `cmd/server/management_authz_test.go` were inconsistent with the actual implementation, causing compilation failure.
- **Fix**: Updated function signatures in the test cases to match the current implementation.
- **Impacted Module**: `cmd/server`
- **Risk Level**: ✅ Low

---
**Verification Status**: All fixes have passed regression testing with the `-race` flag enabled.
