'use client'

import { useEffect } from 'react'
import { useBetSlipStore } from '@stores/bet-slip-store'
import { XMarkIcon, TrashIcon } from '@heroicons/react/24/outline'
import { trackEvent } from '@lib/telemetry'

export function BetSlip() {
  const { selections, combinedOdds, stake, removeSelection, clear, setStake } = useBetSlipStore()

  const totalOdds = combinedOdds()
  const potentialWin = stake * totalOdds

  useEffect(() => {
    if (selections.length > 0) {
      trackEvent('betslip_opened', { selections: selections.length })
    }
  }, [selections.length])

  const handlePlaceBet = () => {
    if (stake <= 0 || selections.length === 0) return

    trackEvent('bet_placed', {
      selections: selections.length,
      stake,
      totalOdds: Number(totalOdds.toFixed(2)),
      potentialWin: Number(potentialWin.toFixed(2)),
    })
  }

  return (
    <div className="border border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))]">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[rgb(var(--border))] bg-[rgb(var(--bg-tertiary))] px-3 py-2">
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-white">Купон</span>
          {selections.length > 0 && <span className="badge badge-blue">{selections.length}</span>}
        </div>
        {selections.length > 0 && (
          <button
            onClick={clear}
            className="rounded p-1 text-gray-500 transition-colors hover:bg-white/5 hover:text-red-400"
          >
            <TrashIcon className="h-3.5 w-3.5" />
          </button>
        )}
      </div>

      {selections.length === 0 ? (
        <div className="py-6 text-center">
          <p className="text-xs text-gray-500">Выберите исход</p>
        </div>
      ) : (
        <div className="space-y-2 p-2">
          {/* Selections */}
          {selections.map((sel) => (
            <div
              key={sel.outcomeId}
              className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-2"
            >
              <div className="flex items-start justify-between gap-1">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-medium text-white">{sel.outcomeName}</p>
                  <p className="truncate text-[10px] text-gray-500">{sel.eventName}</p>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <span className="text-xs font-bold text-blue-400">{sel.odds.toFixed(2)}</span>
                  <button
                    onClick={() => removeSelection(sel.outcomeId)}
                    className="rounded p-0.5 text-gray-600 hover:bg-white/5 hover:text-red-400"
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
            <span className="shrink-0 text-xs text-gray-500">₽</span>
          </div>

          {/* Win */}
          <div className="flex items-center justify-between px-1">
            <span className="text-[10px] text-gray-500">Выигрыш</span>
            <span className="text-xs font-bold text-green-400">{potentialWin.toFixed(2)} ₽</span>
          </div>

          {/* Submit */}
          <button
            onClick={handlePlaceBet}
            disabled={stake <= 0}
            className="btn-yellow w-full py-1.5 text-xs font-semibold disabled:cursor-not-allowed disabled:opacity-50"
          >
            Сделать ставку
          </button>
        </div>
      )}
    </div>
  )
}
