import Link from 'next/link'

const footerLinks = {
  platform: [
    { name: 'О нас', href: '/about' },
    { name: 'Правила', href: '/rules' },
    { name: 'Ответственная игра', href: '/responsible-gambling' },
    { name: 'Партнёрам', href: '/affiliates' },
  ],
  support: [
    { name: 'Помощь', href: '/support' },
    { name: 'FAQ', href: '/faq' },
    { name: 'Контакты', href: '/contact' },
  ],
  legal: [
    { name: 'Конфиденциальность', href: '/privacy' },
    { name: 'Условия использования', href: '/terms' },
    { name: 'KYC/AML', href: '/kyc-aml' },
  ],
}

export function Footer() {
  return (
    <footer className="bg-white dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
          {/* Brand */}
          <div className="col-span-2 md:col-span-1">
            <Link href="/" className="flex items-center space-x-2">
              <span className="text-xl font-bold font-display text-primary-600 dark:text-primary-400">
                OPUS
              </span>
              <span className="text-xl font-bold font-display text-gray-900 dark:text-white">
                CASINO
              </span>
            </Link>
            <p className="mt-4 text-sm text-gray-500 dark:text-gray-400">
              Премиум платформа для ставок и игр с лучшими коэффициентами.
            </p>
            <div className="mt-4 flex items-center space-x-2">
              <span className="text-xs text-gray-400">18+</span>
              <span className="text-xs text-gray-400">•</span>
              <span className="text-xs text-gray-400">Играйте ответственно</span>
            </div>
          </div>

          {/* Platform links */}
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Платформа
            </h3>
            <ul className="mt-4 space-y-3">
              {footerLinks.platform.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href}
                    className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {/* Support links */}
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Поддержка
            </h3>
            <ul className="mt-4 space-y-3">
              {footerLinks.support.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href}
                    className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {/* Legal links */}
          <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
              Правовая информация
            </h3>
            <ul className="mt-4 space-y-3">
              {footerLinks.legal.map((link) => (
                <li key={link.name}>
                  <Link
                    href={link.href}
                    className="text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-white transition-colors"
                  >
                    {link.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <div className="mt-12 pt-8 border-t border-gray-200 dark:border-gray-700">
          <p className="text-sm text-gray-400 text-center">
            © {new Date().getFullYear()} Opus Casino. Все права защищены.
          </p>
          <p className="mt-2 text-xs text-gray-500 text-center">
            Азартные игры могут вызывать зависимость. Пожалуйста, играйте ответственно.
          </p>
        </div>
      </div>
    </footer>
  )
}
