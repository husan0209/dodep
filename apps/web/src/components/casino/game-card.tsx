'use client'

import { useFavoritesStore } from '@stores/favorites-store'
import { StarIcon as StarOutline } from '@heroicons/react/24/outline'
import { StarIcon as StarSolid } from '@heroicons/react/24/solid'
import { trackEvent } from '@lib/telemetry'

interface GameCardProps {
  game: {
    id: string
    name: string
    provider: string
    category: string
    imageUrl: string
    thumbnailUrl: string
    isDemoAvailable: boolean
    popularityScore: number
    rtp: number
    volatility: string
  }
  compact?: boolean
}

export function GameCard({ game, compact = false }: GameCardProps) {
  const { toggleFavorite, isFavorite } = useFavoritesStore()
  const favorite = isFavorite(game.id)

  const handlePlay = () => {
    trackEvent('casino_game_play_clicked', { gameId: game.id, gameName: game.name })
    console.log('Playing game:', game.id)
  }

  const handleDemo = () => {
    console.log('Demo game:', game.id)
  }

  return (
    <div className="group relative rounded bg-[rgb(var(--bg-secondary))] border border-[rgb(var(--border))] hover:border-[rgb(var(--border-light))] transition-colors overflow-hidden">
      {/* Game image */}
      <div className={`relative overflow-hidden ${compact ? 'aspect-[4/5]' : 'aspect-[3/4]'}`}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={game.thumbnailUrl || '/placeholder-game.jpg'}
          alt={game.name}
          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
          loading="lazy"
        />
        
        {/* Favorite button */}
        <button
          onClick={(e) => { e.stopPropagation(); toggleFavorite(game.id) }}
          className={`absolute top-1.5 right-1.5 p-1 rounded bg-black/50 backdrop-blur-sm transition-colors ${
            favorite ? 'text-yellow-400' : 'text-gray-400 opacity-0 group-hover:opacity-100 hover:text-yellow-400'
          }`}
        >
          {favorite ? <StarSolid className="h-3.5 w-3.5" /> : <StarOutline className="h-3.5 w-3.5" />}
        </button>

        {/* HOT badge */}
        {game.popularityScore >= 90 && (
          <div className="absolute top-1.5 left-1.5">
            <span className="text-[9px] font-bold text-white bg-red-500 px-1.5 py-0.5 rounded">HOT</span>
          </div>
        )}
        
        {/* Overlay on hover */}
        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/60 transition-all duration-150 flex items-center justify-center">
          <div className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex flex-col gap-1.5 px-3">
            <button onClick={handlePlay} className="btn-yellow text-xs py-1.5 px-4">
              Играть
            </button>
            {game.isDemoAvailable && (
              <button
                onClick={handleDemo}
                className="bg-white/10 hover:bg-white/20 text-white text-[10px] font-medium py-1.5 rounded transition-colors border border-white/20"
              >
                Демо
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Game info */}
      <div className="p-2">
        <div className="flex items-start justify-between gap-1">
          <div className="min-w-0 flex-1">
            <h3 className="text-[11px] font-medium text-gray-200 truncate">{game.name}</h3>
            <p className="text-[9px] text-gray-600 mt-0.5">{game.provider}</p>
          </div>
        </div>
        <div className="flex items-center justify-between mt-1">
          <span className="text-[9px] text-gray-600">RTP {game.rtp}%</span>
          <span className="text-[9px] text-gray-600 capitalize">
            {game.volatility === 'high' ? 'Выс.' : game.volatility === 'medium' ? 'Сред.' : 'Низ.'}
          </span>
        </div>
      </div>
    </div>
  )
}
