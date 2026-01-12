# LLMux Changelog (Agent Context)

Recent changes and project history for context.

---

## 2026-01-12

### Fixes
- Wire fail_open from config into client and gateway rate limiter setup, with gateway backend-failure coverage.
- Remove duplicate RequestID middleware wrapping and wire streaming plugin hooks into client streams.
- Standardize router stats minute keys using Redis server time in distributed stats.

## 2026-01-09

### Features
- ✅ Added CORS middleware for frontend-backend communication
- ✅ Created comprehensive testing guide (`docs/LOCAL_TESTING_GUIDE.md`)
- ✅ Created PowerShell test script (`scripts/test_api.ps1`)

### Fixes
- 🐛 Fixed govet shadow errors in test files (renamed `mockLLM` → `mockServer`)
- 🐛 Fixed CI build failure due to missing `ui_assets/.gitkeep`
- 🐛 Fixed `closeErr` shadowing in `cmd/server/main.go`

### Documentation
- 📝 Complete rewrite of `README.md` and `README_CN.md`
- 📝 Rewrote `ui/README.md` with full documentation
- 📝 Created `.agent/` directory for AI assistance

---

## Project Status

### ✅ Completed Features

**Backend:**
- Core LLM gateway functionality
- 40+ provider support (OpenAI, Anthropic, Gemini, etc.)
- 6 routing strategies
- Response caching (memory/Redis)
- Plugin system
- Prometheus metrics
- OpenTelemetry tracing

**Enterprise:**
- Multi-tenant: Organizations → Teams → Users → Keys
- API key management with budgets
- OIDC/SSO authentication
- Rate limiting with tenant isolation
- Audit logging (store layer)

**Frontend:**
- Dashboard with all management pages
- Overview with charts
- CRUD for Users, Teams, Organizations, API Keys
- Audit log viewer
- Dark mode support

### ⚠️ Pending/Partial

| Feature            | Status | Notes                                    |
| ------------------ | ------ | ---------------------------------------- |
| PostgreSQL Store   | 70%    | Basic CRUD done, some methods pending    |
| Audit Integration  | 50%    | Store works, handler integration pending |
| Budget Enforcement | 80%    | Middleware works, reset logic pending    |
| Frontend Charts    | 60%    | Components ready, need real data         |

### 🔮 Planned

- [ ] Admin user management in Dashboard
- [ ] Real-time usage graphs
- [ ] Webhook notifications
- [ ] Model cost calculator
- [ ] Request replay/debugging

---

## Known Issues

1. **In-memory store** - Data lost on restart (expected for dev)
2. **Audit logs empty** - Handler integration not complete
3. **Overview charts empty** - Need actual LLM requests

---

## File Changes This Session

| File                            | Change                |
| ------------------------------- | --------------------- |
| `cmd/server/main.go`            | Added CORS middleware |
| `tests/e2e/*.go`                | Fixed shadow errors   |
| `cmd/server/ui_assets/.gitkeep` | Created for CI        |
| `docs/LOCAL_TESTING_GUIDE.md`   | New file              |
| `scripts/test_api.ps1`          | New file              |
| `.agent/*`                      | New directory         |
