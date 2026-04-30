'use client'

import { useEffect, useMemo, useState } from 'react'
import { GameCard } from '@components/casino/game-card'
import { useFavoritesStore } from '@stores/favorites-store'
import { trackEvent } from '@lib/telemetry'

const mockGames = [
  {
    id: '1',
    name: 'Book of Dead',
    provider: 'Play\'n GO',
    category: 'slots',
    imageUrl: '/images/games/book-of-dead.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3EBook%20of%20Dead%3C/text%3E%3C/svg%3E",
    isDemoAvailable: true,
    popularityScore: 95,
    rtp: 96.21,
    volatility: 'high',
  },
  {
    id: '2',
    name: 'Starburst',
    provider: 'NetEnt',
    category: 'slots',
    imageUrl: '/images/games/starburst.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3EStarburst%3C/text%3E%3C/svg%3E",
    isDemoAvailable: true,
    popularityScore: 90,
    rtp: 96.09,
    volatility: 'low',
  },
  {
    id: '3',
    name: 'Blackjack VIP',
    provider: 'Evolution',
    category: 'blackjack',
    imageUrl: '/images/games/blackjack-vip.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3EBlackjack%20VIP%3C/text%3E%3C/svg%3E",
    isDemoAvailable: false,
    popularityScore: 85,
    rtp: 99.5,
    volatility: 'medium',
  },
  {
    id: '4',
    name: 'European Roulette',
    provider: 'NetEnt',
    category: 'roulette',
    imageUrl: '/images/games/roulette.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3EEuropean%20Roulette%3C/text%3E%3C/svg%3E",
    isDemoAvailable: true,
    popularityScore: 80,
    rtp: 97.3,
    volatility: 'medium',
  },
  {
    id: '5',
    name: 'Crazy Time',
    provider: 'Evolution',
    category: 'live',
    imageUrl: '/images/games/crazy-time.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3ECrazy%20Time%3C/text%3E%3C/svg%3E",
    isDemoAvailable: false,
    popularityScore: 92,
    rtp: 95.5,
    volatility: 'high',
  },
  {
    id: '6',
    name: 'Gates of Olympus',
    provider: 'Pragmatic Play',
    category: 'slots',
    imageUrl: '/images/games/gates-of-olympus.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3EGates%20of%20Olympus%3C/text%3E%3C/svg%3E",
    isDemoAvailable: true,
    popularityScore: 88,
    rtp: 96.5,
    volatility: 'high',
  },
  {
    id: '7',
    name: 'Sweet Bonanza',
    provider: 'Pragmatic Play',
    category: 'slots',
    imageUrl: '/images/games/sweet-bonanza.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3ESweet%20Bonanza%3C/text%3E%3C/svg%3E",
    isDemoAvailable: true,
    popularityScore: 93,
    rtp: 96.48,
    volatility: 'high',
  },
  {
    id: '8',
    name: 'Lightning Roulette',
    provider: 'Evolution',
    category: 'live',
    imageUrl: '/images/games/lightning-roulette.jpg',
    thumbnailUrl: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400'%3E%3Crect width='300' height='400' fill='%231a1a2e'/%3E%3Ctext x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle' fill='%23ffffff' font-size='16' font-family='sans-serif'%3ELightning%20Roulette%3C/text%3E%3C/svg%3E",
    isDemoAvailable: false,
    popularityScore: 91,
    rtp: 97.3,
    volatility: 'medium',
  },
]

const categories = [
  { id: 'all', name: 'Все' },
  { id: 'slots', name: 'Слоты' },
  { id: 'live', name: 'Live' },
  { id: 'blackjack', name: 'Блэкджек' },
  { id: 'roulette', name: 'Рулетка' },
  { id: 'table', name: 'Настольные' },
]

const providers = [
  { id: 'all', name: 'Все' },
  { id: 'evolution', name: 'Evolution' },
  { id: 'netent', name: 'NetEnt' },
  { id: 'pragmatic', name: 'Pragmatic' },
  { id: 'playngo', name: 'Play\'n GO' },
]

export function CasinoPage() {
  const [selectedCategory, setSelectedCategory] = useState('all')
  const [selectedProvider, setSelectedProvider] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [showDemoOnly, setShowDemoOnly] = useState(false)
  const [activeTab, setActiveTab] = useState<'all' | 'favorites'>('all')
  const { gameIds: favorites } = useFavoritesStore()

  const filteredGames = mockGames.filter((game) => {
    if (activeTab === 'favorites' && !favorites.includes(game.id)) return false
    if (selectedCategory !== 'all' && game.category !== selectedCategory) return false
    if (selectedProvider !== 'all') {
      const providerId = game.provider.toLowerCase().replace(' ', '')
      if (!providerId.includes(selectedProvider.toLowerCase())) return false
    }
    if (showDemoOnly && !game.isDemoAvailable) return false
    if (searchQuery && !game.name.toLowerCase().includes(searchQuery.toLowerCase())) return false
    return true
  })

  const recentlyPlayedGames = useMemo(() => mockGames.slice(0, 4), [])

  useEffect(() => {
    trackEvent('page_view', { page: 'casino' })
  }, [])

  useEffect(() => {
    trackEvent('casino_filter_changed', {
      category: selectedCategory,
      provider: selectedProvider,
      tab: activeTab,
      demoOnly: showDemoOnly,
      results: filteredGames.length,
    })
  }, [selectedCategory, selectedProvider, activeTab, showDemoOnly, filteredGames.length])

  return (
    <div className="section">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-4">
        <div className="flex items-center gap-2">
          <h1 className="text-base font-bold text-white">Казино</h1>
          <span className="text-[10px] text-gray-500 bg-[rgb(var(--bg-tertiary))] px-1.5 py-0.5 rounded">
            {mockGames.length} игр
          </span>
        </div>
        
        <div className="relative">
          <input
            type="text"
            placeholder="Поиск игры..."
            value={searchQuery}
            onChange={(e) => {
              const value = e.target.value
              setSearchQuery(value)
              if (value.length > 1) {
                trackEvent('casino_search_used', { valueLength: value.length })
              }
            }}
            className="input-field w-full sm:w-56 pl-7"
          />
          <svg className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
      </div>

      {/* Tabs: All / Favorites */}
      <div className="flex items-center gap-1 mb-3">
        <button
          onClick={() => setActiveTab('all')}
          className={`px-3 py-1 text-xs font-medium rounded transition-colors ${
            activeTab === 'all' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white hover:bg-white/5'
          }`}
        >
          Все игры
        </button>
        <button
          onClick={() => setActiveTab('favorites')}
          className={`px-3 py-1 text-xs font-medium rounded transition-colors flex items-center gap-1 ${
            activeTab === 'favorites' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white hover:bg-white/5'
          }`}
        >
          <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/>
          </svg>
          Избранное
          {favorites.length > 0 && (
            <span className="badge badge-yellow">{favorites.length}</span>
          )}
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-2 mb-4">
        {/* Categories */}
        <div className="flex gap-1 overflow-x-auto pb-1 flex-1">
          {categories.map((cat) => (
            <button
              key={cat.id}
              onClick={() => setSelectedCategory(cat.id)}
              className={`px-2.5 py-1 text-xs rounded whitespace-nowrap transition-colors ${
                selectedCategory === cat.id
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-white/5'
              }`}
            >
              {cat.name}
            </button>
          ))}
        </div>

        {/* Providers */}
        <div className="flex gap-1 overflow-x-auto pb-1">
          {providers.map((prov) => (
            <button
              key={prov.id}
              onClick={() => setSelectedProvider(prov.id)}
              className={`px-2 py-1 text-[10px] font-medium rounded whitespace-nowrap transition-colors ${
                selectedProvider === prov.id
                  ? 'bg-[rgb(var(--bg-elevated))] text-white'
                  : 'text-gray-600 hover:text-gray-300 hover:bg-white/5'
              }`}
            >
              {prov.name}
            </button>
          ))}
        </div>

        <label className="flex items-center gap-2 text-[11px] text-gray-400 px-2">
          <input
            type="checkbox"
            checked={showDemoOnly}
            onChange={(e) => setShowDemoOnly(e.target.checked)}
            className="rounded border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))]"
          />
          Только демо
        </label>
      </div>

      <div className="card p-3 mb-4">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-xs font-semibold text-white">Недавно играли</h2>
          <button
            onClick={() => {
              setSelectedCategory('all')
              setSelectedProvider('all')
              setSearchQuery('')
              setShowDemoOnly(false)
            }}
            className="text-[10px] text-gray-400 hover:text-white transition-colors"
          >
            Сбросить фильтры
          </button>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {recentlyPlayedGames.map((game) => (
            <GameCard key={`recent-${game.id}`} game={game} compact />
          ))}
        </div>
      </div>

      {/* Games grid */}
      {filteredGames.length > 0 ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-2">
          {filteredGames.map((game) => (
            <GameCard key={game.id} game={game} />
          ))}
        </div>
      ) : (
        <div className="card text-center py-12">
          <p className="text-2xl mb-2 opacity-20">🎰</p>
          <h3 className="text-xs font-medium text-gray-400 mb-1">
            {activeTab === 'favorites' ? 'Нет избранных игр' : 'Ничего не найдено'}
          </h3>
          <p className="text-[10px] text-gray-600">
            {activeTab === 'favorites' ? 'Нажмите ★ на карточке игры чтобы добавить в избранное' : 'Измените параметры поиска'}
          </p>
        </div>
      )}
    </div>
  )
}
