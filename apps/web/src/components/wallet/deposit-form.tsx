'use client'

import { useState } from 'react'

const paymentMethods = [
  { id: 'card', name: 'Банковская карта', icon: '💳', min: 100, max: 100000 },
  { id: 'sbp', name: 'СБП', icon: '📱', min: 100, max: 50000 },
  { id: 'crypto', name: 'Cryptocurrency', icon: '₿', min: 500, max: 500000 },
  { id: 'wallet', name: 'Электронный кошелек', icon: '👛', min: 100, max: 50000 },
]

export function DepositForm() {
  const [selectedMethod, setSelectedMethod] = useState('card')
  const [amount, setAmount] = useState<number | null>(null)

  const selectedMethodData = paymentMethods.find((m) => m.id === selectedMethod)

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
          Выберите способ оплаты
        </h3>
        <div className="grid grid-cols-2 gap-3">
          {paymentMethods.map((method) => (
            <button
              key={method.id}
              onClick={() => setSelectedMethod(method.id)}
              className={`p-4 rounded-lg border-2 transition-colors ${
                selectedMethod === method.id
                  ? 'border-primary-600 bg-primary-50 dark:bg-primary-900/20'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
              }`}
            >
              <span className="text-2xl mb-2 block">{method.icon}</span>
              <span className="text-sm font-medium text-gray-900 dark:text-white">
                {method.name}
              </span>
            </button>
          ))}
        </div>
      </div>

      <div>
        <label className="block text-sm text-gray-600 dark:text-gray-400 mb-2">
          Сумма депозита
        </label>
        <div className="relative">
          <input
            type="number"
            value={amount || ''}
            onChange={(e) => setAmount(Number(e.target.value))}
            className="input-field pr-12"
            placeholder="0"
            min={selectedMethodData?.min}
            max={selectedMethodData?.max}
          />
          <span className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-500 dark:text-gray-400">
            ₽
          </span>
        </div>
        {selectedMethodData && (
          <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
            Мин: {selectedMethodData.min}₽ | Макс: {selectedMethodData.max}₽
          </p>
        )}
      </div>

      {/* Quick amounts */}
      <div className="flex flex-wrap gap-2">
        {[500, 1000, 2000, 5000, 10000].map((quickAmount) => (
          <button
            key={quickAmount}
            onClick={() => setAmount(quickAmount)}
            className="px-4 py-2 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-sm font-medium rounded-lg transition-colors"
          >
            +{quickAmount}₽
          </button>
        ))}
      </div>

      <button className="btn-primary w-full">
        Пополнить на {amount || 0}₽
      </button>
    </div>
  )
}
