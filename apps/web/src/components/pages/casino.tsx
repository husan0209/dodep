'use client'

import { useState } from 'react'
import { GameCard } from '@components/casino/game-card'

const mockGames = [
  {
    id: '1',
    name: 'Book of Dead',
    provider: 'Play\'n GO',
    category: 'slots',
    imageUrl: '/images/games/book-of-dead.jpg',
    thumbnailUrl: '/images/games/thumbs/book-of-dead.jpg',
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
    thumbnailUrl: '/images/games/thumbs/starburst.jpg',
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
    thumbnailUrl: '/images/games/thumbs/blackjack-vip.jpg',
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
    thumbnailUrl: '/images/games/thumbs/roulette.jpg',
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
    thumbnailUrl: '/images/games/thumbs/crazy-time.jpg',
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
    thumbnailUrl: '/images/games/thumbs/gates-of-olympus.jpg',
    isDemoAvailable: true,
    popularityScore: 88,
    rtp: 96.5,
    volatility: 'high',
  },
]

const categories = [
  { id: 'all', name: 'Все игры' },
  { id: 'slots', name: 'Слоты' },
  { id: 'live', name: 'Live казино' },
  { id: 'blackjack', name: 'Блэкджек' },
  { id: 'roulette', name: 'Рулетка' },
  { id: 'table', name: 'Настольные' },
]

const providers = [
  { id: 'all', name: 'Все провайдеры' },
  { id: 'evolution', name: 'Evolution' },
  { id: 'netent', name: 'NetEnt' },
  { id: 'pragmatic', name: 'Pragmatic Play' },
  { id: 'playngo', name: 'Play\'n GO' },
]

export function CasinoPage() {
  const [selectedCategory, setSelectedCategory] = useState('all')
  const [selectedProvider, setSelectedProvider] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  const filteredGames = mockGames.filter((game) => {
    if (selectedCategory !== 'all' && game.category !== selectedCategory) {
      return false
    }
    if (selectedProvider !== 'all') {
      const providerId = game.provider.toLowerCase().replace(' ', '')
      if (!providerId.includes(selectedProvider.toLowerCase())) return false
    }
    if (searchQuery && !game.name.toLowerCase().includes(searchQuery.toLowerCase())) {
      return false
    }
    return true
  })

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
        <h1 className="text-2xl font-bold font-display text-gray-900 dark:text-white">
          Казино
        </h1>
        
        <div className="relative">
          <input
            type="text"
            placeholder="Поиск игр..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="input-field pl-10 pr-4 py-2 w-full sm:w-64"
          />
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>
      </div>

      {/* Category filter */}
      <div className="flex flex-wrap gap-2 mb-6">
        {categories.map((category) => (
          <button
            key={category.id}
            onClick={() => setSelectedCategory(category.id)}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              selectedCategory === category.id
                ? 'bg-primary-600 text-white'
                : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
            }`}
          >
            {category.name}
          </button>
        ))}
      </div>

      {/* Provider filter */}
      <div className="flex flex-wrap gap-2 mb-6">
        {providers.map((provider) => (
          <button
            key={provider.id}
            onClick={() => setSelectedProvider(provider.id)}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              selectedProvider === provider.id
                ? 'bg-casino-accent text-white'
                : 'bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
            }`}
          >
            {provider.name}
          </button>
        ))}
      </div>

      {/* Games grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
        {filteredGames.map((game) => (
          <GameCard key={game.id} game={game} />
        ))}
      </div>

      {/* Empty state */}
      {filteredGames.length === 0 && (
        <div className="text-center py-12">
          <p className="text-gray-500 dark:text-gray-400">
            Игры не найдены
          </p>
        </div>
      )}
    </div>
  )
}
