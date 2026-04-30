'use client'

import { useBetSlipStore } from '@stores/bet-slip-store'
import { trackEvent } from '@lib/telemetry'
import { cn } from '@/lib/cn'

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
    const outcomeId = Number(`${event.id}${marketId}${selection === 'home' ? 1 : selection === 'draw' ? 2 : 3}`)
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
      outcomeName: selection === 'home' ? event.homeTeam : selection === 'draw' ? 'Ничья' : event.awayTeam,
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
      <div className={cn(
        "bg-[rgb(var(--bg-secondary))] border border-[rgb(var(--border))] hover:border-[rgb(var(--border-light))] transition-colors",
        event.isLive && "border-l-[3px] border-l-red-500"
      )}>
        {/* Info row */}
        <div className="flex items-center justify-between px-3 py-1.5 border-b border-[rgb(var(--border))] bg-[rgb(var(--bg-tertiary))]">
          <div className="flex items-center gap-2 min-w-0">
            {event.isLive && (
              <span className="badge badge-live shrink-0" aria-label={`Live, ${event.liveMinute || 'LIVE'}`}>
                <span className="w-1.5 h-1.5 rounded-full bg-red-500 live-pulse mr-1" aria-hidden="true" />
                {event.liveMinute || 'LIVE'}
              </span>
            )}
            <span className="text-xs text-gray-500 truncate">{event.league}</span>
          </div>
          <span className="text-xs text-gray-500 shrink-0 ml-2">
            {new Date(event.startTime).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>

        {/* Teams + Odds row */}
        <div className="flex items-center px-3 py-2 gap-3">
          {/* Teams */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              {event.isLive && event.homeScore !== undefined && (
                <span className="text-xs font-bold text-yellow-400 shrink-0 w-6 text-right">{event.homeScore}</span>
              )}
              <p className="text-xs font-medium text-gray-200 truncate">{event.homeTeam}</p>
            </div>
            <div className="flex items-center gap-2 mt-1">
              {event.isLive && event.awayScore !== undefined && (
                <span className="text-xs font-bold text-yellow-400 shrink-0 w-6 text-right">{event.awayScore}</span>
              )}
              <p className="text-xs font-medium text-gray-200 truncate">{event.awayTeam}</p>
            </div>
          </div>

          {/* Odds */}
          <div className={`grid gap-1 shrink-0 ${event.odds.draw ? 'grid-cols-3' : 'grid-cols-2'}`}>
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
