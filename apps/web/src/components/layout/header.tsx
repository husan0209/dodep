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
  { name: 'Live', href: '/casino/live', icon: FireIcon, badge: 'LIVE' },
  { name: 'Избранное', href: '/casino/favorites', icon: StarIcon, badge: null },
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
    <header className="sticky top-0 z-header">
      {/* Glassmorphism top bar */}
      <div className="bg-bg-primary/80 backdrop-blur-xl backdrop-saturate-150 border-b border-border/60">
        <div className="mx-auto max-w-[1440px] px-4">
          <div className="flex h-14 items-center justify-between gap-4">
            {/* Logo */}
            <Link href="/" className="flex items-center gap-2 shrink-0 group">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-slate-100 to-cyan-200 flex items-center justify-center shadow-glow-frost-sm group-hover:shadow-glow-frost transition-shadow duration-300">
                <span className="font-display text-sm font-bold text-slate-950">O</span>
              </div>
              <span className="text-xl font-bold tracking-tight text-gradient-frost hidden sm:block">
                <span className="text-cyan-300">O</span>PUS
              </span>
            </Link>

            {/* Main nav - pill style */}
            <nav className="hidden lg:flex items-center gap-1 bg-bg-secondary/60 rounded-2xl p-1 border border-border/40">
              {mainNav.map((item) => {
                const isActive = pathname === item.href || (item.href !== '/' && pathname?.startsWith(item.href.split('?')[0]))
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={cn(
                      'relative flex items-center gap-1.5 px-4 py-2 text-sm font-semibold rounded-xl transition-all duration-200',
                      isActive
                        ? 'text-white bg-bg-tertiary shadow-sm'
                        : 'text-text-secondary hover:text-text-primary hover:bg-white/5'
                    )}
                  >
                    <item.icon className={cn('h-4 w-4', item.badge === 'LIVE' && 'text-red-400')} />
                    {item.name}
                    {item.badge === 'LIVE' && (
                      <span className="absolute -top-0.5 -right-0.5 flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
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
                  className="hidden lg:flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-text-secondary hover:text-text-primary hover:bg-white/5 rounded-xl transition-all duration-200"
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
                    className="hidden md:flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-bg-secondary/60 border border-border/40 hover:border-border-light/60 transition-all duration-200"
                  >
                    <WalletIcon className="h-4 w-4 text-text-muted" />
                    <span className="text-sm font-bold text-text-primary font-mono tabular-nums">
                      {'balance' in (user || {}) && (user as unknown as { balance?: number }).balance
                        ? (user as unknown as { balance: number }).balance.toLocaleString('ru-RU', { minimumFractionDigits: 2 })
                        : '0.00'}
                    </span>
                    <span className="text-xs text-text-muted">₽</span>
                  </Link>

                  <Link href="/wallet" className="btn-primary text-xs px-4 py-2 shadow-glow-frost-sm hidden sm:inline-flex">
                    Депозит
                  </Link>

                  <Link
                    href="/profile"
                    className="flex items-center gap-1.5 px-2.5 py-2 rounded-xl hover:bg-white/5 text-text-secondary hover:text-text-primary transition-all duration-200"
                  >
                    <div className="w-7 h-7 rounded-full bg-gradient-to-br from-violet-500 to-fuchsia-500 flex items-center justify-center text-white text-xs font-bold">
                      {user?.username?.charAt(0).toUpperCase() || 'U'}
                    </div>
                    <span className="hidden lg:inline text-sm font-medium">{user?.username || 'Профиль'}</span>
                  </Link>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <Link href="/login" className="px-4 py-2 text-sm font-medium text-text-secondary hover:text-text-primary hover:bg-white/5 rounded-xl transition-all duration-200">
                    Войти
                  </Link>
                  <Link href="/register" className="btn-primary text-xs px-4 py-2.5 shadow-glow-frost-sm">
                    Регистрация
                  </Link>
                </div>
              )}

              {/* Mobile menu */}
              <button
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="lg:hidden p-2 rounded-xl hover:bg-white/5 text-text-muted hover:text-text-primary transition-all duration-200"
              >
                {mobileMenuOpen ? <XMarkIcon className="h-5 w-5" /> : <Bars3Icon className="h-5 w-5" />}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile menu - glassmorphism drawer */}
      {mobileMenuOpen && (
        <div className="lg:hidden bg-bg-secondary/95 backdrop-blur-2xl backdrop-saturate-150 border-b border-border/60 animate-slide-down">
          <div className="mx-auto max-w-[1440px] px-4 py-3">
            <div className="flex flex-col gap-1">
              {[...mainNav, ...secondaryNav].map((item) => (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={cn(
                    'flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200',
                    pathname === item.href
                      ? 'text-white bg-bg-tertiary shadow-sm'
                      : 'text-text-secondary hover:text-text-primary hover:bg-white/5'
                  )}
                >
                  <item.icon className={cn('h-5 w-5', item.badge === 'LIVE' && 'text-red-400')} />
                  {item.name}
                  {item.badge && (
                    <span className="ml-auto badge badge-live animate-pulse-fast text-[11px]">
                      {item.badge}
                    </span>
                  )}
                </Link>
              ))}
              {!isAuthenticated && (
                <div className="flex gap-2 pt-3 mt-2 border-t border-border/40">
                  <Link
                    href="/login"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex-1 text-center py-2.5 text-sm font-semibold border border-border rounded-xl hover:bg-white/5 transition-colors text-text-secondary"
                  >
                    Войти
                  </Link>
                  <Link
                    href="/register"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex-1 text-center btn-primary text-sm py-2.5"
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
