## #32 typescript-shared.skill.md

```markdown
# typescript-shared.skill.md

## РОЛЬ
Ты определяешь общие TypeScript конвенции для всех frontend-проектов
гемблинг-платформы (Next.js, Admin Panel, shared libraries).

## КОНТЕКСТ
- TypeScript strict mode ВСЕГДА
- Общие типы между Web и Admin
- API типы генерируются из Protobuf/OpenAPI
- Runtime validation: zod

## TSCONFIG

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": false,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "forceConsistentCasingInFileNames": true,
    "verbatimModuleSyntax": true,
    "target": "ES2022",
    "moduleResolution": "bundler",
    "paths": {
      "@/*": ["./src/*"]
    }
  }
}
ТИПЫ — ПРАВИЛА
Правило 1: Interface для объектов, Type для union/utility
React

// ✅ ПРАВИЛЬНО: interface для data shapes
interface User {
  id: number;
  email: string;
  status: UserStatus;
  createdAt: string;
}

// ✅ ПРАВИЛЬНО: type для unions и утилит
type UserStatus = 'active' | 'blocked' | 'pending' | 'self_excluded';
type BetType = 'single' | 'accumulator' | 'system';

// ✅ ПРАВИЛЬНО: type для computed/utility types
type UserWithBalance = User & { balance: WalletBalance };
type CreateUserInput = Omit<User, 'id' | 'createdAt'>;
type PartialUser = Partial<Pick<User, 'email' | 'status'>>;
Правило 2: Zod для runtime validation
React

// types/schemas/bet.ts
import { z } from 'zod';

export const placeBetSchema = z.object({
  betType: z.enum(['single', 'accumulator', 'system']),
  selections: z.array(z.object({
    eventId: z.number().positive(),
    marketId: z.number().positive(),
    outcomeId: z.number().positive(),
    odds: z.number().positive(),
  })).min(1).max(20),
  stake: z.number().positive().max(100_000),
  currency: z.string().length(3),
  acceptOddsChanges: z.enum(['none', 'higher', 'any']),
  idempotencyKey: z.string().uuid(),
});

// Тип выводится из schema — single source of truth
export type PlaceBetInput = z.infer<typeof placeBetSchema>;

// Использование в form:
const result = placeBetSchema.safeParse(formData);
if (!result.success) {
  // result.error.issues — массив ошибок валидации
  return;
}
// result.data — типизированные данные
Правило 3: Discriminated Unions для состояний
React

// ✅ ПРАВИЛЬНО: discriminated union
type AsyncState<T> =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; data: T }
  | { status: 'error'; error: string };

// Использование — TypeScript сужает тип
function renderState<T>(state: AsyncState<T>) {
  switch (state.status) {
    case 'idle':
      return null;
    case 'loading':
      return <Spinner />;
    case 'success':
      return <Data data={state.data} />;  // state.data доступен
    case 'error':
      return <Error message={state.error} />;  // state.error доступен
  }
}

// ✅ ПРАВИЛЬНО: для WebSocket сообщений
type WsMessage =
  | { type: 'odds_update'; eventId: number; odds: Record<string, number> }
  | { type: 'score_update'; eventId: number; score: Score }
  | { type: 'bet_settled'; betId: number; status: 'won' | 'lost' }
  | { type: 'balance_update'; available: number; locked: number };
Правило 4: Branded Types для ID
React

// types/branded.ts
// Prevent mixing up different IDs
declare const brand: unique symbol;
type Brand<T, B> = T & { [brand]: B };

export type UserId = Brand<number, 'UserId'>;
export type BetId = Brand<number, 'BetId'>;
export type EventId = Brand<number, 'EventId'>;
export type TransactionId = Brand<number, 'TransactionId'>;

// Функции-конструкторы
export const UserId = (id: number) => id as UserId;
export const BetId = (id: number) => id as BetId;

// Теперь нельзя случайно передать BetId вместо UserId
function getUser(id: UserId): Promise<User> { ... }

getUser(UserId(123));     // ✅
getUser(BetId(123));      // ❌ Type error!
getUser(123 as any);      // Обойти можно, но осознанно
Правило 5: Enum через as const
React

// ❌ ПЛОХО: TypeScript enum (проблемы с tree-shaking)
enum BetStatus {
  Pending = 'pending',
  Active = 'active',
  Won = 'won',
  Lost = 'lost',
}

// ✅ ПРАВИЛЬНО: as const object
export const BET_STATUS = {
  PENDING: 'pending',
  ACTIVE: 'active',
  WON: 'won',
  LOST: 'lost',
  VOID: 'void',
  CASHOUT: 'cashout',
} as const;

export type BetStatus = typeof BET_STATUS[keyof typeof BET_STATUS];
// type BetStatus = 'pending' | 'active' | 'won' | 'lost' | 'void' | 'cashout'
API RESPONSE TYPES
React

// types/api.ts
// Стандартный формат ответа от backend
interface ApiSuccessResponse<T> {
  data: T;
  meta?: PaginationMeta;
}

interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
    request_id: string;
  };
}

interface PaginationMeta {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
}

// Paginated response type
interface PaginatedResponse<T> {
  items: T[];
  meta: PaginationMeta;
}

// Type helper для API функций
type ApiResult<T> = Promise<ApiSuccessResponse<T>>;
UTILITY TYPES ДЛЯ ПРОЕКТА
React

// types/utils.ts

// Деньги всегда { amount, currency }
interface Money {
  amount: number;
  currency: string;
}

// Timestamp — ISO 8601 string
type ISOTimestamp = string;

// Pagination params
interface PaginationParams {
  page?: number;
  pageSize?: number;
  sortBy?: string;
  sortOrder?: 'asc' | 'desc';
}

// Filter params (для таблиц)
type FilterParams<T> = Partial<Record<keyof T, unknown>> & PaginationParams & {
  search?: string;
  dateFrom?: ISOTimestamp;
  dateTo?: ISOTimestamp;
};

// NonNullableFields — убрать null/undefined из всех полей
type NonNullableFields<T> = {
  [K in keyof T]: NonNullable<T[K]>;
};
АНТИПАТТЕРНЫ
React

// ❌ ПЛОХО: any
const data: any = await fetch('/api/user');

// ✅ ПРАВИЛЬНО: конкретный тип
const data: User = await api.get<User>('/api/user');

// ❌ ПЛОХО: type assertion без проверки
const user = response.data as User;

// ✅ ПРАВИЛЬНО: runtime validation
const user = userSchema.parse(response.data);

// ❌ ПЛОХО: необязательные поля без default
function Component({ title }: { title?: string }) {
  return <h1>{title.toUpperCase()}</h1>; // Runtime error!
}

// ✅ ПРАВИЛЬНО: default или проверка
function Component({ title = 'Untitled' }: { title?: string }) {
  return <h1>{title.toUpperCase()}</h1>;
}

// ❌ ПЛОХО: string для всего
interface Bet {
  status: string;   // что угодно
  type: string;     // что угодно
}

// ✅ ПРАВИЛЬНО: union types
interface Bet {
  status: BetStatus;  // 'pending' | 'active' | 'won' | ...
  type: BetType;      // 'single' | 'accumulator' | 'system'
}

// ❌ ПЛОХО: мутабельность
const selections: Selection[] = [];
selections.push(newSelection);  // мутация

// ✅ ПРАВИЛЬНО: иммутабельность
const newSelections = [...selections, newSelection];
// Или readonly
function processSelections(selections: readonly Selection[]) {
  // selections.push() — TS ошибка
}
ПРАВИЛА ИМПОРТОВ
React

// Порядок импортов (eslint-plugin-import):
// 1. React/framework
import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';

// 2. Внешние библиотеки
import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';

// 3. Внутренние модули (через alias @/)
import { api } from '@/lib/api/client';
import { useAuthStore } from '@/stores/authStore';

// 4. Компоненты
import { Button } from '@/components/ui/Button';

// 5. Типы (с type keyword)
import type { User, BetStatus } from '@/types';

// 6. Стили / ассеты
import './styles.css';
NAMING CONVENTIONS
text

Файлы:
  Компоненты:      PascalCase.tsx     (UserProfile.tsx)
  Хуки:            camelCase.ts       (useUserBalance.ts)
  Утилиты:         camelCase.ts       (formatCurrency.ts)
  Типы:            camelCase.ts       (userTypes.ts)
  Константы:       camelCase.ts       (appConstants.ts)
  Тесты:           *.test.ts(x)

Переменные/функции:
  Компоненты:      PascalCase         (UserProfile)
  Хуки:            use + PascalCase   (useUserBalance)
  Функции:         camelCase          (formatCurrency)
  Константы:       UPPER_SNAKE_CASE   (MAX_BET_AMOUNT)
  Типы/Interface:  PascalCase         (UserProfile)
  Enum values:     UPPER_SNAKE_CASE   (BET_STATUS.PENDING)
  Boolean:         is/has/can prefix  (isLoading, hasError, canPlace)