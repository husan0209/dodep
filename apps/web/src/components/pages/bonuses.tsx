'use client'

export function BonusesPage() {
  const mockBonuses = [
    {
      id: '1',
      name: 'Приветственный бонус',
      description: '100% на первый депозит до 10000₽',
      minDeposit: 500,
      wagering: 35,
      isActive: true,
      color: 'blue',
    },
    {
      id: '2',
      name: 'Кэшбэк 10%',
      description: 'Получите 10% кэшбэк на проигрыши',
      minDeposit: 0,
      wagering: 1,
      isActive: false,
      color: 'green',
    },
    {
      id: '3',
      name: 'Фриспины',
      description: '50 фриспинов в Book of Dead',
      minDeposit: 1000,
      wagering: 40,
      isActive: true,
      color: 'yellow',
    },
  ]

  return (
    <div className="section max-w-3xl">
      <h1 className="mb-4 text-sm font-bold text-white">Бонусы</h1>
      <p className="mb-4 text-xs text-gray-400">
        Выбирайте бонус под ваш стиль игры. Прогресс по вейджеру отображается после активации в
        профиле.
      </p>

      <div className="space-y-2">
        {mockBonuses.map((bonus) => (
          <div key={bonus.id} className="card flex items-center justify-between gap-3 p-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h3 className="text-xs font-semibold text-white">{bonus.name}</h3>
                {bonus.isActive && <span className="badge badge-green">Активен</span>}
              </div>
              <p className="mt-0.5 text-[11px] text-gray-400">{bonus.description}</p>
              <div className="mt-1.5 flex items-center gap-3 text-[10px] text-gray-600">
                <span>Мин: {bonus.minDeposit}₽</span>
                <span>Вейджер: x{bonus.wagering}</span>
              </div>
            </div>
            <button className="btn-yellow ml-3 shrink-0">
              {bonus.isActive ? 'Взять' : 'Подробнее'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
