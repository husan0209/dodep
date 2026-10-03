import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Правила | DOD',
  description: 'Правила платформы DOD.',
}

export default function RulesPage() {
  return (
    <div className="mx-auto max-w-[1440px] space-y-6 px-3 py-6">
      <h1 className="mb-4 text-2xl font-bold tracking-tight text-white">Правила платформы</h1>
      <div className="space-y-4 rounded-lg border border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))] p-6">
        <p className="text-gray-300">
          Ниже приведены основные правила использования платформы DOD.
        </p>
        <h2 className="mt-6 text-xl font-bold text-white">Общие положения</h2>
        <p className="text-gray-300">
          Текст правил находится в процессе наполнения. Играйте честно и соблюдайте все местные
          законы.
        </p>
      </div>
    </div>
  )
}
