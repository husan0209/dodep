export default function TermsPage() {
  return (
    <div className="min-h-[calc(100vh-4rem)] bg-gray-50 px-4 py-12 dark:bg-gray-900 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-3xl">
        <h1 className="mb-8 text-3xl font-bold text-gray-900 dark:text-white">
          Условия использования
        </h1>

        <div className="prose dark:prose-invert max-w-none">
          <p className="text-gray-600 dark:text-gray-400">Эта страница находится в разработке.</p>

          <h2 className="mb-4 mt-8 text-xl font-semibold text-gray-900 dark:text-white">
            1. Общие положения
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Используя нашу платформу, вы соглашаетесь с настоящими условиями использования.
          </p>

          <h2 className="mb-4 mt-8 text-xl font-semibold text-gray-900 dark:text-white">
            2. Регистрация аккаунта
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Для использования услуг платформы необходимо зарегистрироваться и предоставить точную
            информацию.
          </p>

          <h2 className="mb-4 mt-8 text-xl font-semibold text-gray-900 dark:text-white">
            3. Правила использования
          </h2>
          <p className="text-gray-600 dark:text-gray-400">
            Запрещается использовать платформу для незаконной деятельности.
          </p>
        </div>
      </div>
    </div>
  )
}
