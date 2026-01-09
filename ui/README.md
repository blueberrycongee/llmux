# LLMux Dashboard

The enterprise-grade Web Dashboard for LLMux, built with modern React technologies.

## 🛠️ Tech Stack

- **Framework**: [Next.js 14](https://nextjs.org/) with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **UI Components**: [shadcn/ui](https://ui.shadcn.com/) + [Radix UI](https://www.radix-ui.com/)
- **Charts**: [Tremor](https://tremor.so/) + [Recharts](https://recharts.org/)
- **State Management**: [TanStack Query](https://tanstack.com/query) (React Query)
- **Animations**: [Framer Motion](https://www.framer.com/motion/)
- **Forms**: React Hook Form + Zod
- **Testing**: Vitest + Playwright

## ✨ Features

### Dashboard Overview
- Real-time analytics with request volume and token usage charts
- Model distribution visualization
- Top models by spend ranking
- Key performance indicators (KPIs)

### Resource Management
- **API Keys**: Create, list, block/unblock, regenerate, delete
- **Users**: Full user management with role assignment and budget limits
- **Teams**: Team creation, member management, budget tracking
- **Organizations**: Organization hierarchy with member roles

### User Experience
- 🌙 Dark mode by default with theme support
- 📱 Responsive design for all screen sizes
- ⚡ Optimized loading with skeleton states
- 🔍 Server-side search and filtering
- 🎨 Smooth animations and micro-interactions

## 🏁 Getting Started

### Prerequisites

- Node.js 18+
- npm or yarn
- LLMux gateway running (default: http://localhost:8080)

### Installation

```bash
# Install dependencies
npm install

# Copy environment template
cp .env.example .env.local

# Edit .env.local with your API URL
# NEXT_PUBLIC_API_URL=http://localhost:8080
```

### Development

```bash
# Start development server
npm run dev

# Open http://localhost:3000
```

### Build

```bash
# Build for production
npm run build

# Start production server
npm run start
```

## 🧪 Testing

### Unit Tests

```bash
# Run unit tests
npm run test

# Run in watch mode
npm run test:watch

# Run with coverage
npm run test:coverage
```

### E2E Tests

```bash
# Run E2E tests headless
npm run test:e2e

# Run E2E tests with UI
npm run test:e2e:ui

# Run E2E tests headed (visible browser)
npm run test:e2e:headed
```

### All Tests

```bash
npm run test:all
```

## 📁 Project Structure

```
ui/
├── src/
│   ├── app/                    # Next.js App Router
│   │   ├── (dashboard)/        # Dashboard pages (grouped route)
│   │   │   ├── page.tsx        # Overview dashboard
│   │   │   ├── api-keys/       # API Keys management
│   │   │   ├── users/          # Users management
│   │   │   ├── teams/          # Teams management
│   │   │   └── organizations/  # Organizations management
│   │   ├── globals.css         # Global styles
│   │   └── layout.tsx          # Root layout
│   │
│   ├── components/
│   │   ├── ui/                 # shadcn/ui components
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── dialog.tsx
│   │   │   ├── input.tsx
│   │   │   ├── select.tsx
│   │   │   ├── skeleton.tsx
│   │   │   ├── table.tsx
│   │   │   └── ...
│   │   ├── shared/             # Shared components
│   │   │   └── common.tsx      # StatusBadge, RoleBadge, etc.
│   │   ├── api-keys/           # API Keys components
│   │   ├── dashboard-layout.tsx
│   │   ├── client-only.tsx     # Client-only wrapper
│   │   └── providers.tsx       # React Query provider
│   │
│   ├── hooks/
│   │   ├── use-dashboard-stats.ts
│   │   ├── use-model-spend.ts
│   │   ├── use-users.ts
│   │   ├── use-teams.ts
│   │   ├── use-organizations.ts
│   │   └── index.ts            # Barrel export
│   │
│   ├── lib/
│   │   ├── api/
│   │   │   ├── client.ts       # LLMux API client
│   │   │   └── index.ts
│   │   └── utils.ts            # Utility functions
│   │
│   ├── types/
│   │   └── api.ts              # TypeScript types
│   │
│   └── test/
│       └── setup.ts            # Test setup
│
├── e2e/                        # Playwright E2E tests
│   └── phase2.spec.ts
│
├── public/                     # Static assets
├── next.config.mjs             # Next.js configuration
├── tailwind.config.ts          # Tailwind configuration
├── tsconfig.json               # TypeScript configuration
├── vitest.config.ts            # Vitest configuration
└── playwright.config.ts        # Playwright configuration
```

## 🔌 API Integration

The dashboard communicates with the LLMux gateway via the API client in `src/lib/api/client.ts`.

### Configuration

Set the API URL in your environment:

```bash
# .env.local
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### Available Hooks

| Hook                     | Description                                         |
| ------------------------ | --------------------------------------------------- |
| `useDashboardStats`      | Fetch dashboard statistics and daily metrics        |
| `useModelSpend`          | Fetch model spend data                              |
| `useUsers`               | User list with search, filters, and CRUD operations |
| `useUserInfo`            | Single user details                                 |
| `useTeams`               | Team list with pagination                           |
| `useTeamInfo`            | Single team details                                 |
| `useTeamMembers`         | Team member management                              |
| `useOrganizations`       | Organization list                                   |
| `useOrganizationInfo`    | Single organization details                         |
| `useOrganizationMembers` | Organization member management                      |

## 🎨 Theming

The dashboard uses CSS variables for theming, defined in `globals.css`:

```css
:root {
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  /* ... */
}

.dark {
  --background: 222.2 84% 4.9%;
  --foreground: 210 40% 98%;
  /* ... */
}
```

Dark mode is enabled by default. You can toggle themes using `next-themes`.

## 📝 Development Guidelines

1. **Components**: Use shadcn/ui components as base, customize with Tailwind
2. **State**: Use TanStack Query for server state, useState for local state
3. **Types**: Define types in `types/api.ts`, mirror backend types
4. **Hooks**: Create custom hooks in `hooks/` for data fetching
5. **API Client**: Add new endpoints to `lib/api/client.ts`

## 🐛 Troubleshooting

### Hydration Errors

Charts may cause hydration errors due to SSR. Wrap chart components with `ClientOnly`:

```tsx
import { ClientOnly } from "@/components/client-only";

<ClientOnly fallback={<Skeleton />}>
  <Chart data={data} />
</ClientOnly>
```

### API Connection Issues

1. Ensure LLMux gateway is running
2. Check `NEXT_PUBLIC_API_URL` is correctly set
3. Verify CORS is enabled on the gateway

## 📄 License

MIT License - see [LICENSE](../LICENSE)
