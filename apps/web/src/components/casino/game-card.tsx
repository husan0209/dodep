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
        'group relative overflow-hidden rounded-2xl border border-border bg-bg-secondary',
        'shadow-card transition-all duration-300 ease-out',
        'hover:-translate-y-0.5 hover:border-border-light hover:shadow-card-hover',
        className,
      )}
    >
      {/* Game image */}
      <div className={cn('relative overflow-hidden', compact ? 'aspect-[4/5]' : 'aspect-[3/4]')}>
        <img
          src={game.thumbnailUrl || '/placeholder-game.jpg'}
          alt={game.name}
          className="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-110"
          loading="lazy"
        />

        {/* Gradient overlay for depth */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100" />

        {/* Favorite button */}
        <button
          onClick={(e) => {
            e.stopPropagation()
            toggleFavorite(game.id)
          }}
          className={cn(
            'absolute right-2.5 top-2.5 z-10 rounded-lg p-1.5',
            'bg-black/40 backdrop-blur-md backdrop-saturate-150',
            'transition-all duration-200',
            favorite
              ? 'text-yellow-400 opacity-100'
              : 'text-gray-300 opacity-0 hover:text-yellow-400 group-hover:opacity-100',
          )}
        >
          {favorite ? <StarSolid className="h-4 w-4" /> : <StarOutline className="h-4 w-4" />}
        </button>

        {/* Primary badge */}
        {badge && (
          <div className="absolute left-2.5 top-2.5 z-10">
            <span
              className={cn(
                'inline-flex items-center rounded-md px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider',
                badge.variant === 'gold' &&
                  'border border-yellow-500/20 bg-yellow-500/20 text-yellow-400',
                badge.variant === 'violet' &&
                  'border border-violet-500/20 bg-violet-500/20 text-violet-400',
                badge.variant === 'live' && 'border border-red-500/20 bg-red-500/20 text-red-400',
                badge.variant === 'cyan' &&
                  'border border-cyan-500/20 bg-cyan-500/20 text-cyan-400',
                badge.variant === 'emerald' &&
                  'border border-emerald-500/20 bg-emerald-500/20 text-emerald-400',
                badge.animate && 'animate-pulse-fast',
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
        <div className="absolute inset-0 flex items-center justify-center bg-black/0 backdrop-blur-[2px] transition-all duration-300 group-hover:bg-black/50">
          <div className="flex w-full max-w-[85%] translate-y-3 flex-col gap-2 px-4 opacity-0 transition-all duration-300 ease-out group-hover:translate-y-0 group-hover:opacity-100">
            <button
              onClick={handlePlay}
              className="btn-primary w-full animate-glow-pulse py-2.5 text-xs shadow-glow-gold-sm"
            >
              Играть
            </button>
            {game.isDemoAvailable && (
              <button
                onClick={handleDemo}
                className="w-full rounded-xl border border-white/20 bg-white/10 py-2.5 text-xs font-semibold text-white backdrop-blur-md transition-all duration-200 hover:bg-white/20"
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
            <h3 className="truncate text-[13px] font-semibold leading-tight text-text-primary">
              {game.name}
            </h3>
          </div>
        </div>
        <div className="mt-1.5 flex items-center justify-between">
          <span className="text-[11px] font-medium text-text-muted">RTP {game.rtp}%</span>
          <span
            className={cn(
              'rounded px-1.5 py-0.5 text-[11px] font-medium',
              game.volatility === 'high' && 'bg-rose-500/10 text-rose-400',
              game.volatility === 'medium' && 'bg-yellow-500/10 text-yellow-400',
              game.volatility === 'low' && 'bg-emerald-500/10 text-emerald-400',
            )}
          >
            {game.volatility === 'high'
              ? 'Высокая'
              : game.volatility === 'medium'
                ? 'Средняя'
                : 'Низкая'}
          </span>
        </div>
      </div>
    </div>
  )
}
