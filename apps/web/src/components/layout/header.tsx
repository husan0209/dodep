'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useState } from 'react'
import {
  Bars3Icon,
  XMarkIcon,
  UserCircleIcon,
  StarIcon,
  TrophyIcon,
  Squares2X2Icon,
  WalletIcon,
  GiftIcon,
  UserGroupIcon,
  FireIcon,
  CurrencyDollarIcon,
} from '@heroicons/react/24/outline'
import { useAuthStore } from '@stores/auth-store'
import { cn } from '@/lib/cn'

const mainNav = [
  { name: 'Спорт', href: '/sportsbook', icon: TrophyIcon, badge: null as string | null },
  { name: 'Казино', href: '/casino', icon: Squares2X2Icon, badge: null },
  { name: 'Live', href: '/casino?tab=live', icon: FireIcon, badge: 'LIVE' },
  { name: 'Избранное', href: '/casino?tab=favorites', icon: StarIcon, badge: null },
]

const secondaryNav = [
  { name: 'Кошелёк', href: '/wallet', icon: WalletIcon, badge: null as string | null },
  { name: 'Бонусы', href: '/bonuses', icon: GiftIcon, badge: null as string | null },
  { name: 'Affiliate', href: '/affiliate', icon: UserGroupIcon, badge: null as string | null },
]

export function Header() {
  const pathname = usePathname()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const { user, isAuthenticated } = useAuthStore()

  return (
    <header className="sticky top-0 z-50">
      {/* Glassmorphism top bar */}
      <div className="border-b border-border/60 bg-bg-primary/80 backdrop-blur-xl backdrop-saturate-150">
        <div className="mx-auto max-w-[1440px] px-4">
          <div className="flex h-14 items-center justify-between gap-4">
            {/* Logo */}
            <Link href="/" className="group flex shrink-0 items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-yellow-400 to-amber-500 shadow-glow-gold-sm transition-shadow duration-300 group-hover:shadow-glow-gold">
                <CurrencyDollarIcon className="h-5 w-5 text-slate-950" />
              </div>
              <span className="text-gradient-gold hidden text-xl font-bold tracking-tight sm:block">
                DOD
              </span>
            </Link>

            {/* Main nav - pill style */}
            <nav className="hidden items-center gap-1 rounded-2xl border border-border/40 bg-bg-secondary/60 p-1 lg:flex">
              {mainNav.map((item) => {
                const isActive =
                  pathname === item.href ||
                  (item.href !== '/' && pathname?.startsWith(item.href.split('?')[0]))
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={cn(
                      'relative flex items-center gap-1.5 rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-200',
                      isActive
                        ? 'bg-bg-tertiary text-white shadow-sm'
                        : 'text-text-secondary hover:bg-white/5 hover:text-text-primary',
                    )}
                  >
                    <item.icon className={cn('h-4 w-4', item.badge === 'LIVE' && 'text-red-400')} />
                    {item.name}
                    {item.badge === 'LIVE' && (
                      <span className="absolute -right-0.5 -top-0.5 flex h-2 w-2">
                        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-400 opacity-75"></span>
                        <span className="relative inline-flex h-2 w-2 rounded-full bg-red-500"></span>
                      </span>
                    )}
                  </Link>
                )
              })}
            </nav>

            {/* Right side */}
            <div className="flex items-center gap-1.5">
              {secondaryNav.map((item) => (
                <Link
                  key={item.name}
                  href={item.href}
                  className="hidden items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium text-text-secondary transition-all duration-200 hover:bg-white/5 hover:text-text-primary lg:flex"
                >
                  <item.icon className="h-4 w-4" />
                  {item.name}
                </Link>
              ))}

              {/* Auth */}
              {isAuthenticated ? (
                <div className="flex items-center gap-2">
                  {/* Balance display */}
                  <Link
                    href="/wallet"
                    className="hidden items-center gap-1.5 rounded-xl border border-border/40 bg-bg-secondary/60 px-3 py-1.5 transition-all duration-200 hover:border-border-light/60 md:flex"
                  >
                    <WalletIcon className="h-4 w-4 text-text-muted" />
                    <span className="font-mono text-sm font-bold tabular-nums text-text-primary">
                      {'balance' in (user || {}) &&
                      (user as unknown as { balance?: number }).balance
                        ? (user as unknown as { balance: number }).balance.toLocaleString('ru-RU', {
                            minimumFractionDigits: 2,
                          })
                        : '0.00'}
                    </span>
                    <span className="text-xs text-text-muted">₽</span>
                  </Link>

                  <Link
                    href="/wallet"
                    className="btn-primary hidden px-4 py-2 text-xs shadow-glow-gold-sm sm:inline-flex"
                  >
                    Депозит
                  </Link>

                  <Link
                    href="/profile"
                    className="flex items-center gap-1.5 rounded-xl px-2.5 py-2 text-text-secondary transition-all duration-200 hover:bg-white/5 hover:text-text-primary"
                  >
                    <div className="flex h-7 w-7 items-center justify-center rounded-full bg-gradient-to-br from-violet-500 to-fuchsia-500 text-xs font-bold text-white">
                      {user?.username?.charAt(0).toUpperCase() || 'U'}
                    </div>
                    <span className="hidden text-sm font-medium lg:inline">
                      {user?.username || 'Профиль'}
                    </span>
                  </Link>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <Link
                    href="/login"
                    className="rounded-xl px-4 py-2 text-sm font-medium text-text-secondary transition-all duration-200 hover:bg-white/5 hover:text-text-primary"
                  >
                    Войти
                  </Link>
                  <Link
                    href="/register"
                    className="btn-primary px-4 py-2.5 text-xs shadow-glow-gold-sm"
                  >
                    Регистрация
                  </Link>
                </div>
              )}

              {/* Mobile menu */}
              <button
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="rounded-xl p-2 text-text-muted transition-all duration-200 hover:bg-white/5 hover:text-text-primary lg:hidden"
              >
                {mobileMenuOpen ? (
                  <XMarkIcon className="h-5 w-5" />
                ) : (
                  <Bars3Icon className="h-5 w-5" />
                )}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile menu - glassmorphism drawer */}
      {mobileMenuOpen && (
        <div className="animate-slide-down border-b border-border/60 bg-bg-secondary/95 backdrop-blur-2xl backdrop-saturate-150 lg:hidden">
          <div className="mx-auto max-w-[1440px] px-4 py-3">
            <div className="flex flex-col gap-1">
              {[...mainNav, ...secondaryNav].map((item) => (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={cn(
                    'flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-semibold transition-all duration-200',
                    pathname === item.href
                      ? 'bg-bg-tertiary text-white shadow-sm'
                      : 'text-text-secondary hover:bg-white/5 hover:text-text-primary',
                  )}
                >
                  <item.icon className={cn('h-5 w-5', item.badge === 'LIVE' && 'text-red-400')} />
                  {item.name}
                  {item.badge && (
                    <span className="badge badge-live ml-auto animate-pulse-fast text-[9px]">
                      {item.badge}
                    </span>
                  )}
                </Link>
              ))}
              {!isAuthenticated && (
                <div className="mt-2 flex gap-2 border-t border-border/40 pt-3">
                  <Link
                    href="/login"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex-1 rounded-xl border border-border py-2.5 text-center text-sm font-semibold text-text-secondary transition-colors hover:bg-white/5"
                  >
                    Войти
                  </Link>
                  <Link
                    href="/register"
                    onClick={() => setMobileMenuOpen(false)}
                    className="btn-primary flex-1 py-2.5 text-center text-sm"
                  >
                    Регистрация
                  </Link>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </header>
  )
}
