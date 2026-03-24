'use client'

export function ProfilePage() {
  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white mb-6">
        Профиль
      </h1>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Profile info */}
        <div className="card md:col-span-2">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Личная информация
          </h2>
          <div className="space-y-4">
            <div>
              <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                Email
              </label>
              <p className="text-gray-900 dark:text-white">user@example.com</p>
            </div>
            <div>
              <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                Имя пользователя
              </label>
              <p className="text-gray-900 dark:text-white">Player123</p>
            </div>
            <div>
              <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                Страна
              </label>
              <p className="text-gray-900 dark:text-white">Россия</p>
            </div>
          </div>
        </div>

        {/* KYC Status */}
        <div className="card">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            KYC Статус
          </h2>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600 dark:text-gray-400">Уровень</span>
              <span className="text-sm font-medium text-yellow-600">Level 1</span>
            </div>
            <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
              <div className="bg-yellow-500 h-2 rounded-full" style={{ width: '33%' }} />
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Пройдите верификацию для увеличения лимитов
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
