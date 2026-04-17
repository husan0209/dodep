'use client'

import { useBetSlipStore } from '@stores/bet-slip-store'
import { XMarkIcon, TrashIcon } from '@heroicons/react/24/outline'

export function BetSlip() {
  const { selections, combinedOdds, stake, removeSelection, clear, setStake } = useBetSlipStore()

  const totalOdds = combinedOdds()
  const potentialWin = stake * totalOdds

  return (
    <div className="bg-[rgb(var(--bg-secondary))] border border-[rgb(var(--border))]">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[rgb(var(--border))] bg-[rgb(var(--bg-tertiary))]">
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-white">Купон</span>
          {selections.length > 0 && (
            <span className="badge badge-blue">{selections.length}</span>
          )}
        </div>
        {selections.length > 0 && (
          <button
            onClick={clear}
            className="p-1 rounded hover:bg-white/5 text-gray-500 hover:text-red-400 transition-colors"
          >
            <TrashIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {selections.length === 0 ? (
        <div className="text-center py-6">
          <p className="text-xs text-gray-500">Выберите исход</p>
        </div>
      ) : (
        <div className="p-2 space-y-2">
          {/* Selections */}
          {selections.map((sel) => (
            <div
              key={sel.outcomeId}
              className="p-2 bg-[rgb(var(--bg-primary))] rounded border border-[rgb(var(--border))]"
            >
              <div className="flex items-start justify-between gap-1">
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-medium text-white truncate">{sel.outcomeName}</p>
                  <p className="text-[10px] text-gray-500 truncate">{sel.eventName}</p>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <span className="text-xs font-bold text-blue-400">{sel.odds.toFixed(2)}</span>
                  <button
                    onClick={() => removeSelection(sel.outcomeId)}
                    className="p-0.5 rounded hover:bg-white/5 text-gray-600 hover:text-red-400"
                  >
                    <XMarkIcon className="h-3 w-3" />
                  </button>
                </div>
              </div>
            </div>
          ))}

          {/* Total */}
          <div className="flex items-center justify-between px-1 py-1">
            <span className="text-[10px] text-gray-500">Коэффициент</span>
            <span className="text-xs font-bold text-blue-400">{totalOdds.toFixed(2)}</span>
          </div>

          {/* Stake */}
          <div className="flex items-center gap-1">
            <input
              type="number"
              value={stake || ''}
              onChange={(e) => setStake(Number(e.target.value))}
              className="input-field flex-1"
              placeholder="Сумма"
              min="0"
            />
            <span className="text-xs text-gray-500 shrink-0">₽</span>
          </div>

          {/* Win */}
          <div className="flex items-center justify-between px-1">
            <span className="text-[10px] text-gray-500">Выигрыш</span>
            <span className="text-xs font-bold text-green-400">{potentialWin.toFixed(2)} ₽</span>
          </div>

          {/* Submit */}
          <button
            disabled={stake <= 0}
            className="btn-yellow w-full disabled:opacity-50 disabled:cursor-not-allowed py-1.5 text-xs font-semibold"
          >
            Сделать ставку
          </button>
        </div>
      )}
    </div>
  )
}
