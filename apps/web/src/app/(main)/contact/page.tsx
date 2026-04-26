import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Контакты | DOD',
  description: 'Страница обратной связи DOD.'
}

export default function ContactPage() {
  return (
    <div className="mx-auto max-w-[1440px] px-3 py-6 space-y-6">
      <h1 className="text-2xl font-bold tracking-tight text-white mb-4">Контакты</h1>
      <div className="bg-[rgb(var(--bg-secondary))] rounded-lg border border-[rgb(var(--border))] p-6 space-y-4">
        <p className="text-gray-300">Служба поддержки DOD всегда готова прийти вам на помощь.</p>
        <p className="text-gray-300">
          <strong>Email:</strong> support@dod.casino
        </p>
      </div>
    </div>
  )
}
