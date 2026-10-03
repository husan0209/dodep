import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Контакты | DOD',
  description: 'Страница обратной связи DOD.',
}

export default function ContactPage() {
  return (
    <div className="mx-auto max-w-[1440px] space-y-6 px-3 py-6">
      <h1 className="mb-4 text-2xl font-bold tracking-tight text-white">Контакты</h1>
      <div className="space-y-4 rounded-lg border border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))] p-6">
        <p className="text-gray-300">Служба поддержки DOD всегда готова прийти вам на помощь.</p>
        <p className="text-gray-300">
          <strong>Email:</strong> support@dod.casino
        </p>
      </div>
    </div>
  )
}
