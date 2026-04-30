/**
 * Query keys for TanStack Query
 * Follows the pattern: [entity, action, params]
 */

export const queryKeys = {
  // Auth
  auth: {
    all: ["auth"] as const,
    me: () => [...queryKeys.auth.all, "me"] as const,
  },

  // User
  user: {
    all: ["user"] as const,
    profile: () => [...queryKeys.user.all, "profile"] as const,
    preferences: () => [...queryKeys.user.all, "preferences"] as const,
    kyc: () => [...queryKeys.user.all, "kyc"] as const,
  },

  // Wallet
  wallet: {
    all: ["wallet"] as const,
    balances: () => [...queryKeys.wallet.all, "balances"] as const,
    balance: (currency: string) => [...queryKeys.wallet.all, "balance", currency] as const,
    transactions: (filters?: Record<string, string>) =>
      [...queryKeys.wallet.all, "transactions", filters] as const,
  },

  // Bets
  bets: {
    all: ["bets"] as const,
    active: () => [...queryKeys.bets.all, "active"] as const,
    history: (filters?: Record<string, string>) =>
      [...queryKeys.bets.all, "history", filters] as const,
    detail: (id: number) => [...queryKeys.bets.all, "detail", id] as const,
    cashoutValue: (id: number) => [...queryKeys.bets.all, "cashout", id] as const,
  },

  // Casino
  casino: {
    all: ["casino"] as const,
    games: (filters?: Record<string, string>) =>
      [...queryKeys.casino.all, "games", filters] as const,
    game: (id: string) => [...queryKeys.casino.all, "game", id] as const,
    providers: () => [...queryKeys.casino.all, "providers"] as const,
    sessions: () => [...queryKeys.casino.all, "sessions"] as const,
    history: (filters?: Record<string, string>) =>
      [...queryKeys.casino.all, "history", filters] as const,
  },

  // Bonuses
  bonuses: {
    all: ["bonuses"] as const,
    list: () => [...queryKeys.bonuses.all, "list"] as const,
    wagering: () => [...queryKeys.bonuses.all, "wagering"] as const,
  },

  // Notifications
  notifications: {
    all: ["notifications"] as const,
    list: (filters?: Record<string, string>) =>
      [...queryKeys.notifications.all, "list", filters] as const,
    unread: () => [...queryKeys.notifications.all, "unread"] as const,
    settings: () => [...queryKeys.notifications.all, "settings"] as const,
  },
};
