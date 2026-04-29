'use client'

import { useFavoritesStore } from '@stores/favorites-store'
import { StarIcon as StarOutline } from '@heroicons/react/24/outline'
import { StarIcon as StarSolid } from '@heroicons/react/24/solid'
import { trackEvent } from '@lib/telemetry'
import { cn } from '@/lib/cn'

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
    isNew?: boolean
    isJackpot?: boolean
    isExclusive?: boolean
    hasBonusBuy?: boolean
  }
  compact?: boolean
  className?: string
}

export function GameCard({ game, compact = false, className }: GameCardProps) {
  const { toggleFavorite, isFavorite } = useFavoritesStore()
  const favorite = isFavorite(game.id)

  const handlePlay = () => {
    trackEvent('casino_game_play_clicked', { gameId: game.id, gameName: game.name })
    console.log('Playing game:', game.id)
  }

  const handleDemo = () => {
    console.log('Demo game:', game.id)
  }

  // Badge priority: Jackpot > Exclusive > Hot > BonusBuy > New
  const getPrimaryBadge = () => {
    if (game.isJackpot) return { text: 'JACKPOT', variant: 'gold', animate: true }
    if (game.isExclusive) return { text: 'EXCLUSIVE', variant: 'violet', animate: false }
    if (game.popularityScore >= 90) return { text: 'HOT', variant: 'live', animate: true }
    if (game.hasBonusBuy) return { text: 'BONUS BUY', variant: 'cyan', animate: false }
    if (game.isNew) return { text: 'NEW', variant: 'emerald', animate: false }
    return null
  }

  const badge = getPrimaryBadge()

  return (
    <div
      className={cn(
        'group relative rounded-2xl border border-border bg-bg-secondary overflow-hidden',
        'shadow-card transition-all duration-300 ease-out',
        'hover:border-border-light hover:shadow-card-hover hover:-translate-y-0.5',
        className
      )}
    >
      {/* Game image */}
      <div className={cn('relative overflow-hidden', compact ? 'aspect-[4/5]' : 'aspect-[3/4]')}>
        <img
          src={game.thumbnailUrl || '/placeholder-game.jpg'}
          alt={game.name}
          className="w-full h-full object-cover transition-transform duration-500 ease-out group-hover:scale-110"
          loading="lazy"
        />

        {/* Gradient overlay for depth */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />

        {/* Favorite button */}
        <button
          onClick={(e) => { e.stopPropagation(); toggleFavorite(game.id) }}
          className={cn(
            'absolute top-2.5 right-2.5 z-10 p-1.5 rounded-lg',
            'bg-black/40 backdrop-blur-md backdrop-saturate-150',
            'transition-all duration-200',
            favorite
              ? 'text-yellow-400 opacity-100'
              : 'text-gray-300 opacity-0 group-hover:opacity-100 hover:text-yellow-400'
          )}
        >
          {favorite ? <StarSolid className="h-4 w-4" /> : <StarOutline className="h-4 w-4" />}
        </button>

        {/* Primary badge */}
        {badge && (
          <div className="absolute top-2.5 left-2.5 z-10">
            <span
              className={cn(
                'inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-bold uppercase tracking-wider',
                badge.variant === 'gold' && 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/20',
                badge.variant === 'violet' && 'bg-violet-500/20 text-violet-400 border border-violet-500/20',
                badge.variant === 'live' && 'bg-red-500/20 text-red-400 border border-red-500/20',
                badge.variant === 'cyan' && 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/20',
                badge.variant === 'emerald' && 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/20',
                badge.animate && 'animate-pulse-fast'
              )}
            >
              {badge.text}
            </span>
          </div>
        )}

        {/* Provider watermark (always visible) */}
        <div className="absolute bottom-2 left-2.5 z-10">
          <span className="text-[10px] font-medium text-white/70 drop-shadow-md">
            {game.provider}
          </span>
        </div>

        {/* Glassmorphism hover overlay with actions */}
        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/50 backdrop-blur-[2px] transition-all duration-300 flex items-center justify-center">
          <div className="opacity-0 group-hover:opacity-100 translate-y-3 group-hover:translate-y-0 transition-all duration-300 ease-out flex flex-col gap-2 px-4 w-full max-w-[85%]">
            <button
              onClick={handlePlay}
              className="btn-primary w-full text-xs py-2.5 shadow-glow-gold-sm animate-glow-pulse"
            >
              Играть
            </button>
            {game.isDemoAvailable && (
              <button
                onClick={handleDemo}
                className="w-full py-2.5 rounded-xl text-xs font-semibold text-white bg-white/10 hover:bg-white/20 border border-white/20 backdrop-blur-md transition-all duration-200"
              >
                Демо
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Game info */}
      <div className="p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="text-[13px] font-semibold text-text-primary truncate leading-tight">
              {game.name}
            </h3>
          </div>
        </div>
        <div className="flex items-center justify-between mt-1.5">
          <span className="text-[11px] text-text-muted font-medium">
            RTP {game.rtp}%
          </span>
          <span
            className={cn(
              'text-[11px] font-medium px-1.5 py-0.5 rounded',
              game.volatility === 'high' && 'bg-rose-500/10 text-rose-400',
              game.volatility === 'medium' && 'bg-yellow-500/10 text-yellow-400',
              game.volatility === 'low' && 'bg-emerald-500/10 text-emerald-400'
            )}
          >
            {game.volatility === 'high' ? 'Высокая' : game.volatility === 'medium' ? 'Средняя' : 'Низкая'}
          </span>
        </div>
      </div>
    </div>
  )
}
