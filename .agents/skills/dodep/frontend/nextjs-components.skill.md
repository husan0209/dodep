#26 nextjs-components.skill.md
Markdown

# nextjs-components.skill.md

## РОЛЬ
Ты — Senior Frontend Developer, создающий UI-компоненты для
гемблинг-платформы на Next.js 14 + TypeScript + Tailwind CSS + Radix UI.

## КОНТЕКСТ
- Платформа: онлайн-гемблинг, 10M+ пользователей
- Компоненты должны быть accessible (WCAG 2.1 AA)
- Mobile-first подход (60%+ трафика с мобильных)
- Real-time обновления через WebSocket (odds, баланс)
- Интернационализация: 20+ языков (next-intl)

## СТРУКТУРА КОМПОНЕНТОВ
src/
├── components/
│ ├── ui/ # Базовые UI-примитивы (atoms)
│ │ ├── Button/
│ │ │ ├── Button.tsx
│ │ │ ├── Button.test.tsx
│ │ │ └── index.ts
│ │ ├── Input/
│ │ ├── Badge/
│ │ ├── Modal/
│ │ ├── Skeleton/
│ │ └── Tooltip/
│ │
│ ├── shared/ # Переиспользуемые бизнес-компоненты
│ │ ├── OddsButton/
│ │ ├── BalanceDisplay/
│ │ ├── CurrencyAmount/
│ │ ├── CountdownTimer/
│ │ ├── GameCard/
│ │ └── UserAvatar/
│ │
│ ├── features/ # Крупные фича-компоненты
│ │ ├── BetSlip/
│ │ ├── SportsSidebar/
│ │ ├── LiveScoreboard/
│ │ ├── CasinoGrid/
│ │ ├── DepositForm/
│ │ └── KYCWidget/
│ │
│ └── layouts/ # Layout-компоненты
│ ├── MainLayout.tsx
│ ├── AuthLayout.tsx
│ ├── AdminLayout.tsx
│ └── MobileNav.tsx

text


## ПРАВИЛА СОЗДАНИЯ КОМПОНЕНТОВ

### Правило 1: Один файл — один компонент
```tsx
// ✅ ПРАВИЛЬНО: компонент в отдельной папке
// components/ui/Button/Button.tsx
import { forwardRef } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cn } from '@/lib/utils';
import type { ButtonProps } from './types';

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', size = 'md', 
     asChild = false, isLoading, disabled, children, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';
    
    return (
      <Comp
        ref={ref}
        className={cn(
          'inline-flex items-center justify-center rounded-lg font-medium',
          'transition-colors focus-visible:outline-none focus-visible:ring-2',
          'disabled:pointer-events-none disabled:opacity-50',
          variants[variant],
          sizes[size],
          className
        )}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading && <Spinner className="mr-2 h-4 w-4" />}
        {children}
      </Comp>
    );
  }
);
Button.displayName = 'Button';
Правило 2: Типы отдельно от компонента
React

// components/ui/Button/types.ts
export interface ButtonProps 
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'link';
  size?: 'sm' | 'md' | 'lg';
  isLoading?: boolean;
  asChild?: boolean;
}
Правило 3: Barrel export
React

// components/ui/Button/index.ts
export { Button } from './Button';
export type { ButtonProps } from './types';
ПАТТЕРНЫ ДЛЯ ГЕМБЛИНГ-КОМПОНЕНТОВ
OddsButton — анимация изменения коэффициентов
React

// components/shared/OddsButton/OddsButton.tsx
'use client';

import { useEffect, useRef, useState } from 'react';
import { cn } from '@/lib/utils';

interface OddsButtonProps {
  odds: number;
  previousOdds?: number;
  label: string;
  isSelected: boolean;
  isSuspended: boolean;
  onClick: () => void;
}

export function OddsButton({ 
  odds, previousOdds, label, isSelected, isSuspended, onClick 
}: OddsButtonProps) {
  const [flash, setFlash] = useState<'up' | 'down' | null>(null);
  const prevOddsRef = useRef(odds);

  useEffect(() => {
    if (prevOddsRef.current !== odds) {
      setFlash(odds > prevOddsRef.current ? 'up' : 'down');
      prevOddsRef.current = odds;
      const timer = setTimeout(() => setFlash(null), 2000);
      return () => clearTimeout(timer);
    }
  }, [odds]);

  return (
    <button
      onClick={onClick}
      disabled={isSuspended}
      aria-pressed={isSelected}
      aria-label={`${label}: ${odds}`}
      className={cn(
        'flex flex-col items-center px-3 py-2 rounded-lg border',
        'transition-all duration-200 min-w-[64px]',
        isSelected && 'bg-brand-500 text-white border-brand-600',
        !isSelected && 'bg-surface-secondary hover:bg-surface-tertiary',
        isSuspended && 'opacity-50 cursor-not-allowed',
        flash === 'up' && 'animate-flash-green',
        flash === 'down' && 'animate-flash-red',
      )}
    >
      <span className="text-xs text-muted truncate max-w-full">
        {label}
      </span>
      <span className="text-sm font-bold tabular-nums">
        {odds.toFixed(2)}
      </span>
    </button>
  );
}
CurrencyAmount — отображение денег
React

// components/shared/CurrencyAmount/CurrencyAmount.tsx
import { useLocale } from 'next-intl';

interface CurrencyAmountProps {
  amount: number;
  currency: string;
  showSign?: boolean;         // +100.00 / -50.00
  colorize?: boolean;         // зелёный/красный по знаку
  compact?: boolean;          // 1.2K вместо 1,200
  className?: string;
}

export function CurrencyAmount({
  amount, currency, showSign, colorize, compact, className
}: CurrencyAmountProps) {
  const locale = useLocale();
  
  const formatted = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    notation: compact ? 'compact' : 'standard',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.abs(amount));
  
  const sign = amount > 0 ? '+' : amount < 0 ? '-' : '';
  
  return (
    <span
      className={cn(
        'tabular-nums font-medium',
        colorize && amount > 0 && 'text-green-500',
        colorize && amount < 0 && 'text-red-500',
        className
      )}
      aria-label={`${sign}${formatted}`}
    >
      {showSign && sign}{formatted}
    </span>
  );
}
BalanceDisplay — real-time баланс
React

// components/shared/BalanceDisplay/BalanceDisplay.tsx
'use client';

import { useUserBalance } from '@/hooks/useUserBalance';
import { CurrencyAmount } from '../CurrencyAmount';
import { Skeleton } from '@/components/ui/Skeleton';

export function BalanceDisplay() {
  const { balance, currency, isLoading } = useUserBalance();
  
  if (isLoading) {
    return <Skeleton className="h-6 w-24" />;
  }
  
  return (
    <div className="flex items-center gap-1">
      <WalletIcon className="h-4 w-4 text-muted" />
      <CurrencyAmount 
        amount={balance} 
        currency={currency} 
      />
    </div>
  );
}
АНТИПАТТЕРНЫ
React

// ❌ ПЛОХО: бизнес-логика в компоненте
function BetSlip() {
  const [bets, setBets] = useState([]);
  // 100 строк логики расчёта ставок...
  const calculateOdds = () => { /* ... */ };
  const validateBet = () => { /* ... */ };
}

// ✅ ПРАВИЛЬНО: логика в хуке, компонент только отображение
function BetSlip() {
  const { bets, totalOdds, potentialWin, placeBet, removeBet } = useBetSlip();
  // Только JSX рендеринг
}

// ❌ ПЛОХО: инлайн стили и магические числа
<div style={{ marginTop: 16, padding: '12px 24px' }}>

// ✅ ПРАВИЛЬНО: Tailwind + семантические классы
<div className="mt-4 px-6 py-3">

// ❌ ПЛОХО: отсутствие loading/error states
function GameGrid() {
  const { data } = useGames();
  return data.map(game => <GameCard key={game.id} game={game} />);
}

// ✅ ПРАВИЛЬНО: все состояния обработаны
function GameGrid() {
  const { data, isLoading, isError, error } = useGames();
  
  if (isLoading) return <GameGridSkeleton count={12} />;
  if (isError) return <ErrorState message={error.message} onRetry={refetch} />;
  if (!data?.length) return <EmptyState icon={GameIcon} message="No games" />;
  
  return data.map(game => <GameCard key={game.id} game={game} />);
}

// ❌ ПЛОХО: нет aria-атрибутов
<button onClick={toggleMenu}>☰</button>

// ✅ ПРАВИЛЬНО: accessible
<button 
  onClick={toggleMenu}
  aria-label="Toggle navigation menu"
  aria-expanded={isOpen}
>
  <MenuIcon aria-hidden="true" />
</button>
ПРАВИЛА TAILWIND
React

// Используй cn() для условных классов (clsx + tailwind-merge)
import { cn } from '@/lib/utils';

// lib/utils.ts
import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Порядок классов в Tailwind:
// layout → sizing → spacing → typography → colors → effects → states
className="flex items-center w-full px-4 py-2 text-sm text-white bg-brand-500 
           rounded-lg shadow-sm hover:bg-brand-600 focus:ring-2"
SKELETON / LOADING PATTERNS
React

// Для каждого компонента создавай Skeleton-версию
export function GameCardSkeleton() {
  return (
    <div className="animate-pulse rounded-lg bg-surface-secondary p-4">
      <div className="aspect-video rounded bg-surface-tertiary mb-3" />
      <div className="h-4 w-3/4 rounded bg-surface-tertiary mb-2" />
      <div className="h-3 w-1/2 rounded bg-surface-tertiary" />
    </div>
  );
}

export function GameGridSkeleton({ count = 12 }: { count?: number }) {
  return (
    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
      {Array.from({ length: count }, (_, i) => (
        <GameCardSkeleton key={i} />
      ))}
    </div>
  );
}
RESPONSIVE DESIGN
text

Breakpoints (mobile-first):
  sm:  640px   — small tablets
  md:  768px   — tablets
  lg:  1024px  — desktop
  xl:  1280px  — wide desktop
  2xl: 1536px  — ultra-wide

Обязательно:
  - Все компоненты работают от 320px ширины
  - Touch targets: минимум 44x44px на мобильных
  - Swipe gestures для мобильных (bet slip, menus)
  - Не используй hover-only интерактивность
ТЕСТИРОВАНИЕ КОМПОНЕНТОВ
React

// components/shared/OddsButton/OddsButton.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { OddsButton } from './OddsButton';

describe('OddsButton', () => {
  it('renders odds value and label', () => {
    render(
      <OddsButton odds={1.95} label="Home" isSelected={false}
                  isSuspended={false} onClick={() => {}} />
    );
    expect(screen.getByText('1.95')).toBeInTheDocument();
    expect(screen.getByText('Home')).toBeInTheDocument();
  });

  it('calls onClick when not suspended', () => {
    const onClick = vi.fn();
    render(
      <OddsButton odds={1.95} label="Home" isSelected={false}
                  isSuspended={false} onClick={onClick} />
    );
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('disables click when suspended', () => {
    const onClick = vi.fn();
    render(
      <OddsButton odds={1.95} label="Home" isSelected={false}
                  isSuspended={true} onClick={onClick} />
    );
    fireEvent.click(screen.getByRole('button'));
    expect(onClick).not.toHaveBeenCalled();
  });

  it('has aria-pressed when selected', () => {
    render(
      <OddsButton odds={1.95} label="Home" isSelected={true}
                  isSuspended={false} onClick={() => {}} />
    );
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'true');
  });
});
PERFORMANCE ПРАВИЛА
text

1. Используй React.memo() для компонентов, получающих 
   стабильные пропсы, но рендерящихся в списке (GameCard, OddsButton)
2. Используй useCallback/useMemo только при реальной необходимости
3. Для списков > 50 элементов — виртуализация (@tanstack/react-virtual)
4. Изображения: next/image с blur placeholder и lazy loading
5. Динамический импорт для тяжёлых компонентов:
   const Chart = dynamic(() => import('./Chart'), { ssr: false })
6. Не импортируй иконки целой библиотекой — только конкретные