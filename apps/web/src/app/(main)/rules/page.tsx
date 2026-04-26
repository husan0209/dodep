import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Правила | DOD',
  description: 'Правила платформы DOD.'
}

export default function RulesPage() {
  return (
    <div className="mx-auto max-w-[1440px] px-3 py-6 space-y-6">
      <h1 className="text-2xl font-bold tracking-tight text-white mb-4">Правила платформы</h1>
      <div className="bg-[rgb(var(--bg-secondary))] rounded-lg border border-[rgb(var(--border))] p-6 space-y-4">
        <p className="text-gray-300">Ниже приведены основные правила использования платформы DOD.</p>
        <h2 className="text-xl font-bold text-white mt-6">Общие положения</h2>
        <p className="text-gray-300">Текст правил находится в процессе наполнения. Играйте честно и соблюдайте все местные законы.</p>
      </div>
    </div>
  )
}
