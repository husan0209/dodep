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
      expiresAt: '2024-04-24T23:59:59Z',
    },
    {
      id: '2',
      name: 'Кэшбэк 10%',
      description: 'Получите 10% кэшбэк на проигрыши',
      minDeposit: 0,
      wagering: 1,
      isActive: false,
      expiresAt: null,
    },
    {
      id: '3',
      name: 'Фриспины',
      description: '50 фриспинов в Book of Dead',
      minDeposit: 1000,
      wagering: 40,
      isActive: true,
      expiresAt: '2024-03-31T23:59:59Z',
    },
  ]

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white mb-6">
        Бонусы
      </h1>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {mockBonuses.map((bonus) => (
          <div key={bonus.id} className="card">
            <div className="flex items-start justify-between mb-4">
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                  {bonus.name}
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  {bonus.description}
                </p>
              </div>
              {bonus.isActive && (
                <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-medium rounded">
                  Активен
                </span>
              )}
            </div>

            <div className="space-y-2 mb-4">
              <div className="flex justify-between text-sm">
                <span className="text-gray-600 dark:text-gray-400">Мин. депозит</span>
                <span className="text-gray-900 dark:text-white">{bonus.minDeposit}₽</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-gray-600 dark:text-gray-400">Вейджер</span>
                <span className="text-gray-900 dark:text-white">x{bonus.wagering}</span>
              </div>
            </div>

            <button className="btn-primary w-full">
              {bonus.isActive ? 'Активировать' : 'Подробнее'}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
