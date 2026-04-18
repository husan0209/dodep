'use client'

export function ProfilePage() {
  return (
    <div className="section max-w-2xl">
      <h1 className="text-sm font-bold text-white mb-4">Профиль</h1>

      <div className="card p-3 mb-3">
        <h2 className="text-xs font-semibold text-white mb-3">Личная информация</h2>
        <div className="space-y-2">
          {[
            { label: 'Email', value: 'user@example.com' },
            { label: 'Имя пользователя', value: 'Player123' },
            { label: 'Страна', value: 'Россия' },
          ].map((item) => (
            <div key={item.label} className="flex items-center justify-between py-1.5 border-b border-[rgb(var(--border))] last:border-0">
              <span className="text-[10px] text-gray-500">{item.label}</span>
              <span className="text-xs text-gray-200">{item.value}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="card p-3">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-xs font-semibold text-white">KYC Статус</h2>
          <span className="badge badge-yellow">Level 1</span>
        </div>
        <div className="w-full bg-[rgb(var(--bg-primary))] rounded-full h-1.5 mb-1.5">
          <div className="bg-yellow-500 h-1.5 rounded-full" style={{ width: '33%' }} />
        </div>
        <p className="text-[10px] text-gray-600">Пройдите верификацию для увеличения лимитов</p>
        <button className="btn-outline mt-3 w-full">
          Пройти KYC сейчас
        </button>
      </div>
    </div>
  )
}
