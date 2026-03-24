'use client'

import { useBetSlipStore } from '@stores/bet-slip-store'
import { XMarkIcon } from '@heroicons/react/24/outline'

export function BetSlip() {
  const { bets, totalOdds, stake, removeBet, clearBets, setStake } = useBetSlipStore()

  const potentialWin = stake * totalOdds

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
          Купон
        </h2>
        {bets.length > 0 && (
          <button
            onClick={clearBets}
            className="text-sm text-red-600 hover:text-red-700 dark:text-red-400"
          >
            Очистить
          </button>
        )}
      </div>

      {bets.length === 0 ? (
        <div className="text-center py-8">
          <p className="text-gray-500 dark:text-gray-400 text-sm">
            Купон пуст
          </p>
          <p className="text-gray-400 dark:text-gray-500 text-xs mt-2">
            Добавьте ставки из событий
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {/* Bets list */}
          <div className="space-y-2">
            {bets.map((bet) => (
              <div
                key={bet.id}
                className="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">
                      {bet.selectionName}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      {bet.eventName}
                    </p>
                  </div>
                  <div className="flex items-center space-x-2">
                    <span className="text-sm font-semibold text-primary-600 dark:text-primary-400">
                      {bet.odds.toFixed(2)}
                    </span>
                    <button
                      onClick={() => removeBet(bet.id)}
                      className="text-gray-400 hover:text-red-500"
                    >
                      <XMarkIcon className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Total odds */}
          <div className="flex items-center justify-between pt-4 border-t border-gray-200 dark:border-gray-700">
            <span className="text-sm text-gray-600 dark:text-gray-400">
              Общий коэффициент
            </span>
            <span className="text-lg font-bold text-primary-600 dark:text-primary-400">
              {totalOdds.toFixed(2)}
            </span>
          </div>

          {/* Stake input */}
          <div>
            <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">
              Сумма ставки
            </label>
            <input
              type="number"
              value={stake}
              onChange={(e) => setStake(Number(e.target.value))}
              className="input-field"
              placeholder="0"
              min="0"
            />
          </div>

          {/* Potential win */}
          <div className="flex items-center justify-between pt-4 border-t border-gray-200 dark:border-gray-700">
            <span className="text-sm text-gray-600 dark:text-gray-400">
              Возможный выигрыш
            </span>
            <span className="text-lg font-bold text-green-600 dark:text-green-400">
              {potentialWin.toFixed(2)} ₽
            </span>
          </div>

          {/* Place bet button */}
          <button
            disabled={stake <= 0}
            className="btn-primary w-full disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Сделать ставку
          </button>
        </div>
      )}
    </div>
  )
}
