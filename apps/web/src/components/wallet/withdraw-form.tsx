'use client'

import { useState } from 'react'
import { trackEvent } from '@lib/telemetry'

const withdrawMethods = [
  { id: 'card', name: 'Банковская карта', icon: '💳', min: 500, max: 50000, time: '1-3 дня' },
  { id: 'sbp', name: 'СБП', icon: '📱', min: 1000, max: 100000, time: 'До 24 часов' },
  { id: 'crypto', name: 'Cryptocurrency', icon: '₿', min: 1000, max: 500000, time: 'До 1 часа' },
]

export function WithdrawForm() {
  const [selectedMethod, setSelectedMethod] = useState('card')
  const [amount, setAmount] = useState<number | null>(null)

  const selectedMethodData = withdrawMethods.find((m) => m.id === selectedMethod)

  const handleSubmit = () => {
    if (!amount || amount <= 0 || !selectedMethodData) return
    trackEvent('withdraw_submitted', {
      method: selectedMethodData.id,
      amount,
      eta: selectedMethodData.time,
    })
  }

  return (
    <div className="space-y-4">
      <div className="p-3 bg-yellow-500/10 border border-yellow-500/30 rounded">
        <p className="text-xs text-yellow-300">
          ⚠️ Перед выводом необходимо пройти верификацию (KYC)
        </p>
      </div>

      <div>
        <h3 className="text-sm font-semibold text-white mb-3">
          Выберите способ вывода
        </h3>
        <div className="grid grid-cols-2 gap-3">
          {withdrawMethods.map((method) => (
            <button
              key={method.id}
              onClick={() => setSelectedMethod(method.id)}
              className={`p-3 rounded border transition-colors ${
                selectedMethod === method.id
                  ? 'border-blue-500 bg-blue-500/10'
                  : 'border-[rgb(var(--border))] hover:border-[rgb(var(--border-light))]'
              }`}
            >
              <span className="text-xl mb-2 block">{method.icon}</span>
              <span className="text-xs font-medium text-gray-200">
                {method.name}
              </span>
            </button>
          ))}
        </div>
      </div>

      <div>
        <label className="block text-xs text-gray-400 mb-2">
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
          <span className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-500">
            ₽
          </span>
        </div>
        {selectedMethodData && (
          <p className="text-[10px] text-gray-500 mt-1">
            Мин: {selectedMethodData.min}₽ | Макс: {selectedMethodData.max}₽ | Время: {selectedMethodData.time}
          </p>
        )}
      </div>

      <button 
        onClick={handleSubmit}
        disabled={!amount || amount <= 0}
        className="btn-primary w-full disabled:opacity-50 disabled:cursor-not-allowed"
      >
        Вывести {amount || 0}₽
      </button>
    </div>
  )
}
