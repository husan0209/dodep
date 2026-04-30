'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  HomeIcon,
  TrophyIcon,
  Squares2X2Icon,
  ClipboardDocumentListIcon,
  UserIcon,
} from '@heroicons/react/24/outline'

export function MobileNav() {
  const pathname = usePathname()

  // TODO: wire to bet slip store for real count
  const betSlipCount = 0

  const navigation = [
    { name: 'Главная', href: '/', icon: HomeIcon, badge: null as number | null },
    { name: 'Спорт', href: '/sportsbook', icon: TrophyIcon, badge: null },
    { name: 'Казино', href: '/casino', icon: Squares2X2Icon, badge: null },
    { name: 'Купон', href: '/bets', icon: ClipboardDocumentListIcon, badge: betSlipCount },
    { name: 'Профиль', href: '/profile', icon: UserIcon, badge: null },
  ]

  return (
    <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-mobile-nav bg-[rgb(var(--bg-secondary)/0.98)] border-t border-[rgb(var(--border))] pb-safe backdrop-blur-xl">
      <div className="grid grid-cols-5 h-[60px]">
        {navigation.map((item) => {
          const isActive = pathname === item.href || (item.href !== '/' && pathname?.startsWith(item.href))
          return (
            <Link
              key={item.name}
              href={item.href}
              className={`flex flex-col items-center justify-center gap-0.5 transition-colors relative min-h-[44px] ${
                isActive ? 'text-cyan-400' : 'text-gray-500'
              }`}
            >
              <item.icon className="h-5 w-5" />
              <span className="text-xs font-medium">{item.name}</span>
              {item.badge !== null && item.badge > 0 && (
                <span className="absolute top-1 right-2 bg-[rgb(var(--text-primary))] text-[rgb(var(--bg-primary))] text-[11px] font-bold rounded-full min-w-[18px] h-[18px] flex items-center justify-center px-1">
                  {item.badge}
                </span>
              )}
            </Link>
          )
        })}
      </div>
    </nav>
  )
}
