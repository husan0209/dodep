'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  TrophyIcon,
  Squares2X2Icon,
  CurrencyDollarIcon,
  UserIcon,
} from '@heroicons/react/24/outline'

export function MobileNav() {
  const pathname = usePathname()

  const navigation = [
    { name: 'Спорт', href: '/sportsbook', icon: TrophyIcon },
    { name: 'Казино', href: '/casino', icon: Squares2X2Icon },
    { name: 'Кошелёк', href: '/wallet', icon: CurrencyDollarIcon },
    { name: 'Профиль', href: '/profile', icon: UserIcon },
  ]

  return (
    <nav className="pb-safe fixed bottom-0 left-0 right-0 z-40 border-t border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))/0.98] backdrop-blur lg:hidden">
      <div className="grid h-12 grid-cols-4">
        {navigation.map((item) => {
          const isActive = pathname === item.href
          return (
            <Link
              key={item.name}
              href={item.href}
              className={`flex flex-col items-center justify-center gap-0.5 transition-colors ${
                isActive ? 'text-blue-400' : 'text-gray-500'
              }`}
            >
              <item.icon className="h-4 w-4" />
              <span className="text-[9px] font-medium">{item.name}</span>
            </Link>
          )
        })}
      </div>
    </nav>
  )
}
