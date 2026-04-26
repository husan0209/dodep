import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'FAQ | DOD',
  description: 'Часто задаваемые вопросы платформы DOD.'
}

export default function FAQPage() {
  return (
    <div className="mx-auto max-w-[1440px] px-3 py-6 space-y-6">
      <h1 className="text-2xl font-bold tracking-tight text-white mb-4">Часто задаваемые вопросы (FAQ)</h1>
      <div className="bg-[rgb(var(--bg-secondary))] rounded-lg border border-[rgb(var(--border))] p-6 space-y-4">
        <p className="text-gray-300">В этом разделе мы собрали для вас ответы на самые популярные вопросы.</p>
        <p className="text-gray-300">Данный раздел пока в разработке и скоро будет дополнен.</p>
      </div>
    </div>
  )
}
