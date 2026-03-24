'use client'

import { useState } from 'react'
import { SportsEvent } from '@components/sportsbook/sports-event'
import { BetSlip } from '@components/sportsbook/bet-slip'
import { useBetSlipStore } from '@stores/bet-slip-store'

const mockEvents = [
  {
    id: '1',
    sport: 'Football',
    league: 'Premier League',
    homeTeam: 'Arsenal',
    awayTeam: 'Liverpool',
    startTime: '2024-03-24T15:00:00Z',
    isLive: true,
    odds: {
      home: 2.50,
      draw: 3.20,
      away: 2.80,
    },
  },
  {
    id: '2',
    sport: 'Football',
    league: 'La Liga',
    homeTeam: 'Real Madrid',
    awayTeam: 'Barcelona',
    startTime: '2024-03-24T20:00:00Z',
    isLive: false,
    odds: {
      home: 2.10,
      draw: 3.40,
      away: 3.20,
    },
  },
  {
    id: '3',
    sport: 'Basketball',
    league: 'NBA',
    homeTeam: 'Lakers',
    awayTeam: 'Celtics',
    startTime: '2024-03-25T02:00:00Z',
    isLive: false,
    odds: {
      home: 1.85,
      away: 1.95,
    },
  },
]

const sports = [
  { id: 'all', name: 'Все', icon: '🏆' },
  { id: 'football', name: 'Футбол', icon: '⚽' },
  { id: 'basketball', name: 'Баскетбол', icon: '🏀' },
  { id: 'tennis', name: 'Теннис', icon: '🎾' },
  { id: 'hockey', name: 'Хоккей', icon: '🏒' },
  { id: 'esports', name: 'Киберспорт', icon: '🎮' },
]

export function SportsbookPage() {
  const [selectedSport, setSelectedSport] = useState('all')
  const [showLiveOnly, setShowLiveOnly] = useState(false)
  const { bets } = useBetSlipStore()

  const filteredEvents = mockEvents.filter((event) => {
    if (showLiveOnly && !event.isLive) return false
    if (selectedSport !== 'all') {
      const sportId = event.sport.toLowerCase()
      if (sportId !== selectedSport) return false
    }
    return true
  })

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* Main content */}
        <div className="lg:col-span-3 space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white">
              Sportsbook
            </h1>
            
            <label className="flex items-center space-x-2">
              <input
                type="checkbox"
                checked={showLiveOnly}
                onChange={(e) => setShowLiveOnly(e.target.checked)}
                className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              <span className="text-sm text-gray-600 dark:text-gray-300">
                Только Live
              </span>
            </label>
          </div>

          {/* Sports filter */}
          <div className="flex space-x-2 overflow-x-auto pb-2">
            {sports.map((sport) => (
              <button
                key={sport.id}
                onClick={() => setSelectedSport(sport.id)}
                className={`flex items-center space-x-2 px-4 py-2 rounded-lg whitespace-nowrap transition-colors ${
                  selectedSport === sport.id
                    ? 'bg-primary-600 text-white'
                    : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                <span>{sport.icon}</span>
                <span className="text-sm font-medium">{sport.name}</span>
              </button>
            ))}
          </div>

          {/* Events list */}
          <div className="space-y-4">
            {filteredEvents.length === 0 ? (
              <div className="text-center py-12">
                <p className="text-gray-500 dark:text-gray-400">
                  События не найдены
                </p>
              </div>
            ) : (
              filteredEvents.map((event) => (
                <SportsEvent key={event.id} event={event} />
              ))
            )}
          </div>
        </div>

        {/* Bet slip sidebar */}
        <div className="lg:col-span-1">
          <div className="sticky top-20">
            <BetSlip />
          </div>
        </div>
      </div>
    </div>
  )
}
