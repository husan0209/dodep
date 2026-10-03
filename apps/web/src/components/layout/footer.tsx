'use client'

import Link from 'next/link'

const footerLinks = {
  platform: [
    { name: 'О нас', href: '/about' },
    { name: 'Правила', href: '/rules' },
    { name: 'Ответственная игра', href: '/responsible-gambling' },
  ],
  support: [
    { name: 'Помощь', href: '/support' },
    { name: 'FAQ', href: '/faq' },
    { name: 'Контакты', href: '/contact' },
  ],
  legal: [
    { name: 'Конфиденциальность', href: '/privacy' },
    { name: 'Условия', href: '/terms' },
  ],
}

export function Footer() {
  return (
    <footer className="mt-auto border-t border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))]">
      <div className="mx-auto max-w-[1440px] px-3 py-6">
        <div className="grid grid-cols-2 gap-6 md:grid-cols-4">
          <div className="col-span-2 md:col-span-1">
            <Link href="/" className="flex items-center gap-1">
              <span className="text-base font-bold text-blue-500">DOD</span>
            </Link>
            <p className="mt-2 text-[10px] leading-relaxed text-gray-600">
              Лицензированная платформа для ставок и игр.
            </p>
            <div className="mt-3 flex items-center gap-2">
              <span className="rounded bg-red-500/10 px-1.5 py-0.5 text-[10px] font-bold text-red-400">
                18+
              </span>
            </div>
          </div>

          {Object.entries(footerLinks).map(([key, links]) => (
            <div key={key}>
              <h3 className="mb-2 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
                {key === 'platform' ? 'Платформа' : key === 'support' ? 'Поддержка' : 'Информация'}
              </h3>
              <ul className="space-y-1">
                {links.map((link) => (
                  <li key={link.name}>
                    <Link
                      href={link.href}
                      className="text-[10px] text-gray-600 transition-colors hover:text-gray-400"
                    >
                      {link.name}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-4 flex flex-col items-center justify-between gap-2 border-t border-[rgb(var(--border))] pt-3 sm:flex-row">
          <p className="text-[10px] text-gray-600">© {new Date().getFullYear()} DOD</p>
          <p className="text-[10px] text-gray-700">Азартные игры могут вызывать зависимость</p>
        </div>
      </div>
    </footer>
  )
}
