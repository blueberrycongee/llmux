# Middleware & Data Fetching Refactoring Todo List

This document tracks the locations where hardcoded data or logic needs to be replaced with real data fetching implementations.

## 1. Authentication Store Wiring
**Location:** `cmd/server/main.go`
**Status:** ⚠️ Hardcoded to `MemoryStore`
**Task:**
- [ ] Initialize `PostgresStore` when `cfg.Database.Enabled` is true.
- [ ] Pass the persistent store to `auth.NewMiddleware` and other consumers.
- [ ] Ensure proper database connection management (pooling, closing).

## 2. Model Access Control Middleware
**Location:** `internal/auth/middleware.go` -> `ModelAccessMiddleware`
**Status:** ❌ Empty / Placeholder
**Task:**
- [ ] Implement logic to parse the request body (partially) to extract the requested model.
- [ ] Query `Store` (or check `AuthContext`) to verify if the API Key/User has permission to access the requested model.
- [ ] Return `403 Forbidden` if access is denied.

## 3. List Models Endpoint
**Location:** `internal/api/handler.go` -> `ListModels`
**Status:** ❌ Hardcoded empty list
**Task:**
- [ ] Iterate through all registered providers in `h.registry`.
- [ ] Aggregate available models from all providers.
- [ ] (Optional) Filter models based on the authenticated user's permissions (if `ModelAccessMiddleware` logic is integrated).
- [ ] Return the real list of models in OpenAI-compatible format.
