'use client'

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
}

export function GameCard({ game }: GameCardProps) {
  const handlePlay = () => {
    // Launch game logic
    console.log('Playing game:', game.id)
  }

  const handleDemo = () => {
    // Launch demo mode
    console.log('Demo game:', game.id)
  }

  return (
    <div className="group relative bg-white dark:bg-gray-800 rounded-lg overflow-hidden shadow-lg hover:shadow-xl transition-all duration-300">
      {/* Game image */}
      <div className="aspect-[3/4] relative overflow-hidden">
        <img
          src={game.thumbnailUrl || '/placeholder-game.jpg'}
          alt={game.name}
          className="w-full h-full object-cover group-hover:scale-110 transition-transform duration-300"
          loading="lazy"
        />
        
        {/* Overlay on hover */}
        <div className="absolute inset-0 bg-black bg-opacity-0 group-hover:bg-opacity-60 transition-all duration-300 flex items-center justify-center">
          <div className="opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex flex-col space-y-2">
            <button
              onClick={handlePlay}
              className="btn-primary text-sm px-6 py-2"
            >
              Играть
            </button>
            {game.isDemoAvailable && (
              <button
                onClick={handleDemo}
                className="bg-white/20 hover:bg-white/30 text-white text-sm font-medium px-6 py-2 rounded-lg transition-colors"
              >
                Демо
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Game info */}
      <div className="p-3">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white truncate">
          {game.name}
        </h3>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
          {game.provider}
        </p>
        
        {/* Stats */}
        <div className="flex items-center justify-between mt-2 text-xs">
          <span className="text-gray-400 dark:text-gray-500">
            RTP: {game.rtp}%
          </span>
          <span className="text-gray-400 dark:text-gray-500 capitalize">
            {game.volatility}
          </span>
        </div>
      </div>

      {/* Popularity badge */}
      {game.popularityScore >= 90 && (
        <div className="absolute top-2 right-2">
          <span className="px-2 py-1 bg-casino-accent text-white text-xs font-medium rounded">
            🔥 Hot
          </span>
        </div>
      )}
    </div>
  )
}
