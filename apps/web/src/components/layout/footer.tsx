'use client'

import Link from 'next/link'

const footerLinks = {
  platform: [
    { name: 'О нас', href: '/about' },
    { name: 'Правила', href: '/rules' },
    { name: 'Ответственная игра', href: '/responsible-gambling' },
  ],
  games: [
    { name: 'Казино', href: '/casino' },
    { name: 'Спорт', href: '/sportsbook' },
    { name: 'Live Casino', href: '/casino/live' },
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

const PaymentLogo = ({ name }: { name: string }) => (
  <span className="text-xs font-mono text-gray-600 bg-white/5 px-2 py-1 rounded border border-white/10">{name}</span>
)

export function Footer() {
  return (
    <footer className="bg-[rgb(var(--bg-secondary))] border-t border-[rgb(var(--border))] mt-auto">
      <div className="mx-auto max-w-[1440px] px-4 py-12">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
          <div className="col-span-2 md:col-span-1">
            <Link href="/" className="flex items-center gap-1">
              <span className="text-base font-bold text-cyan-300">O<span className="text-white">PUS</span></span>
            </Link>
            <p className="mt-2 text-xs text-gray-500 leading-relaxed">
              Лицензированная платформа для ставок и игр.
            </p>
            <div className="mt-3 flex items-center gap-2">
              <span className="text-xs font-bold text-red-400 bg-red-500/10 px-2 py-0.5 rounded border border-red-500/20">18+</span>
              <span className="text-xs text-gray-600">Лицензия № MGA/B2C/000/0000</span>
            </div>
          </div>

          {Object.entries(footerLinks).map(([key, links]) => (
            <div key={key}>
              <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
                {key === 'platform' ? 'О платформе' : key === 'games' ? 'Игры' : key === 'support' ? 'Поддержка' : 'Информация'}
              </h3>
              <ul className="space-y-2">
                {links.map((link) => (
                  <li key={link.name}>
                    <Link href={link.href} className="text-xs text-gray-500 hover:text-gray-300 transition-colors">
                      {link.name}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-8 pt-4 border-t border-[rgb(var(--border))]">
          <div className="flex flex-wrap items-center gap-2 mb-3">
            <PaymentLogo name="Visa" />
            <PaymentLogo name="Mastercard" />
            <PaymentLogo name="BTC" />
            <PaymentLogo name="ETH" />
            <PaymentLogo name="USDT" />
          </div>
          <div className="flex flex-col sm:flex-row items-center justify-between gap-2">
            <p className="text-xs text-gray-600">© {new Date().getFullYear()} OPUS Casino</p>
            <p className="text-xs text-gray-700">Азартные игры могут вызывать зависимость</p>
          </div>
        </div>
      </div>
    </footer>
  )
}
