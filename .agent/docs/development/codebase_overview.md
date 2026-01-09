# LLMux Codebase Guide

Quick reference for finding and understanding code.

## Backend (Go)

### Entry Point
- `cmd/server/main.go` - Server initialization, middleware stack, route registration

### API Handlers (internal/api/)

| File                        | Endpoints                            | Description        |
| --------------------------- | ------------------------------------ | ------------------ |
| `client_handler.go`         | `/v1/chat/completions`, `/v1/models` | Core LLM proxy     |
| `key_endpoints.go`          | `/key/*`                             | API key management |
| `user_endpoints.go`         | `/user/*`                            | User CRUD          |
| `team_endpoints.go`         | `/team/*`                            | Team management    |
| `organization_endpoints.go` | `/organization/*`                    | Org management     |
| `spend_endpoints.go`        | `/spend/*`                           | Usage analytics    |
| `audit_endpoints.go`        | `/audit/*`                           | Audit logs         |
| `invitation_endpoints.go`   | `/invitation/*`                      | Invite links       |

### Authentication (internal/auth/)

| File            | Purpose                                  |
| --------------- | ---------------------------------------- |
| `types.go`      | User, Team, APIKey, Organization structs |
| `store.go`      | Store interface definition               |
| `memory.go`     | In-memory Store implementation           |
| `postgres.go`   | PostgreSQL Store (partial)               |
| `middleware.go` | API key auth middleware                  |
| `oidc.go`       | SSO/JWT validation                       |
| `invitation.go` | Invitation link service                  |
| `audit.go`      | Audit logging                            |

### Routing (internal/router/)

| File         | Strategy                  |
| ------------ | ------------------------- |
| `simple.go`  | Base router with cooldown |
| `shuffle.go` | Random/weighted selection |
| `latency.go` | Lowest latency            |
| `busy.go`    | Least busy                |
| `tpm_rpm.go` | Lowest usage              |
| `cost.go`    | Lowest cost               |
| `tag.go`     | Tag-based filtering       |

### Providers (providers/)

Each provider implements the `Provider` interface:
- `openai/` - OpenAI API
- `anthropic/` - Claude API
- `gemini/` - Google Gemini
- `azure/` - Azure OpenAI
- `openailike/` - Generic OpenAI-compatible

---

## Frontend (Next.js)

### Pages (ui/src/app/)

| Path             | File                                 | Purpose            |
| ---------------- | ------------------------------------ | ------------------ |
| `/`              | `(dashboard)/page.tsx`               | Overview dashboard |
| `/api-keys`      | `(dashboard)/api-keys/page.tsx`      | Key management     |
| `/users`         | `(dashboard)/users/page.tsx`         | User management    |
| `/teams`         | `(dashboard)/teams/page.tsx`         | Team management    |
| `/organizations` | `(dashboard)/organizations/page.tsx` | Org management     |
| `/audit-logs`    | `(dashboard)/audit-logs/page.tsx`    | Audit viewer       |

### Key Components (ui/src/components/)

| Component              | Purpose                                           |
| ---------------------- | ------------------------------------------------- |
| `dashboard-layout.tsx` | Sidebar + main layout                             |
| `ui/`                  | shadcn/ui primitives (button, card, dialog, etc.) |
| `shared/common.tsx`    | StatusBadge, RoleBadge, etc.                      |
| `client-only.tsx`      | SSR-safe wrapper for charts                       |

### Data Hooks (ui/src/hooks/)

| Hook                     | API Endpoint                    |
| ------------------------ | ------------------------------- |
| `use-users.ts`           | `/user/list`, `/user/new`, etc. |
| `use-teams.ts`           | `/team/list`, `/team/new`, etc. |
| `use-organizations.ts`   | `/organization/*`               |
| `use-dashboard-stats.ts` | `/global/activity`              |
| `use-model-spend.ts`     | `/global/spend/models`          |

### API Client (ui/src/lib/api/client.ts)

Single class `LLMuxClient` with all API methods:
```typescript
class LLMuxClient {
  // Keys
  generateKey(req: GenerateKeyRequest): Promise<GenerateKeyResponse>
  listKeys(params): Promise<{data: APIKey[], total: number}>
  
  // Users
  createUser(req: CreateUserRequest): Promise<User>
  listUsers(params): Promise<{data: User[], total: number}>
  
  // Teams
  createTeam(req: CreateTeamRequest): Promise<Team>
  listTeams(params): Promise<{data: Team[], total: number}>
  
  // ... etc
}
```

---

## Common Patterns

### Adding a New API Endpoint (Backend)

1. Define handler in `internal/api/xxx_endpoints.go`
2. Register route in handler's `RegisterRoutes(mux)`
3. Use `auth.Store` for data operations
4. Return JSON response

### Adding a New Page (Frontend)

1. Create `ui/src/app/(dashboard)/new-page/page.tsx`
2. Create hook in `ui/src/hooks/use-new-data.ts`
3. Add API methods to `ui/src/lib/api/client.ts`
4. Add nav item in `ui/src/components/dashboard-layout.tsx`

### Data Flow

```
Frontend Hook → API Client → HTTP Request
                                   ↓
Backend Handler ← Router ← Middleware Stack
       ↓
   auth.Store (memory/postgres)
       ↓
   JSON Response → Frontend → React Query Cache → UI
```

---

## Testing Files

| Path                      | Purpose              |
| ------------------------- | -------------------- |
| `tests/e2e/`              | E2E API tests        |
| `internal/auth/*_test.go` | Unit tests for auth  |
| `ui/src/test/`            | Frontend unit tests  |
| `ui/e2e/`                 | Playwright E2E tests |
