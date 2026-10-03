'use client'

export function ProfilePage() {
  return (
    <div className="section max-w-2xl">
      <h1 className="mb-4 text-sm font-bold text-white">Профиль</h1>

      <div className="card mb-3 p-3">
        <h2 className="mb-3 text-xs font-semibold text-white">Личная информация</h2>
        <div className="space-y-2">
          {[
            { label: 'Email', value: 'user@example.com' },
            { label: 'Имя пользователя', value: 'Player123' },
            { label: 'Страна', value: 'Россия' },
          ].map((item) => (
            <div
              key={item.label}
              className="flex items-center justify-between border-b border-[rgb(var(--border))] py-1.5 last:border-0"
            >
              <span className="text-[10px] text-gray-500">{item.label}</span>
              <span className="text-xs text-gray-200">{item.value}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="card p-3">
        <div className="mb-2 flex items-center justify-between">
          <h2 className="text-xs font-semibold text-white">KYC Статус</h2>
          <span className="badge badge-yellow">Level 1</span>
        </div>
        <div className="mb-1.5 h-1.5 w-full rounded-full bg-[rgb(var(--bg-primary))]">
          <div className="h-1.5 rounded-full bg-yellow-500" style={{ width: '33%' }} />
        </div>
        <p className="text-[10px] text-gray-600">Пройдите верификацию для увеличения лимитов</p>
        <button className="btn-outline mt-3 w-full">Пройти KYC сейчас</button>
      </div>
    </div>
  )
}
