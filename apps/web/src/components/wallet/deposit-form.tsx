'use client'



import { useState } from 'react'

import { trackEvent } from '@lib/telemetry'



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



  const handleSubmit = () => {

    if (!amount || amount <= 0 || !selectedMethodData) return

    trackEvent('deposit_submitted', {

      method: selectedMethodData.id,

      amount,

      min: selectedMethodData.min,

      max: selectedMethodData.max,

    })

  }



  return (

    <div className="space-y-4">

      <div>

        <h3 className="text-sm font-semibold text-white mb-3">

          Выберите способ оплаты

        </h3>

        <div className="grid grid-cols-2 gap-3">

          {paymentMethods.map((method) => (

            <button

              key={method.id}

              onClick={() => {

                setSelectedMethod(method.id)

                trackEvent('deposit_started', { method: method.id })

              }}

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

          <span className="absolute right-4 top-1/2 -translate-y-1/2 text-gray-500">

            ₽

          </span>

        </div>

        {selectedMethodData && (

          <p className="text-[10px] text-gray-500 mt-1">

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

            className="px-4 py-2 bg-[rgb(var(--bg-primary))] hover:bg-[rgb(var(--bg-tertiary))] text-xs font-medium rounded transition-colors text-gray-300 border border-[rgb(var(--border))]"

          >

            +{quickAmount}₽

          </button>

        ))}

      </div>



      <button onClick={handleSubmit} className="btn-primary w-full">

        Пополнить на {amount || 0}₽

      </button>

    </div>

  )

}

