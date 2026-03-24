'use client'

import { useState } from 'react'

const withdrawMethods = [
  { id: 'card', name: 'Банковская карта', icon: '💳', min: 500, max: 50000, time: '1-3 дня' },
  { id: 'sbp', name: 'СБП', icon: '📱', min: 1000, max: 100000, time: 'До 24 часов' },
  { id: 'crypto', name: 'Cryptocurrency', icon: '₿', min: 1000, max: 500000, time: 'До 1 часа' },
]

export function WithdrawForm() {
  const [selectedMethod, setSelectedMethod] = useState('card')
  const [amount, setAmount] = useState<number | null>(null)

  const selectedMethodData = withdrawMethods.find((m) => m.id === selectedMethod)

  return (
    <div className="space-y-6">
      <div className="p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
        <p className="text-sm text-yellow-800 dark:text-yellow-200">
          ⚠️ Перед выводом необходимо пройти верификацию (KYC)
        </p>
      </div>

      <div>
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
          Выберите способ вывода
        </h3>
        <div className="grid grid-cols-2 gap-3">
          {withdrawMethods.map((method) => (
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
          Сумма вывода
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
            Мин: {selectedMethodData.min}₽ | Макс: {selectedMethodData.max}₽ | Время: {selectedMethodData.time}
          </p>
        )}
      </div>

      <button 
        disabled={!amount || amount <= 0}
        className="btn-primary w-full disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Вывести {amount || 0}₽
      </button>
    </div>
  )
}
