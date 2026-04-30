import type { Metadata } from 'next'
import { Inter, Montserrat } from 'next/font/google'
import './globals.css'
import { QueryProviders } from '@components/providers'
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
  title: 'DOD - Премиум платформа для ставок и игр',
  description: 'Делайте ставки на спорт, играйте в казино игры и получайте бонусы на платформе DOD.',
  keywords: ['казино', 'ставки', 'спорт', 'игры', 'бонусы'],
  authors: [{ name: 'DOD Team' }],
  openGraph: {
    title: 'DOD',
    description: 'Премиум платформа для ставок и игр',
    type: 'website',
    locale: 'ru_RU',
    siteName: 'DOD',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'DOD',
    description: 'Премиум платформа для ставок и игр',
  },
  robots: {
    index: false,
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
        <QueryProviders>
          <div className="min-h-screen flex flex-col">
            <Header />
            <main className="flex-1 pb-16 lg:pb-0">
              {children}
            </main>
            <Footer />
            <MobileNav />
          </div>
        </QueryProviders>
      </body>
    </html>
  )
}
