# DOD Web Platform

Next.js 14 web application for DOD gambling platform.

## 🏗 Architecture

This project follows modern Next.js 14 conventions with proper separation of concerns:

```
apps/web/
├── src/
│   ├── app/                    # Next.js App Router pages
│   │   ├── (main)/             # Main layout routes
│   │   │   ├── sportsbook/
│   │   │   ├── casino/
│   │   │   ├── wallet/
│   │   │   ├── profile/
│   │   │   ├── bonuses/
│   │   │   └── support/
│   │   ├── layout.tsx
│   │   └── globals.css
│   │
│   ├── components/
│   │   ├── ui/                 # Radix-based primitives
│   │   │   ├── button.tsx
│   │   │   ├── skeleton.tsx
│   │   │   └── ...
│   │   ├── shared/             # Shared business components
│   │   │   ├── currency-amount.tsx
│   │   │   ├── odds-button.tsx
│   │   │   └── ...
│   │   ├── layout/             # Layout components
│   │   │   ├── header.tsx
│   │   │   ├── footer.tsx
│   │   │   └── mobile-nav.tsx
│   │   └── pages/              # Page components
│   │
│   ├── lib/
│   │   ├── api/                # API client & modules
│   │   │   ├── client.ts       # Base API client
│   │   │   ├── auth.ts         # Auth endpoints
│   │   │   ├── wallet.ts       # Wallet endpoints
│   │   │   ├── bets.ts         # Betting endpoints
│   │   │   ├── casino.ts       # Casino endpoints
│   │   │   ├── errors.ts       # Error handling
│   │   │   └── idempotency.ts  # Idempotency keys
│   │   ├── query-keys.ts       # TanStack Query keys
│   │   ├── websocket.ts        # WebSocket manager
│   │   ├── format.ts           # Formatters (money, odds, dates)
│   │   └── utils.ts            # Utilities (cn helper)
│   │
│   ├── stores/                 # Zustand stores
│   │   ├── auth-store.ts
│   │   ├── bet-slip-store.ts
│   │   └── index.ts
│   │
│   └── types/                  # TypeScript types
│
├── package.json
├── next.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── README.md
```

## 🛠 Tech Stack

### Core
- **Framework:** Next.js 14 (App Router)
- **Language:** TypeScript 5 (strict mode)
- **Styling:** Tailwind CSS 3 + Radix UI primitives
- **UI Components:** class-variance-authority, clsx, tailwind-merge

### State & Data
- **State Management:** Zustand (client state)
- **Data Fetching:** TanStack Query v5 (server state)
- **URL State:** nuqs (type-safe search params)
- **WebSocket:** Native WebSocket API with manager

### Forms & Validation
- **Forms:** React Hook Form
- **Validation:** Zod
- **Resolvers:** @hookform/resolvers

### UX
- **Notifications:** Sonner (toast notifications)
- **Theme:** next-themes (dark/light)
- **Charts:** Recharts

## 🚀 Quick Start

```bash
# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

## 📄 Pages

| Page | Route | Description |
|------|-------|-------------|
| Sportsbook | `/sportsbook` | Sports betting (live/pre-match) |
| Casino | `/casino` | Casino games (slots, live, table) |
| Wallet | `/wallet` | Wallet (deposit, withdraw, history) |
| Profile | `/profile` | User profile, KYC |
| Bonuses | `/bonuses` | Available bonuses |
| Support | `/support` | Support, FAQ |

## 🔌 API Integration

### API Client

```typescript
// Typed API modules
import { authApi } from "@/lib/api/auth";
import { walletApi } from "@/lib/api/wallet";
import { betsApi } from "@/lib/api/bets";

// Usage with error handling
try {
  const user = await authApi.me();
} catch (error) {
  const message = getErrorMessage(error);
}
```

### TanStack Query

```typescript
import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "@/lib/query-keys";
import { walletApi } from "@/lib/api/wallet";

function useBalance(currency: string) {
  return useQuery({
    queryKey: queryKeys.wallet.balance(currency),
    queryFn: () => walletApi.getBalance(currency),
    staleTime: 30_000,
  });
}
```

### WebSocket

```typescript
import { useWebSocket } from "@/lib/websocket";

function useLiveOdds(eventId: number) {
  const [odds, setOdds] = useState({});
  
  useWebSocket(
    `event:${eventId}:odds`,
    (data) => setOdds(data),
    true // enabled
  );
}
```

## 🎨 Components

### UI Primitives

```typescript
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

<Button variant="default" size="lg" isLoading={isLoading}>
  Place Bet
</Button>

<Skeleton className="h-4 w-32" />
```

### Shared Components

```typescript
import { CurrencyAmount } from "@/components/shared/currency-amount";
import { OddsButton } from "@/components/shared/odds-button";

<CurrencyAmount amount={1000} currency="RUB" showSign colorize />

<OddsButton
  eventId={1}
  marketId={1}
  outcomeId={1}
  outcomeName="Home"
  odds={2.50}
  previousOdds={2.40}
  isSelected={true}
/>
```

## 🔧 Configuration

### Environment Variables

```bash
# .env.local
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080
NEXT_PUBLIC_APP_ENV=development
```

### Security Headers

Configured in `next.config.ts`:
- Strict-Transport-Security
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Content-Security-Policy
- Referrer-Policy

## 📊 Performance Targets

| Metric | Target |
|--------|--------|
| LCP | < 2.5s |
| FID | < 100ms |
| CLS | < 0.1 |
| TTI | < 3.5s |

## 🧪 Testing

```bash
# Unit tests
npm run test

# E2E tests
npm run test:e2e

# Type check
npm run type-check

# Lint
npm run lint
```

## 📦 Deployment

### Docker

```bash
docker build -t dod-web apps/web
docker run -p 3000:3000 dod-web
```

### Kubernetes

```bash
helm upgrade --install web infra/helm/charts/web \
  --namespace platform-dev \
  --set image.tag=latest
```

## 🔐 Security

- **Authentication:** JWT with auto-refresh
- **Authorization:** Role-based access control
- **Input Validation:** Zod schemas
- **XSS Protection:** React auto-escaping + CSP
- **CSRF Protection:** SameSite cookies
- **Rate Limiting:** Server-side via API Gateway

## 📝 License

Proprietary - DOD
