import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Ответственная игра | DOD',
  description: 'Политика ответственной игры на платформе DOD.',
}

export default function ResponsibleGamblingPage() {
  return (
    <div className="mx-auto max-w-[1440px] space-y-6 px-3 py-6">
      <h1 className="mb-4 text-2xl font-bold tracking-tight text-white">Ответственная игра</h1>
      <div className="space-y-4 rounded-lg border border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))] p-6">
        <p className="text-gray-300">
          Мы заботимся о наших игроках и призываем к ответственной игре.
        </p>
        <h2 className="mt-6 text-xl font-bold text-white">Контроль и ограничения</h2>
        <p className="text-gray-300">
          Азартные игры предназначены исключительно для развлечения. Пожалуйста, используйте функции
          самоконтроля и установки лимитов для безопасной игры.
        </p>
      </div>
    </div>
  )
}
