'use client'

import { useState } from 'react'

export function SupportPage() {
  const [message, setMessage] = useState('')

  const faqs = [
    {
      question: 'Как сделать депозит?',
      answer: 'Перейдите в раздел "Кошелёк", выберите "Депозит", укажите сумму и способ оплаты.',
    },
    {
      question: 'Как долго обрабатывается вывод?',
      answer: 'Вывод средств обычно обрабатывается в течение 24 часов. Банковские переводы могут занять 3-5 рабочих дней.',
    },
    {
      question: 'Как пройти верификацию?',
      answer: 'Загрузите документы в разделе профиля KYC. Мы принимаем паспорт, водительские права и коммунальные платежи.',
    },
  ]

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white mb-6">
        Поддержка
      </h1>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Contact form */}
        <div className="card">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Написать нам
          </h2>
          <form className="space-y-4">
            <div>
              <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                Тема
              </label>
              <select className="input-field">
                <option>Общий вопрос</option>
                <option>Депозит</option>
                <option>Вывод</option>
                <option>Техническая проблема</option>
                <option>Верификация</option>
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
                Сообщение
              </label>
              <textarea
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                rows={4}
                className="input-field"
                placeholder="Опишите вашу проблему..."
              />
            </div>
            <button type="submit" className="btn-primary w-full">
              Отправить
            </button>
          </form>
        </div>

        {/* FAQ */}
        <div className="card">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Частые вопросы
          </h2>
          <div className="space-y-4">
            {faqs.map((faq, index) => (
              <div key={index} className="border-b border-gray-200 dark:border-gray-700 pb-4 last:border-0 last:pb-0">
                <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-2">
                  {faq.question}
                </h3>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  {faq.answer}
                </p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Contact info */}
      <div className="mt-6 card">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Контакты
        </h2>
        <div className="space-y-2 text-sm">
          <p className="text-gray-600 dark:text-gray-400">
            📧 Email: support@opus.casino
          </p>
          <p className="text-gray-600 dark:text-gray-400">
            💬 Live чат: доступен 24/7
          </p>
          <p className="text-gray-600 dark:text-gray-400">
            📱 Telegram: @opuscasino_support
          </p>
        </div>
      </div>
    </div>
  )
}
