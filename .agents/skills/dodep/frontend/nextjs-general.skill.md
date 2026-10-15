# SKILL #25 — nextjs-general.skill.md

```markdown
# nextjs-general.skill.md
# GAMBLING PLATFORM — NEXT.JS GENERAL CONVENTIONS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Frontend Web Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Main web application: Next.js 14 with App Router + TypeScript.
SSR for SEO (landing, sports pages). CSR for interactive (betslip, live).
Real-time via WebSocket (odds, scores, balance).

Target: FCP < 1.5s, LCP < 2.5s, TTI < 3.5s.

# ============================================================
# SECTION 2: PROJECT STRUCTURE
# ============================================================

```text
frontend/web/
├── next.config.ts
├── package.json
├── tsconfig.json
├── tailwind.config.ts
├── .env.local
│
├── public/
│   ├── icons/
│   ├── images/
│   └── locales/                # i18n translation files
│       ├── en/
│       ├── de/
│       └── pt/
│
├── src/
│   ├── app/                    # App Router pages
│   │   ├── layout.tsx          # Root layout
│   │   ├── page.tsx            # Home page
│   │   ├── (auth)/             # Auth route group
│   │   │   ├── login/page.tsx
│   │   │   ├── register/page.tsx
│   │   │   └── layout.tsx
│   │   ├── sports/
│   │   │   ├── page.tsx        # Sports lobby
│   │   │   ├── [sport]/page.tsx
│   │   │   └── [sport]/[event]/page.tsx
│   │   ├── casino/
│   │   │   ├── page.tsx
│   │   │   └── game/[id]/page.tsx
│   │   ├── wallet/
│   │   │   ├── page.tsx
│   │   │   ├── deposit/page.tsx
│   │   │   └── withdraw/page.tsx
│   │   ├── profile/
│   │   │   └── page.tsx
│   │   └── bets/
│   │       └── page.tsx
│   │
│   ├── components/             # Reusable UI components
│   │   ├── ui/                 # Primitives (Button, Input, Modal)
│   │   ├── sports/             # Sports-specific components
│   │   ├── casino/             # Casino-specific components
│   │   ├── wallet/             # Wallet components
│   │   ├── betslip/            # Bet slip components
│   │   └── layout/             # Header, Footer, Sidebar
│   │
│   ├── hooks/                  # Custom React hooks
│   │   ├── useAuth.ts
│   │   ├── useWebSocket.ts
│   │   ├── useBetslip.ts
│   │   └── useBalance.ts
│   │
│   ├── stores/                 # Zustand stores
│   │   ├── authStore.ts
│   │   ├── betslipStore.ts
│   │   ├── oddsStore.ts
│   │   └── balanceStore.ts
│   │
│   ├── lib/                    # Utilities
│   │   ├── api.ts              # API client (fetch wrapper)
│   │   ├── ws.ts               # WebSocket client
│   │   ├── format.ts           # Formatters (money, odds, dates)
│   │   └── validators.ts       # Form validators
│   │
│   ├── types/                  # TypeScript types
│   │   ├── api.ts              # API response types
│   │   ├── betting.ts          # Betting domain types
│   │   ├── casino.ts
│   │   └── user.ts
│   │
│   └── styles/
│       └── globals.css         # Tailwind base + custom
│
├── tests/
│   ├── e2e/                    # Playwright
│   └── unit/                   # Vitest
============================================================
SECTION 3: COMPONENT RULES
============================================================
text

1. Components are functional (no class components)
2. TypeScript strict: all props typed, no `any`
3. Server Components by default, "use client" only when needed:
   ✅ Server: static content, data fetching, SEO pages
   ✅ Client: interactive (forms, betslip, real-time, state)
4. Small components: < 100 lines (extract if larger)
5. One component per file, file named same as component
6. Props interface named {Component}Props
7. Use composition over prop drilling
8. No business logic in components — extract to hooks/stores
COMPONENT EXAMPLE
React

// src/components/sports/OddsButton.tsx
"use client";

import { cn } from "@/lib/utils";
import { useBetslipStore } from "@/stores/betslipStore";
import { formatOdds } from "@/lib/format";

interface OddsButtonProps {
  eventId: number;
  marketId: number;
  outcomeId: number;
  outcomeName: string;
  odds: number;
  previousOdds?: number;
  suspended?: boolean;
  oddsFormat: "decimal" | "fractional" | "american";
}

export function OddsButton({
  eventId, marketId, outcomeId, outcomeName,
  odds, previousOdds, suspended = false, oddsFormat,
}: OddsButtonProps) {
  const { selections, addSelection, removeSelection } = useBetslipStore();
  
  const isSelected = selections.some(
    (s) => s.outcomeId === outcomeId && s.marketId === marketId
  );
  
  const oddsDirection = previousOdds
    ? odds > previousOdds ? "up" : odds < previousOdds ? "down" : "none"
    : "none";
  
  const handleClick = () => {
    if (suspended) return;
    if (isSelected) {
      removeSelection(outcomeId);
    } else {
      addSelection({ eventId, marketId, outcomeId, outcomeName, odds });
    }
  };
  
  return (
    <button
      onClick={handleClick}
      disabled={suspended}
      className={cn(
        "px-3 py-2 rounded text-sm font-mono transition-all duration-200",
        isSelected && "bg-primary text-white",
        !isSelected && "bg-gray-100 hover:bg-gray-200",
        suspended && "opacity-50 cursor-not-allowed",
        oddsDirection === "up" && "text-green-600",
        oddsDirection === "down" && "text-red-600",
      )}
      aria-label={`${outcomeName} at ${formatOdds(odds, oddsFormat)}`}
    >
      {formatOdds(odds, oddsFormat)}
    </button>
  );
}
============================================================
SECTION 4: STATE MANAGEMENT
============================================================
React

// ── Zustand for client state ──
// src/stores/betslipStore.ts

import { create } from "zustand";

interface Selection {
  eventId: number;
  marketId: number;
  outcomeId: number;
  outcomeName: string;
  odds: number;
}

interface BetslipState {
  selections: Selection[];
  stake: string;
  betType: "single" | "accumulator";
  addSelection: (selection: Selection) => void;
  removeSelection: (outcomeId: number) => void;
  clearAll: () => void;
  setStake: (stake: string) => void;
  setBetType: (type: "single" | "accumulator") => void;
  combinedOdds: () => number;
  potentialWin: () => number;
}

export const useBetslipStore = create<BetslipState>((set, get) => ({
  selections: [],
  stake: "",
  betType: "single",
  
  addSelection: (selection) => set((state) => {
    // Max 20 selections
    if (state.selections.length >= 20) return state;
    // No duplicate outcomes
    if (state.selections.some((s) => s.outcomeId === selection.outcomeId)) return state;
    return { selections: [...state.selections, selection] };
  }),
  
  removeSelection: (outcomeId) => set((state) => ({
    selections: state.selections.filter((s) => s.outcomeId !== outcomeId),
  })),
  
  clearAll: () => set({ selections: [], stake: "" }),
  
  setStake: (stake) => set({ stake }),
  setBetType: (type) => set({ betType: type }),
  
  combinedOdds: () => {
    const { selections, betType } = get();
    if (betType === "single" || selections.length === 0) {
      return selections[0]?.odds ?? 0;
    }
    return selections.reduce((acc, s) => acc * s.odds, 1);
  },
  
  potentialWin: () => {
    const stake = parseFloat(get().stake) || 0;
    return stake * get().combinedOdds();
  },
}));

// ── TanStack Query for server state ──
// src/hooks/useBalance.ts

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

export function useBalance() {
  return useQuery({
    queryKey: ["balance"],
    queryFn: () => api.get<BalanceResponse>("/api/v1/wallet/balance"),
    refetchInterval: 30_000, // refetch every 30s
    staleTime: 10_000,       // consider stale after 10s
  });
}
============================================================
SECTION 5: API CLIENT
============================================================
React

// src/lib/api.ts
import { useAuthStore } from "@/stores/authStore";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.platform.com";

class ApiClient {
  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    const url = new URL(path, BASE_URL);
    if (params) Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
    return this.request<T>(url.toString(), { method: "GET" });
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(`${BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  private async request<T>(url: string, init: RequestInit): Promise<T> {
    const token = useAuthStore.getState().accessToken;
    
    const headers: HeadersInit = {
      ...init.headers,
      ...(token && { Authorization: `Bearer ${token}` }),
      "X-Request-ID": crypto.randomUUID(),
    };

    const response = await fetch(url, { ...init, headers });

    if (response.status === 401) {
      // Try refresh token
      const refreshed = await this.refreshToken();
      if (refreshed) {
        headers["Authorization"] = `Bearer ${useAuthStore.getState().accessToken}`;
        const retryResponse = await fetch(url, { ...init, headers });
        return this.handleResponse<T>(retryResponse);
      }
      useAuthStore.getState().logout();
      throw new ApiError("UNAUTHORIZED", "Session expired");
    }

    return this.handleResponse<T>(response);
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    const json = await response.json();
    if (!response.ok) {
      throw new ApiError(
        json.error?.code || "UNKNOWN",
        json.error?.message || "Unknown error",
        json.error?.details,
      );
    }
    return json.data as T;
  }

  private async refreshToken(): Promise<boolean> {
    const refreshToken = useAuthStore.getState().refreshToken;
    if (!refreshToken) return false;
    
    try {
      const response = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
      if (!response.ok) return false;
      const json = await response.json();
      useAuthStore.getState().setTokens(json.data.access_token, json.data.refresh_token);
      return true;
    } catch {
      return false;
    }
  }
}

export const api = new ApiClient();

export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public details?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}
============================================================
SECTION 6: FORMATTING UTILITIES
============================================================
React

// src/lib/format.ts

export function formatMoney(amount: number | string, currency: string): string {
  const num = typeof amount === "string" ? parseFloat(amount) : amount;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(num);
}

export function formatOdds(odds: number, format: "decimal" | "fractional" | "american"): string {
  switch (format) {
    case "decimal":
      return odds.toFixed(2);
    case "american":
      if (odds >= 2.0) return `+${Math.round((odds - 1) * 100)}`;
      return `-${Math.round(100 / (odds - 1))}`;
    case "fractional": {
      const profit = odds - 1;
      // Simple fraction approximation
      const denominator = 100;
      const numerator = Math.round(profit * denominator);
      const gcd = (a: number, b: number): number => b ? gcd(b, a % b) : a;
      const d = gcd(numerator, denominator);
      return `${numerator / d}/${denominator / d}`;
    }
  }
}

export function formatDate(date: string | Date, style: "short" | "long" = "short"): string {
  const d = typeof date === "string" ? new Date(date) : date;
  if (style === "short") {
    return new Intl.DateTimeFormat("en-US", {
      month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
    }).format(d);
  }
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric", month: "long", day: "numeric",
    hour: "2-digit", minute: "2-digit", second: "2-digit",
  }).format(d);
}
============================================================
SECTION 7: ANTI-PATTERNS
============================================================
text

❌ NEVER use `any` type → USE proper TypeScript types
❌ NEVER fetch data in useEffect → USE TanStack Query or Server Components
❌ NEVER put API keys in client code → USE server-side env vars only
❌ NEVER use "use client" on pages that can be Server Components
❌ NEVER store sensitive data in localStorage → USE httpOnly cookies or memory
❌ NEVER import server-only modules in client components
❌ NEVER hardcode API URLs → USE NEXT_PUBLIC_API_URL env var
❌ NEVER skip error boundaries → wrap sections in ErrorBoundary
❌ NEVER use inline styles → USE Tailwind CSS classes
❌ NEVER skip accessibility (aria labels, keyboard nav, semantic HTML)
❌ NEVER use float for money display → parse as string, format with Intl