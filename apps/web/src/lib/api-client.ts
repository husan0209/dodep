import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios'
import { getAuthToken, clearAuthToken } from './auth'

// Base API client — all requests route through nginx (/api/*)
// In production: NEXT_PUBLIC_API_URL=/api (relative, same origin)
// In development: NEXT_PUBLIC_API_URL=http://localhost:8083 (direct service)
export const apiClient = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor for auth
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = getAuthToken()
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor for error handling
apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      clearAuthToken()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// API endpoints
export const api = {
  // Auth
  auth: {
    login: (data: { email: string; password: string }) =>
      apiClient.post('/v1/auth/login', data),
    register: (data: { email: string; password: string; username: string }) =>
      apiClient.post('/v1/auth/register', data),
    logout: () => apiClient.post('/v1/auth/logout'),
    me: () => apiClient.get('/v1/auth/me'),
    refresh: () => apiClient.post('/v1/auth/refresh'),
  },

  // User
  user: {
    get: (userId: string) => apiClient.get(`/v1/users/${userId}`),
    update: (userId: string, data: any) =>
      apiClient.put(`/v1/users/${userId}`, data),
    getPreferences: (userId: string) =>
      apiClient.get(`/v1/users/${userId}/preferences`),
    updatePreferences: (userId: string, data: any) =>
      apiClient.put(`/v1/users/${userId}/preferences`, data),
    getKycStatus: (userId: string) =>
      apiClient.get(`/v1/users/${userId}/kyc-status`),
  },

  // Wallet (Rust wallet-core — balances, transactions)
  wallet: {
    getBalances: () => apiClient.get('/v1/wallet/balances'),
    getTransactions: (params?: any) =>
      apiClient.get('/v1/wallet/transactions', { params }),
  },

  // Payments (Go payment-service — crypto deposit/withdrawal via NOWPayments)
  payments: {
    // Create crypto deposit — returns pay_address + pay_amount
    initiateDeposit: (data: {
      amount: number
      currency: string  // e.g. "BTC", "ETH", "USDT", "USDC"
      idempotency_key: string
    }) => apiClient.post('/v1/payments/deposit', data),

    // Get deposit status
    getDeposit: (paymentUUID: string) =>
      apiClient.get(`/v1/payments/${paymentUUID}`),

    // Payment history
    getHistory: (params?: { limit?: number; cursor?: string; status?: string }) =>
      apiClient.get('/v1/payments/history', { params }),

    // Get supported crypto currencies
    getCurrencies: () => apiClient.get('/v1/payments/currencies'),

    // Initiate crypto withdrawal
    initiateWithdrawal: (data: {
      amount: number
      currency: string
      address: string
      idempotency_key: string
    }) => apiClient.post('/v1/payments/withdraw', data),

    // Get withdrawal status
    getWithdrawal: (uuid: string) =>
      apiClient.get(`/v1/payments/withdrawals/${uuid}`),

    // Withdrawal history
    getWithdrawalHistory: (params?: { limit?: number; cursor?: string }) =>
      apiClient.get('/v1/payments/withdrawals/history', { params }),
  },

  // Casino
  casino: {
    getGames: (params?: any) =>
      apiClient.get('/v1/casino/games', { params }),
    getGame: (gameId: string) =>
      apiClient.get(`/v1/casino/games/${gameId}`),
    launchGame: (data: { game_id: string; device_type: string; lobby_url?: string }) =>
      apiClient.post('/v1/casino/games/launch', data),
    getGameSession: (sessionId: string) =>
      apiClient.get(`/v1/casino/sessions/${sessionId}`),
    endGameSession: (sessionId: string) =>
      apiClient.post(`/v1/casino/sessions/${sessionId}/end`),
    getGameHistory: (params?: any) =>
      apiClient.get('/v1/casino/history', { params }),
    getProviders: () => apiClient.get('/v1/casino/providers'),
  },

  // Sportsbook (Rust betting-engine)
  sportsbook: {
    getEvents: (params?: { sport?: string; live?: boolean; limit?: number }) =>
      apiClient.get('/v1/sports/events', { params }),
    getEvent: (eventId: string) =>
      apiClient.get(`/v1/sports/events/${eventId}`),
    getMarkets: (eventId: string) =>
      apiClient.get(`/v1/sports/events/${eventId}/markets`),
    placeBet: (data: {
      selections: Array<{ event_id: string; market_id: string; outcome_id: string; odds: number }>
      stake: number
      currency: string
      idempotency_key: string
    }) => apiClient.post('/v1/bets/place', data),
    getBet: (betId: string) => apiClient.get(`/v1/bets/${betId}`),
    getBetHistory: (params?: { limit?: number; cursor?: string; status?: string }) =>
      apiClient.get('/v1/bets/history', { params }),
  },

  // Bonuses
  bonuses: {
    getList: (params?: { limit?: number; offset?: number }) =>
      apiClient.get('/v1/bonuses', { params }),
    getActive: () => apiClient.get('/v1/bonuses/active'),
    activate: (bonusId: string) =>
      apiClient.post(`/v1/bonuses/${bonusId}/activate`),
  },

  // Notifications
  notifications: {
    getList: (params?: any) =>
      apiClient.get('/v1/notifications', { params }),
    markAsRead: (notificationId: string) =>
      apiClient.post(`/v1/notifications/${notificationId}/read`),
    markAllAsRead: () => apiClient.post('/v1/notifications/read-all'),
    getSettings: () => apiClient.get('/v1/notifications/settings'),
    updateSettings: (data: any) =>
      apiClient.put('/v1/notifications/settings', data),
  },

  // Support
  support: {
    getTickets: () => apiClient.get('/v1/support/tickets'),
    createTicket: (data: { subject: string; message: string }) =>
      apiClient.post('/v1/support/tickets', data),
    getTicket: (ticketId: string) =>
      apiClient.get(`/v1/support/tickets/${ticketId}`),
    sendMessage: (ticketId: string, message: string) =>
      apiClient.post(`/v1/support/tickets/${ticketId}/messages`, { message }),
  },

  // KYC
  kyc: {
    getStatus: () => apiClient.get('/v1/kyc/status'),
    createApplicant: () => apiClient.post('/v1/kyc/applicant'),
    getSdkToken: () => apiClient.get('/v1/kyc/sdk-token'),
  },
}

export default apiClient
