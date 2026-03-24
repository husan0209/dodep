## #27 nextjs-state-management.skill.md

```markdown
# nextjs-state-management.skill.md

## РОЛЬ
Ты управляешь состоянием в Next.js 14 приложении для гемблинг-платформы.
Zustand для клиентского стейта, TanStack Query для серверного.

## КОНТЕКСТ
- Real-time данные: odds обновляются каждые 1-5 сек через WebSocket
- Финансовые данные: баланс, ставки — критическая точность
- Оптимистичные обновления для быстрого UX
- SSR: Server Components + Client Components разделение

## АРХИТЕКТУРА СОСТОЯНИЯ
┌─────────────────────────────────────────────────┐
│ STATE LAYERS │
│ │
│ Server State (TanStack Query): │
│ ├── User profile, settings │
│ ├── Bet history │
│ ├── Game catalog │
│ ├── Payment history │
│ └── Promotions, bonuses │
│ │
│ Client State (Zustand): │
│ ├── Auth (tokens, isAuthenticated) │
│ ├── BetSlip (selections, stake) │
│ ├── UI (sidebar, modals, theme) │
│ └── Preferences (odds format, language) │
│ │
│ Real-time State (WebSocket + Zustand): │
│ ├── Live odds │
│ ├── Live scores │
│ ├── Balance updates │
│ └── Bet status changes │
│ │
│ URL State (nuqs / searchParams): │
│ ├── Filters (sport, league, date) │
│ ├── Pagination │
│ ├── Search query │
│ └── Tab selection │
└─────────────────────────────────────────────────┘

src/
├── stores/ # Zustand stores
│ ├── authStore.ts
│ ├── betSlipStore.ts
│ ├── uiStore.ts
│ └── preferencesStore.ts
├── hooks/
│ ├── queries/ # TanStack Query hooks
│ │ ├── useUser.ts
│ │ ├── useBets.ts
│ │ ├── useGames.ts
│ │ ├── usePayments.ts
│ │ └── useBonuses.ts
│ ├── mutations/ # TanStack Mutation hooks
│ │ ├── usePlaceBet.ts
│ │ ├── useDeposit.ts
│ │ └── useUpdateProfile.ts
│ ├── websocket/ # WebSocket hooks
│ │ ├── useOddsStream.ts
│ │ ├── useBalanceStream.ts
│ │ └── useBetStatusStream.ts
│ └── domain/ # Составные бизнес-хуки
│ ├── useBetSlip.ts
│ ├── useUserBalance.ts
│ └── useLiveEvent.ts

text


## ZUSTAND STORES

### AuthStore
```tsx
// stores/authStore.ts
import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  userId: number | null;
  isAuthenticated: boolean;
  
  setTokens: (access: string, refresh: string, userId: number) => void;
  clearAuth: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      userId: null,
      isAuthenticated: false,
      
      setTokens: (access, refresh, userId) =>
        set({
          accessToken: access,
          refreshToken: refresh,
          userId,
          isAuthenticated: true,
        }),
        
      clearAuth: () =>
        set({
          accessToken: null,
          refreshToken: null,
          userId: null,
          isAuthenticated: false,
        }),
    }),
    {
      name: 'auth-storage',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        refreshToken: state.refreshToken,
        userId: state.userId,
      }),
      // НЕ сохраняй accessToken в localStorage — только в памяти
    }
  )
);
BetSlipStore
React

// stores/betSlipStore.ts
import { create } from 'zustand';

interface Selection {
  eventId: number;
  marketId: number;
  outcomeId: number;
  odds: number;
  eventName: string;
  marketName: string;
  outcomeName: string;
}

interface BetSlipState {
  selections: Selection[];
  stake: number;
  betType: 'single' | 'accumulator' | 'system';
  
  // Computed
  totalOdds: () => number;
  potentialWin: () => number;
  
  // Actions
  addSelection: (selection: Selection) => void;
  removeSelection: (outcomeId: number) => void;
  toggleSelection: (selection: Selection) => void;
  updateOdds: (outcomeId: number, newOdds: number) => void;
  setStake: (stake: number) => void;
  setBetType: (type: BetSlipState['betType']) => void;
  clear: () => void;
}

export const useBetSlipStore = create<BetSlipState>((set, get) => ({
  selections: [],
  stake: 0,
  betType: 'single',
  
  totalOdds: () => {
    const { selections, betType } = get();
    if (betType === 'single' || selections.length <= 1) {
      return selections[0]?.odds ?? 0;
    }
    return selections.reduce((acc, s) => acc * s.odds, 1);
  },
  
  potentialWin: () => {
    const { stake } = get();
    return stake * get().totalOdds();
  },
  
  addSelection: (selection) =>
    set((state) => {
      // Нельзя добавить два исхода из одного маркета
      const filtered = state.selections.filter(
        (s) => s.marketId !== selection.marketId
      );
      return { selections: [...filtered, selection] };
    }),
    
  removeSelection: (outcomeId) =>
    set((state) => ({
      selections: state.selections.filter((s) => s.outcomeId !== outcomeId),
    })),
    
  toggleSelection: (selection) => {
    const exists = get().selections.find(
      (s) => s.outcomeId === selection.outcomeId
    );
    if (exists) {
      get().removeSelection(selection.outcomeId);
    } else {
      get().addSelection(selection);
    }
  },
  
  updateOdds: (outcomeId, newOdds) =>
    set((state) => ({
      selections: state.selections.map((s) =>
        s.outcomeId === outcomeId ? { ...s, odds: newOdds } : s
      ),
    })),
    
  setStake: (stake) => set({ stake: Math.max(0, stake) }),
  setBetType: (betType) => set({ betType }),
  clear: () => set({ selections: [], stake: 0, betType: 'single' }),
}));
TANSTACK QUERY — СЕРВЕРНОЕ СОСТОЯНИЕ
Правила Query Keys
React

// hooks/queries/queryKeys.ts
export const queryKeys = {
  user: {
    all: ['user'] as const,
    profile: () => [...queryKeys.user.all, 'profile'] as const,
    balance: () => [...queryKeys.user.all, 'balance'] as const,
    sessions: () => [...queryKeys.user.all, 'sessions'] as const,
  },
  bets: {
    all: ['bets'] as const,
    list: (filters: BetFilters) => [...queryKeys.bets.all, 'list', filters] as const,
    active: () => [...queryKeys.bets.all, 'active'] as const,
    detail: (id: number) => [...queryKeys.bets.all, 'detail', id] as const,
  },
  sports: {
    all: ['sports'] as const,
    events: (sportId: number, filters?: EventFilters) =>
      [...queryKeys.sports.all, 'events', sportId, filters] as const,
    event: (eventId: number) =>
      [...queryKeys.sports.all, 'event', eventId] as const,
    markets: (eventId: number) =>
      [...queryKeys.sports.all, 'markets', eventId] as const,
  },
  games: {
    all: ['games'] as const,
    list: (filters: GameFilters) => [...queryKeys.games.all, 'list', filters] as const,
    detail: (id: number) => [...queryKeys.games.all, 'detail', id] as const,
    providers: () => [...queryKeys.games.all, 'providers'] as const,
  },
} as const;
Query Hook пример
React

// hooks/queries/useBets.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';
import { betsApi } from '@/lib/api/bets';

export function useActiveBets() {
  return useQuery({
    queryKey: queryKeys.bets.active(),
    queryFn: betsApi.getActive,
    refetchInterval: 30_000,      // обновлять каждые 30 сек
    staleTime: 10_000,            // считать свежим 10 сек
  });
}

export function useBetHistory(filters: BetFilters) {
  return useQuery({
    queryKey: queryKeys.bets.list(filters),
    queryFn: () => betsApi.getHistory(filters),
    staleTime: 60_000,
    placeholderData: keepPreviousData, // при смене фильтров
  });
}
Mutation с оптимистичным обновлением
React

// hooks/mutations/usePlaceBet.ts
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '../queries/queryKeys';
import { betsApi } from '@/lib/api/bets';
import { useBetSlipStore } from '@/stores/betSlipStore';
import { useAuthStore } from '@/stores/authStore';
import { toast } from 'sonner';

export function usePlaceBet() {
  const queryClient = useQueryClient();
  const clearBetSlip = useBetSlipStore((s) => s.clear);
  
  return useMutation({
    mutationFn: betsApi.placeBet,
    
    onMutate: async (newBet) => {
      // Отмена текущих refetch баланса
      await queryClient.cancelQueries({ queryKey: queryKeys.user.balance() });
      
      // Сохранить предыдущий баланс
      const previousBalance = queryClient.getQueryData(
        queryKeys.user.balance()
      );
      
      // Оптимистично уменьшить баланс
      queryClient.setQueryData(queryKeys.user.balance(), (old: any) => ({
        ...old,
        available: old.available - newBet.stake,
      }));
      
      return { previousBalance };
    },
    
    onSuccess: (data) => {
      clearBetSlip();
      toast.success('Ставка принята', {
        description: `Потенциальный выигрыш: ${data.potentialWin}`,
      });
      // Обновить историю ставок
      queryClient.invalidateQueries({ queryKey: queryKeys.bets.active() });
    },
    
    onError: (error, _variables, context) => {
      // Откатить оптимистичное обновление
      if (context?.previousBalance) {
        queryClient.setQueryData(
          queryKeys.user.balance(),
          context.previousBalance
        );
      }
      toast.error('Ошибка', {
        description: getErrorMessage(error),
      });
    },
    
    onSettled: () => {
      // Всегда обновить баланс с сервера
      queryClient.invalidateQueries({ queryKey: queryKeys.user.balance() });
    },
  });
}
WEBSOCKET + ZUSTAND
React

// hooks/websocket/useOddsStream.ts
'use client';

import { useEffect, useRef } from 'react';
import { useBetSlipStore } from '@/stores/betSlipStore';

export function useOddsStream(eventIds: number[]) {
  const wsRef = useRef<WebSocket | null>(null);
  const updateOdds = useBetSlipStore((s) => s.updateOdds);
  
  useEffect(() => {
    if (!eventIds.length) return;
    
    const ws = new WebSocket(
      `${process.env.NEXT_PUBLIC_WS_URL}/odds`
    );
    wsRef.current = ws;
    
    ws.onopen = () => {
      ws.send(JSON.stringify({
        action: 'subscribe',
        events: eventIds,
      }));
    };
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      // Обновить BetSlip если пользователь выбрал этот исход
      if (data.type === 'odds_update') {
        updateOdds(data.outcomeId, data.newOdds);
      }
    };
    
    ws.onclose = () => {
      // Автоматический reconnect через 3 сек
      setTimeout(() => {
        if (wsRef.current === ws) {
          // Переподключение будет через re-render
        }
      }, 3000);
    };
    
    return () => {
      ws.close();
      wsRef.current = null;
    };
  }, [eventIds.join(',')]);
}
АНТИПАТТЕРНЫ
React

// ❌ ПЛОХО: серверные данные в Zustand
const useStore = create((set) => ({
  user: null,
  fetchUser: async () => {
    const user = await api.getUser();
    set({ user });
  },
}));

// ✅ ПРАВИЛЬНО: серверные данные в TanStack Query
function useUser() {
  return useQuery({
    queryKey: queryKeys.user.profile(),
    queryFn: userApi.getProfile,
  });
}

// ❌ ПЛОХО: глобальный стор для всего
const useGlobalStore = create((set) => ({
  user: null, bets: [], games: [], theme: 'dark',
  odds: {}, balance: 0, notifications: [],
  // ... 50 полей в одном сторе
}));

// ✅ ПРАВИЛЬНО: маленькие специализированные сторы
const useAuthStore = create(/* ... */);
const useBetSlipStore = create(/* ... */);
const useUIStore = create(/* ... */);

// ❌ ПЛОХО: подписка на весь стор
function Component() {
  const store = useBetSlipStore(); // ре-рендер на ЛЮБОЕ изменение
}

// ✅ ПРАВИЛЬНО: подписка на конкретный slice
function Component() {
  const stake = useBetSlipStore((s) => s.stake);
  const setStake = useBetSlipStore((s) => s.setStake);
}

// ❌ ПЛОХО: нет staleTime — бесконечные рефетчи
useQuery({ queryKey: ['user'], queryFn: getUser });

// ✅ ПРАВИЛЬНО: адекватный staleTime
useQuery({ 
  queryKey: ['user'], 
  queryFn: getUser,
  staleTime: 5 * 60 * 1000,  // 5 минут
});
QUERY CLIENT CONFIG
React

// lib/queryClient.ts
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,              // 30 сек по умолчанию
      gcTime: 5 * 60_000,            // 5 мин в кэше
      retry: 2,
      refetchOnWindowFocus: false,    // гемблинг: не рефетчить при фокусе
      refetchOnReconnect: true,
    },
    mutations: {
      retry: 1,
    },
  },
});
ПРАВИЛА РАЗДЕЛЕНИЯ SERVER / CLIENT
text

Server Components (default в Next.js 14):
  ✅ Статический контент (правила, FAQ, промо)
  ✅ SEO-критичные страницы (лендинг)
  ✅ Layout-компоненты без интерактивности
  ✅ Начальная загрузка данных (dehydrate)

Client Components ('use client'):
  ✅ Всё с обработкой событий (onClick, onChange)
  ✅ Формы (login, deposit, bet placement)
  ✅ Real-time компоненты (odds, balance)
  ✅ WebSocket подписки
  ✅ Zustand/TanStack Query хуки
  ✅ Browser API (localStorage, geolocation)