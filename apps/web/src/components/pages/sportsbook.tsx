'use client'

import { useState } from 'react'
import { useEffect } from 'react'
import { SportsEvent } from '@components/sportsbook/sports-event'
import { BetSlip } from '@components/sportsbook/bet-slip'
import { useBetSlipStore } from '@stores/bet-slip-store'
import { trackEvent } from '@lib/telemetry'

const mockEvents = [
  {
    id: '1',
    sport: 'football',
    league: 'Premier League',
    homeTeam: 'Arsenal',
    awayTeam: 'Liverpool',
    startTime: '2024-03-24T15:00:00Z',
    isLive: true,
    liveMinute: "62'",
    homeScore: 2,
    awayScore: 1,
    odds: { home: 2.5, draw: 3.2, away: 2.8 },
  },
  {
    id: '2',
    sport: 'football',
    league: 'La Liga',
    homeTeam: 'Real Madrid',
    awayTeam: 'Barcelona',
    startTime: '2024-03-24T20:00:00Z',
    isLive: false,
    odds: { home: 2.1, draw: 3.4, away: 3.2 },
  },
  {
    id: '3',
    sport: 'basketball',
    league: 'NBA',
    homeTeam: 'Lakers',
    awayTeam: 'Celtics',
    startTime: '2024-03-25T02:00:00Z',
    isLive: false,
    odds: { home: 1.85, away: 1.95 },
  },
  {
    id: '4',
    sport: 'tennis',
    league: 'ATP Miami',
    homeTeam: 'Djokovic N.',
    awayTeam: 'Alcaraz C.',
    startTime: '2024-03-24T18:00:00Z',
    isLive: true,
    liveMinute: 'Set 2',
    homeScore: 1,
    awayScore: 0,
    odds: { home: 1.65, away: 2.2 },
  },
  {
    id: '5',
    sport: 'football',
    league: 'Serie A',
    homeTeam: 'Juventus',
    awayTeam: 'AC Milan',
    startTime: '2024-03-25T19:45:00Z',
    isLive: false,
    odds: { home: 2.3, draw: 3.1, away: 3.0 },
  },
]

const sports = [
  { id: 'all', name: 'Все', icon: '🏆', count: mockEvents.length },
  {
    id: 'football',
    name: 'Футбол',
    icon: '⚽',
    count: mockEvents.filter((e) => e.sport === 'football').length,
  },
  {
    id: 'basketball',
    name: 'Баскетбол',
    icon: '🏀',
    count: mockEvents.filter((e) => e.sport === 'basketball').length,
  },
  {
    id: 'tennis',
    name: 'Теннис',
    icon: '🎾',
    count: mockEvents.filter((e) => e.sport === 'tennis').length,
  },
  { id: 'hockey', name: 'Хоккей', icon: '🏒', count: 0 },
  { id: 'esports', name: 'Киберспорт', icon: '🎮', count: 0 },
]

export function SportsbookPage() {
  const [selectedSport, setSelectedSport] = useState('all')
  const [showLiveOnly, setShowLiveOnly] = useState(false)
  const { selections } = useBetSlipStore()
  const [mobileBetSlipOpen, setMobileBetSlipOpen] = useState(false)

  const filteredEvents = mockEvents.filter((event) => {
    if (showLiveOnly && !event.isLive) return false
    if (selectedSport !== 'all' && event.sport !== selectedSport) return false
    return true
  })

  const liveCount = mockEvents.filter((e) => e.isLive).length

  useEffect(() => {
    trackEvent('page_view', { page: 'sportsbook' })
  }, [])

  useEffect(() => {
    trackEvent('sportsbook_filter_changed', {
      selectedSport,
      showLiveOnly,
      eventsVisible: filteredEvents.length,
    })
  }, [selectedSport, showLiveOnly, filteredEvents.length])

  return (
    <div className="mx-auto flex max-w-[1440px]">
      {/* Left sidebar - Sports */}
      <aside className="sidebar sticky top-10 hidden min-h-[calc(100vh-40px)] w-48 shrink-0 lg:block">
        <div className="p-2">
          <div className="px-2 py-1.5 text-[10px] font-semibold uppercase tracking-wider text-gray-500">
            Виды спорта
          </div>
          <div className="space-y-0.5">
            {sports.map((sport) => (
              <button
                key={sport.id}
                onClick={() => setSelectedSport(sport.id)}
                className={`flex w-full items-center justify-between rounded px-2 py-1.5 text-xs transition-colors ${
                  selectedSport === sport.id
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'
                }`}
              >
                <span className="flex items-center gap-1.5">
                  <span>{sport.icon}</span>
                  <span>{sport.name}</span>
                </span>
                {sport.count > 0 && (
                  <span
                    className={`text-[10px] ${selectedSport === sport.id ? 'text-blue-200' : 'text-gray-600'}`}
                  >
                    {sport.count}
                  </span>
                )}
              </button>
            ))}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <div className="min-w-0 flex-1">
        <div className="px-2 py-3">
          {/* Mobile sports filter */}
          <div className="-mx-2 mb-2 flex gap-1 overflow-x-auto px-2 pb-2 lg:hidden">
            {sports.map((sport) => (
              <button
                key={sport.id}
                onClick={() => setSelectedSport(sport.id)}
                className={`flex items-center gap-1 whitespace-nowrap rounded px-2.5 py-1 text-xs transition-colors ${
                  selectedSport === sport.id
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'
                }`}
              >
                <span>{sport.icon}</span>
                <span>{sport.name}</span>
                {sport.count > 0 && <span className="text-[10px] opacity-60">{sport.count}</span>}
              </button>
            ))}
          </div>

          {/* Header */}
          <div className="mb-3 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <h1 className="text-sm font-bold text-white">Ставки на спорт</h1>
              {liveCount > 0 && (
                <span className="badge badge-live">
                  <span className="live-pulse mr-1 h-1.5 w-1.5 rounded-full bg-red-500" />
                  {liveCount} Live
                </span>
              )}
            </div>

            <label className="flex cursor-pointer items-center gap-1.5">
              <div className="relative">
                <input
                  type="checkbox"
                  checked={showLiveOnly}
                  onChange={(e) => {
                    setShowLiveOnly(e.target.checked)
                  }}
                  className="peer sr-only"
                />
                <div className="h-4 w-8 rounded-full border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] transition-colors peer-checked:bg-blue-600" />
                <div className="absolute left-0.5 top-0.5 h-3 w-3 rounded-full bg-gray-500 transition-transform peer-checked:translate-x-4 peer-checked:bg-white" />
              </div>
              <span className="text-[10px] text-gray-500">Live</span>
            </label>
          </div>

          {/* Events */}
          <div className="space-y-1">
            {filteredEvents.length === 0 ? (
              <div className="card py-10 text-center">
                <p className="mb-2 text-xl opacity-20">⚽</p>
                <h3 className="text-xs font-medium text-gray-400">Нет событий</h3>
                <p className="mt-1 text-[10px] text-gray-600">Измените параметры фильтрации</p>
              </div>
            ) : (
              filteredEvents.map((event) => <SportsEvent key={event.id} event={event} />)
            )}
          </div>
        </div>
      </div>

      {/* Right sidebar - Bet slip */}
      <aside className="sticky top-10 hidden min-h-[calc(100vh-40px)] w-64 shrink-0 p-2 xl:block">
        <BetSlip />
      </aside>

      {selections.length > 0 && (
        <button
          onClick={() => setMobileBetSlipOpen((value) => !value)}
          className="fixed bottom-16 right-3 z-40 rounded-lg bg-blue-600 px-3 py-2 text-xs font-semibold text-white shadow-lg shadow-blue-900/40 xl:hidden"
        >
          Купон ({selections.length})
        </button>
      )}

      {mobileBetSlipOpen && (
        <div className="fixed inset-x-0 bottom-0 z-50 max-h-[70vh] overflow-y-auto border-t border-[rgb(var(--border))] bg-[rgb(var(--bg-secondary))] p-2 xl:hidden">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-xs font-semibold text-white">Купон</h2>
            <button
              onClick={() => setMobileBetSlipOpen(false)}
              className="text-[10px] text-gray-400 transition-colors hover:text-white"
            >
              Закрыть
            </button>
          </div>
          <BetSlip />
        </div>
      )}
    </div>
  )
}
