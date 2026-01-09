# LLMux Agent Context

This directory contains documentation specifically designed for AI coding assistants to understand the LLMux project quickly.

## Directory Structure

```
.agent/
├── README.md           # This file - entry point for AI
├── ARCHITECTURE.md     # System architecture and design decisions
├── CODEBASE.md         # Key code locations and patterns
├── WORKFLOWS.md        # Common development workflows
└── CHANGELOG.md        # Recent changes and context
```

## Quick Project Summary

**LLMux** is a high-performance LLM Gateway written in Go with:
- Unified OpenAI-compatible API for multiple providers
- Enterprise features: multi-tenancy, budgets, SSO
- Next.js 14 Dashboard for management

## Key Technologies

| Component          | Technology                       |
| ------------------ | -------------------------------- |
| Backend Gateway    | Go 1.23+, net/http               |
| Frontend Dashboard | Next.js 14, React 18, TypeScript |
| UI Components      | shadcn/ui, Tailwind CSS, Tremor  |
| State Management   | TanStack Query                   |

## Important Files to Know

| File                       | Purpose                        |
| -------------------------- | ------------------------------ |
| `cmd/server/main.go`       | Server entry point             |
| `internal/api/`            | HTTP handlers                  |
| `internal/auth/`           | Authentication & authorization |
| `ui/src/lib/api/client.ts` | Frontend API client            |
| `ui/src/hooks/`            | React data hooks               |
| `config/config.yaml`       | Main configuration             |

## Development Commands

```bash
# Backend
make build            # Build gateway
make test             # Run tests
make lint             # Run linters

# Frontend
cd ui && npm run dev  # Start dev server
npm run test          # Run tests
npm run lint          # Run linter
```

## Current State

- ✅ Core gateway functionality complete
- ✅ Enterprise features (auth, teams, budgets)
- ✅ Dashboard UI with all CRUD pages
- ⚠️ Audit logging integration pending
- ⚠️ PostgreSQL store needs full implementation
