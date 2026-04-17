export default function PrivacyPage() {
  return (
    <div className="min-h-[calc(100vh-4rem)] bg-gray-50 dark:bg-gray-900 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-8">
          Политика конфиденциальности
        </h1>
        
        <div className="prose dark:prose-invert max-w-none">
          <p className="text-gray-600 dark:text-gray-400">
            Эта страница находится в разработке.
          </p>
          
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mt-8 mb-4">
            1. Сбор данных
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Мы собираем только необходимые данные для предоставления услуг.
          </p>
          
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mt-8 mb-4">
            2. Использование данных
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Ваши данные используются только для предоставления услуг платформы.
          </p>
          
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mt-8 mb-4">
            3. Защита данных
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Мы принимаем все необходимые меры для защиты ваших данных.
          </p>
        </div>
      </div>
    </div>
  )
}
