# LLMux Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Clients                                    │
│   (Cursor, SDKs, ChatBots, Applications)                            │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │ HTTP (OpenAI-compatible)
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        LLMux Gateway                                 │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                │
│  │    Auth      │ │   Router     │ │    Cache     │                │
│  │  Middleware  │ │  (6 types)   │ │ (mem/redis)  │                │
│  └──────────────┘ └──────────────┘ └──────────────┘                │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                │
│  │   Metrics    │ │   Tracing    │ │   Plugins    │                │
│  │ (Prometheus) │ │  (OTel)      │ │  (Pipeline)  │                │
│  └──────────────┘ └──────────────┘ └──────────────┘                │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
          ┌───────────────────────┼───────────────────────┐
          ▼                       ▼                       ▼
   ┌─────────────┐         ┌─────────────┐         ┌─────────────┐
   │   OpenAI    │         │  Anthropic  │         │   Gemini    │
   └─────────────┘         └─────────────┘         └─────────────┘
```

## Directory Structure

```
llmux/
├── cmd/server/           # Entry point, main.go
├── config/               # YAML configuration files
│
├── internal/             # Private application code
│   ├── api/              # HTTP handlers & routes
│   │   ├── client_handler.go   # /v1/chat/completions
│   │   ├── user_endpoints.go   # /user/* endpoints
│   │   ├── team_endpoints.go   # /team/* endpoints
│   │   ├── key_endpoints.go    # /key/* endpoints
│   │   └── ...
│   ├── auth/             # Authentication & authorization
│   │   ├── store.go      # Store interface
│   │   ├── memory.go     # In-memory implementation
│   │   ├── postgres.go   # PostgreSQL implementation
│   │   ├── middleware.go # Auth middleware
│   │   ├── oidc.go       # SSO/OIDC support
│   │   └── types.go      # User, Team, APIKey types
│   ├── cache/            # Response caching
│   ├── config/           # Config loading
│   ├── router/           # Routing strategies
│   ├── plugin/           # Plugin system
│   └── ...
│
├── providers/            # LLM provider adapters (public API)
│   ├── openai/
│   ├── anthropic/
│   ├── gemini/
│   └── ...
│
├── ui/                   # Next.js Dashboard
│   ├── src/
│   │   ├── app/          # Pages (App Router)
│   │   ├── components/   # React components
│   │   ├── hooks/        # Data fetching hooks
│   │   └── lib/api/      # API client
│   └── package.json
│
├── tests/                # Integration & E2E tests
│   └── e2e/
│
└── deploy/               # K8s, Helm, Docker configs
```

## Key Interfaces

### auth.Store (internal/auth/store.go)
```go
type Store interface {
    // API Keys
    CreateAPIKey(ctx context.Context, key *APIKey) error
    GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
    ListAPIKeys(ctx context.Context, filter APIKeyFilter) ([]*APIKey, int, error)
    
    // Users
    CreateUser(ctx context.Context, user *User) error
    GetUser(ctx context.Context, id string) (*User, error)
    ListUsers(ctx context.Context, filter UserFilter) ([]*User, int, error)
    
    // Teams
    CreateTeam(ctx context.Context, team *Team) error
    ListTeams(ctx context.Context, filter TeamFilter) ([]*Team, int, error)
    
    // ... more methods
}
```

### provider.Provider (internal/provider/provider.go)
```go
type Provider interface {
    Name() string
    SupportsModel(model string) bool
    ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
    ChatCompletionStream(ctx context.Context, req *types.ChatCompletionRequest) (<-chan *types.ChatCompletionChunk, error)
}
```

## Request Flow

1. **Client** sends `POST /v1/chat/completions`
2. **CORS Middleware** adds headers (for dashboard)
3. **Auth Middleware** validates API key, checks budget
4. **ClientHandler** parses request
5. **Router** selects best provider/deployment
6. **Provider** forwards to upstream LLM
7. **Response** returned (optionally cached)
8. **Usage Logged** to store

## Routing Strategies

| Strategy         | Description            | Use Case                   |
| ---------------- | ---------------------- | -------------------------- |
| `simple-shuffle` | Random selection       | Default, load distribution |
| `lowest-latency` | Pick fastest           | Latency-sensitive          |
| `least-busy`     | Fewest active requests | Avoid overload             |
| `lowest-tpm-rpm` | Lowest usage           | Rate limit avoidance       |
| `lowest-cost`    | Cheapest option        | Cost optimization          |
| `tag-based`      | Filter by tags         | Workload isolation         |

## Authentication Flow

```
Request → Auth Middleware
           │
           ├─ Extract API Key from Authorization header
           │
           ├─ Hash key and lookup in Store
           │
           ├─ Check: key.is_active && !key.blocked
           │
           ├─ Check: budget not exceeded
           │
           └─ Inject auth context → Next Handler
```

## Enterprise Features

- **Multi-tenancy**: Organizations → Teams → Users → API Keys
- **Budget Management**: Max budget per key/user/team with reset periods
- **SSO/OIDC**: JWT validation with role mapping
- **Audit Logging**: Track all management operations
- **Rate Limiting**: TPM/RPM limits with tenant isolation
