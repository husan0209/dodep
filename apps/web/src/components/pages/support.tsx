'use client'

import { useState } from 'react'

export function SupportPage() {
  const [message, setMessage] = useState('')
  const [isSent, setIsSent] = useState(false)

  const faqs = [
    { question: 'Как сделать депозит?', answer: 'Перейдите в раздел "Кошелёк" → "Пополнить".' },
    {
      question: 'Как долго обрабатывается вывод?',
      answer: 'Обычно в течение 24 часов. Банковские переводы — 3-5 дней.',
    },
    { question: 'Как пройти верификацию?', answer: 'Загрузите документы в разделе профиля KYC.' },
  ]

  return (
    <div className="section max-w-2xl">
      <h1 className="mb-4 text-sm font-bold text-white">Поддержка</h1>

      <div className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2">
        <div className="card p-3">
          <h2 className="mb-3 text-xs font-semibold text-white">Написать нам</h2>
          <form
            className="space-y-2"
            onSubmit={(event) => {
              event.preventDefault()
              setIsSent(true)
              setMessage('')
            }}
          >
            <select className="input-field">
              <option>Общий вопрос</option>
              <option>Депозит</option>
              <option>Вывод</option>
              <option>Техническая проблема</option>
            </select>
            <textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              rows={3}
              className="input-field resize-none"
              placeholder="Сообщение..."
            />
            <button type="submit" className="btn-primary w-full">
              Отправить
            </button>
            {isSent && (
              <p className="text-[10px] text-green-400">
                Запрос отправлен. Ответим в чате или на email.
              </p>
            )}
          </form>
        </div>

        <div className="card p-3">
          <h2 className="mb-3 text-xs font-semibold text-white">FAQ</h2>
          <div className="space-y-2">
            {faqs.map((faq, i) => (
              <div
                key={i}
                className="border-b border-[rgb(var(--border))] pb-2 last:border-0 last:pb-0"
              >
                <p className="text-[10px] font-medium text-gray-300">{faq.question}</p>
                <p className="mt-0.5 text-[10px] text-gray-600">{faq.answer}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="card p-3">
        <h2 className="mb-2 text-xs font-semibold text-white">Контакты</h2>
        <div className="space-y-1 text-[10px] text-gray-500">
          <p>Email: support@dod.casino</p>
          <p>Live чат: 24/7</p>
          <p>Telegram: @dod_support</p>
        </div>
      </div>
    </div>
  )
}
