import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface FavoritesState {
  gameIds: string[]
  toggleFavorite: (gameId: string) => void
  isFavorite: (gameId: string) => boolean
}

export const useFavoritesStore = create<FavoritesState>()(
  persist(
    (set, get) => ({
      gameIds: [],
      toggleFavorite: (gameId: string) => {
        set((state) => ({
          gameIds: state.gameIds.includes(gameId)
            ? state.gameIds.filter((id) => id !== gameId)
            : [...state.gameIds, gameId],
        }))
      },
      isFavorite: (gameId: string) => {
        return get().gameIds.includes(gameId)
      },
    }),
    {
      name: 'opus-favorites',
    }
  )
)
