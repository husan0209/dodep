import type { Metadata } from 'next'
import { Inter, Montserrat } from 'next/font/google'
import './globals.css'
import { Providers } from '@components/providers'
import { Header } from '@components/layout/header'
import { Footer } from '@components/layout/footer'
import { MobileNav } from '@components/layout/mobile-nav'

const inter = Inter({ 
  subsets: ['latin', 'cyrillic'],
  variable: '--font-inter',
})

const montserrat = Montserrat({ 
  subsets: ['latin', 'cyrillic'],
  variable: '--font-montserrat',
})

export const metadata: Metadata = {
  title: 'Opus Casino - Премиум платформа для ставок и игр',
  description: 'Делайте ставки на спорт, играйте в казино игры и получайте бонусы на лучшей игровой платформе.',
  keywords: ['казино', 'ставки', 'спорт', 'игры', 'бонусы'],
  authors: [{ name: 'Opus Casino Team' }],
  openGraph: {
    title: 'Opus Casino',
    description: 'Премиум платформа для ставок и игр',
    type: 'website',
    locale: 'ru_RU',
    siteName: 'Opus Casino',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Opus Casino',
    description: 'Премиум платформа для ставок и игр',
  },
  robots: {
    index: false, // Закрыто от индексации в разработке
    follow: false,
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <body className={`${inter.variable} ${montserrat.variable} font-sans antialiased`}>
        <Providers>
          <div className="min-h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
            <Header />
            <main className="flex-1">
              {children}
            </main>
            <Footer />
            <MobileNav />
          </div>
        </Providers>
      </body>
    </html>
  )
}
