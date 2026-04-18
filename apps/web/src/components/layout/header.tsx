'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useTheme } from 'next-themes'
import { useState } from 'react'
import {
  Bars3Icon,
  XMarkIcon,
  SunIcon,
  MoonIcon,
  UserCircleIcon,
  StarIcon,
  TrophyIcon,
  Squares2X2Icon,
  WalletIcon,
  GiftIcon,
} from '@heroicons/react/24/outline'
import { useAuthStore } from '@stores/auth-store'

const mainNav = [
  { name: 'Спорт', href: '/sportsbook', icon: TrophyIcon },
  { name: 'Казино', href: '/casino', icon: Squares2X2Icon },
  { name: 'Live', href: '/casino?tab=live', icon: Squares2X2Icon },
  { name: 'Избранное', href: '/casino?tab=favorites', icon: StarIcon },
]

const secondaryNav = [
  { name: 'Кошелёк', href: '/wallet', icon: WalletIcon },
  { name: 'Бонусы', href: '/bonuses', icon: GiftIcon },
]

export function Header() {
  const pathname = usePathname()
  const { theme, setTheme } = useTheme()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const { user, isAuthenticated } = useAuthStore()

  return (
    <header className="sticky top-0 z-50">
      {/* Top bar */}
      <div className="bg-[rgb(var(--bg-primary))] border-b border-[rgb(var(--border))]">
        <div className="mx-auto max-w-[1440px] px-3">
          <div className="flex h-10 items-center justify-between">
            {/* Logo */}
            <Link href="/" className="flex items-center gap-1 shrink-0">
              <span className="text-lg font-bold text-blue-500 tracking-tight">OPUS</span>
              <span className="text-lg font-bold text-gray-300 tracking-tight">CASINO</span>
            </Link>

            {/* Main nav */}
            <nav className="hidden lg:flex items-center gap-0.5">
              {mainNav.map((item) => {
                const isActive = pathname === item.href || (item.href !== '/' && pathname?.startsWith(item.href.split('?')[0]))
                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded transition-colors ${
                      isActive
                        ? 'text-white bg-blue-600'
                        : 'text-gray-400 hover:text-white hover:bg-white/5'
                    }`}
                  >
                    <item.icon className="h-3.5 w-3.5" />
                    {item.name}
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
                  className="hidden sm:flex items-center gap-1 px-2 py-1 text-xs text-gray-300 hover:text-white hover:bg-white/5 rounded transition-colors"
                >
                  <item.icon className="h-3.5 w-3.5" />
                  {item.name}
                </Link>
              ))}

              {/* Theme toggle */}
              <button
                onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                className="p-1.5 rounded hover:bg-white/5 text-gray-500 transition-colors"
              >
                {theme === 'dark' ? <SunIcon className="h-4 w-4" /> : <MoonIcon className="h-4 w-4" />}
              </button>

              {/* Auth */}
              {isAuthenticated ? (
                <div className="flex items-center gap-1">
                  <Link href="/wallet" className="btn-primary text-xs px-2.5 py-1.5 hidden sm:inline-flex">
                    Депозит
                  </Link>
                  <Link
                    href="/profile"
                    className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-white/5 text-gray-300 transition-colors text-xs"
                  >
                    <UserCircleIcon className="h-4 w-4" />
                    <span className="hidden sm:inline">{user?.username || 'Профиль'}</span>
                  </Link>
                </div>
              ) : (
                <div className="flex items-center gap-1">
                  <Link href="/login" className="px-2 py-1 text-xs text-gray-300 hover:text-white transition-colors">
                    Войти
                  </Link>
                  <Link href="/register" className="btn-yellow text-xs">
                    Регистрация
                  </Link>
                </div>
              )}

              {/* Mobile menu */}
              <button
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="lg:hidden p-1.5 rounded hover:bg-white/5 text-gray-500 transition-colors"
              >
                {mobileMenuOpen ? <XMarkIcon className="h-4 w-4" /> : <Bars3Icon className="h-4 w-4" />}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="lg:hidden bg-[rgb(var(--bg-secondary))] border-b border-[rgb(var(--border))] fade-in">
          <div className="mx-auto max-w-[1440px] px-3 py-2">
            <div className="flex flex-col gap-0.5">
              {[...mainNav, ...secondaryNav].map((item) => (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={`flex items-center gap-2 px-3 py-2 rounded text-xs font-medium transition-colors ${
                    pathname === item.href
                      ? 'text-white bg-blue-600'
                      : 'text-gray-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <item.icon className="h-4 w-4" />
                  {item.name}
                </Link>
              ))}
              {!isAuthenticated && (
                <div className="flex gap-2 pt-2 mt-1 border-t border-[rgb(var(--border))]">
                  <Link
                    href="/login"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex-1 text-center px-3 py-1.5 text-xs border border-[rgb(var(--border))] rounded hover:bg-white/5 transition-colors text-gray-300"
                  >
                    Войти
                  </Link>
                  <Link
                    href="/register"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex-1 text-center btn-yellow text-xs"
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
