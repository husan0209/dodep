import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Ответственная игра | DOD',
  description: 'Политика ответственной игры на платформе DOD.'
}

export default function ResponsibleGamblingPage() {
  return (
    <div className="mx-auto max-w-[1440px] px-3 py-6 space-y-6">
      <h1 className="text-2xl font-bold tracking-tight text-white mb-4">Ответственная игра</h1>
      <div className="bg-[rgb(var(--bg-secondary))] rounded-lg border border-[rgb(var(--border))] p-6 space-y-4">
        <p className="text-gray-300">Мы заботимся о наших игроках и призываем к ответственной игре.</p>
        <h2 className="text-xl font-bold text-white mt-6">Контроль и ограничения</h2>
        <p className="text-gray-300">Азартные игры предназначены исключительно для развлечения. Пожалуйста, используйте функции самоконтроля и установки лимитов для безопасной игры.</p>
      </div>
    </div>
  )
}
