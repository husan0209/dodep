'use client'

import { useBetSlipStore } from '@stores/bet-slip-store'
import { trackEvent } from '@lib/telemetry'

interface SportsEventProps {
  event: {
    id: string
    sport: string
    league: string
    homeTeam: string
    awayTeam: string
    startTime: string
    isLive: boolean
    liveMinute?: string
    homeScore?: number
    awayScore?: number
    odds: {
      home: number
      draw?: number
      away: number
    }
  }
}

export function SportsEvent({ event }: SportsEventProps) {
  const { addSelection, selections, removeSelection } = useBetSlipStore()

  const isSelected = (outcomeId: number) => {
    return selections.some((s) => s.outcomeId === outcomeId)
  }

  const handleAddBet = (selection: 'home' | 'draw' | 'away', odds: number, marketId: number) => {
    const outcomeId = Number(
      `${event.id}${marketId}${selection === 'home' ? 1 : selection === 'draw' ? 2 : 3}`,
    )
    if (isSelected(outcomeId)) {
      removeSelection(outcomeId)
      return
    }

    trackEvent('odds_selected', {
      eventId: event.id,
      sport: event.sport,
      market: selection,
      odds,
      isLive: event.isLive,
    })

    addSelection({
      eventId: Number(event.id),
      marketId,
      outcomeId,
      outcomeName:
        selection === 'home' ? event.homeTeam : selection === 'draw' ? 'Ничья' : event.awayTeam,
      odds,
      eventName: `${event.homeTeam} vs ${event.awayTeam}`,
      marketName: selection === 'home' ? 'П1' : selection === 'draw' ? 'X' : 'П2',
    })
  }

  const outcomeIds = {
    home: Number(`${event.id}11`),
    draw: event.odds.draw ? Number(`${event.id}12`) : undefined,
    away: Number(`${event.id}13`),
  }

  return (
    <div className="fade-in">
      {/* Event row - 1xbet style compact layout */}
      <div className="border border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))] transition-colors hover:border-[rgb(var(--border-light))]">
        {/* Info row */}
        <div className="flex items-center justify-between border-b border-[rgb(var(--border))] bg-[rgb(var(--bg-tertiary))] px-3 py-1.5">
          <div className="flex min-w-0 items-center gap-2">
            {event.isLive && (
              <span className="badge badge-live shrink-0">
                <span className="live-pulse mr-1 h-1.5 w-1.5 rounded-full bg-red-500" />
                {event.liveMinute || 'LIVE'}
              </span>
            )}
            <span className="truncate text-[10px] text-gray-500">{event.league}</span>
          </div>
          <span className="ml-2 shrink-0 text-[10px] text-gray-500">
            {new Date(event.startTime).toLocaleTimeString('ru-RU', {
              hour: '2-digit',
              minute: '2-digit',
            })}
          </span>
        </div>

        {/* Teams + Odds row */}
        <div className="flex items-center gap-3 px-3 py-2">
          {/* Teams */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              {event.isLive && event.homeScore !== undefined && (
                <span className="w-6 shrink-0 text-right text-xs font-bold text-yellow-400">
                  {event.homeScore}
                </span>
              )}
              <p className="truncate text-xs font-medium text-gray-200">{event.homeTeam}</p>
            </div>
            <div className="mt-1 flex items-center gap-2">
              {event.isLive && event.awayScore !== undefined && (
                <span className="w-6 shrink-0 text-right text-xs font-bold text-yellow-400">
                  {event.awayScore}
                </span>
              )}
              <p className="truncate text-xs font-medium text-gray-200">{event.awayTeam}</p>
            </div>
          </div>

          {/* Odds */}
          <div className={`grid shrink-0 gap-1 ${event.odds.draw ? 'grid-cols-3' : 'grid-cols-2'}`}>
            <button
              onClick={() => handleAddBet('home', event.odds.home, 1)}
              className={`odds-btn ${isSelected(outcomeIds.home) ? 'odds-btn-selected' : ''}`}
            >
              <span className="odds-label">1</span>
              <span className="odds-value">{event.odds.home.toFixed(2)}</span>
            </button>

            {event.odds.draw && (
              <button
                onClick={() => handleAddBet('draw', event.odds.draw!, 1)}
                className={`odds-btn ${outcomeIds.draw && isSelected(outcomeIds.draw) ? 'odds-btn-selected' : ''}`}
              >
                <span className="odds-label">X</span>
                <span className="odds-value">{event.odds.draw.toFixed(2)}</span>
              </button>
            )}

            <button
              onClick={() => handleAddBet('away', event.odds.away, 1)}
              className={`odds-btn ${isSelected(outcomeIds.away) ? 'odds-btn-selected' : ''}`}
            >
              <span className="odds-label">2</span>
              <span className="odds-value">{event.odds.away.toFixed(2)}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
