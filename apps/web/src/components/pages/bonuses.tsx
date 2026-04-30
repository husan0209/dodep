'use client'

export function BonusesPage() {
  const mockBonuses = [
    { id: '1', name: 'Приветственный бонус', description: '100% на первый депозит до 10000₽', minDeposit: 500, wagering: 35, isActive: true, color: 'blue' },
    { id: '2', name: 'Кэшбэк 10%', description: 'Получите 10% кэшбэк на проигрыши', minDeposit: 0, wagering: 1, isActive: false, color: 'green' },
    { id: '3', name: 'Фриспины', description: '50 фриспинов в Book of Dead', minDeposit: 1000, wagering: 40, isActive: true, color: 'yellow' },
  ]

  return (
    <div className="section max-w-3xl">
      <h1 className="text-sm font-bold text-white mb-4">Бонусы</h1>
      <p className="text-xs text-gray-400 mb-4">
        Выбирайте бонус под ваш стиль игры. Прогресс по вейджеру отображается после активации в профиле.
      </p>

      <div className="space-y-2">
        {mockBonuses.map((bonus) => (
          <div key={bonus.id} className="card p-3 flex items-center justify-between gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <h3 className="text-xs font-semibold text-white">{bonus.name}</h3>
                {bonus.isActive && <span className="badge badge-green">Активен</span>}
              </div>
              <p className="text-[11px] text-gray-400 mt-0.5">{bonus.description}</p>
              <div className="flex items-center gap-3 mt-1.5 text-[10px] text-gray-600">
                <span>Мин: {bonus.minDeposit}₽</span>
                <span>Вейджер: x{bonus.wagering}</span>
              </div>
            </div>
            <button className="btn-yellow shrink-0 ml-3">
              {bonus.isActive ? 'Взять' : 'Подробнее'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
