'use client'

import { useBetSlipStore } from '@stores/bet-slip-store'

interface SportsEventProps {
  event: {
    id: string
    sport: string
    league: string
    homeTeam: string
    awayTeam: string
    startTime: string
    isLive: boolean
    odds: {
      home: number
      draw?: number
      away: number
    }
  }
}

export function SportsEvent({ event }: SportsEventProps) {
  const { addBet } = useBetSlipStore()

  const handleAddBet = (selection: 'home' | 'draw' | 'away', odds: number) => {
    addBet({
      id: `${event.id}-${selection}`,
      eventId: event.id,
      eventName: `${event.homeTeam} vs ${event.awayTeam}`,
      selectionId: selection,
      selectionName: selection === 'home' ? event.homeTeam : selection === 'draw' ? 'Ничья' : event.awayTeam,
      odds,
      sportId: event.sport.toLowerCase(),
      sportName: event.sport,
      startTime: event.startTime,
    })
  }

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="text-sm text-gray-500 dark:text-gray-400">{event.league}</p>
          <div className="flex items-center space-x-2 mt-1">
            {event.isLive && (
              <span className="px-2 py-0.5 bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-xs font-medium rounded animate-pulse-fast">
                LIVE
              </span>
            )}
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {new Date(event.startTime).toLocaleString('ru-RU')}
            </span>
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between mb-4">
        <div className="flex-1">
          <p className="font-medium text-gray-900 dark:text-white">{event.homeTeam}</p>
          <p className="font-medium text-gray-900 dark:text-white mt-2">{event.awayTeam}</p>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-2">
        <button
          onClick={() => handleAddBet('home', event.odds.home)}
          className="flex flex-col items-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-600 transition-colors"
        >
          <span className="text-xs text-gray-500 dark:text-gray-400 mb-1">1</span>
          <span className="font-semibold text-gray-900 dark:text-white">{event.odds.home.toFixed(2)}</span>
        </button>
        
        {event.odds.draw && (
          <button
            onClick={() => handleAddBet('draw', event.odds.draw)}
            className="flex flex-col items-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-600 transition-colors"
          >
            <span className="text-xs text-gray-500 dark:text-gray-400 mb-1">X</span>
            <span className="font-semibold text-gray-900 dark:text-white">{event.odds.draw.toFixed(2)}</span>
          </button>
        )}
        
        <button
          onClick={() => handleAddBet('away', event.odds.away)}
          className="flex flex-col items-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-600 transition-colors"
        >
          <span className="text-xs text-gray-500 dark:text-gray-400 mb-1">2</span>
          <span className="font-semibold text-gray-900 dark:text-white">{event.odds.away.toFixed(2)}</span>
        </button>
      </div>
    </div>
  )
}
