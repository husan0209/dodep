import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios'
import { getAuthToken, clearAuthToken } from './auth'

// Base API client
export const apiClient = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
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
      apiClient.post('/api/v1/auth/login', data),
    register: (data: { email: string; password: string; username: string }) =>
      apiClient.post('/api/v1/auth/register', data),
    logout: () => apiClient.post('/api/v1/auth/logout'),
    me: () => apiClient.get('/api/v1/auth/me'),
    refresh: () => apiClient.post('/api/v1/auth/refresh'),
  },

  // User
  user: {
    get: (userId: string) => apiClient.get(`/api/v1/users/${userId}`),
    update: (userId: string, data: any) =>
      apiClient.put(`/api/v1/users/${userId}`, data),
    getPreferences: (userId: string) =>
      apiClient.get(`/api/v1/users/${userId}/preferences`),
    updatePreferences: (userId: string, data: any) =>
      apiClient.put(`/api/v1/users/${userId}/preferences`, data),
    getKycStatus: (userId: string) =>
      apiClient.get(`/api/v1/users/${userId}/kyc-status`),
  },

  // Wallet
  wallet: {
    getBalances: () => apiClient.get('/api/v1/wallet/balances'),
    getTransactions: (params?: any) =>
      apiClient.get('/api/v1/wallet/transactions', { params }),
    deposit: (data: { amount: number; method: string }) =>
      apiClient.post('/api/v1/wallet/deposit', data),
    withdraw: (data: { amount: number; method: string }) =>
      apiClient.post('/api/v1/wallet/withdraw', data),
  },

  // Casino
  casino: {
    getGames: (params?: any) =>
      apiClient.get('/api/v1/casino/games', { params }),
    getGame: (gameId: string) =>
      apiClient.get(`/api/v1/casino/games/${gameId}`),
    launchGame: (data: { gameId: string; deviceType: string }) =>
      apiClient.post('/api/v1/casino/games/launch', data),
    getGameSession: (sessionId: string) =>
      apiClient.get(`/api/v1/casino/sessions/${sessionId}`),
    endGameSession: (sessionId: string) =>
      apiClient.post(`/api/v1/casino/sessions/${sessionId}/end`),
    getGameHistory: (params?: any) =>
      apiClient.get('/api/v1/casino/history', { params }),
    getProviders: () => apiClient.get('/api/v1/casino/providers'),
  },

  // Bonuses
  bonuses: {
    getList: () => apiClient.get('/api/v1/bonuses'),
    activate: (bonusId: string) =>
      apiClient.post(`/api/v1/bonuses/${bonusId}/activate`),
    getWagering: () => apiClient.get('/api/v1/bonuses/wagering'),
  },

  // Notifications
  notifications: {
    getList: (params?: any) =>
      apiClient.get('/api/v1/notifications', { params }),
    markAsRead: (notificationId: string) =>
      apiClient.post(`/api/v1/notifications/${notificationId}/read`),
    markAllAsRead: () => apiClient.post('/api/v1/notifications/read-all'),
    getSettings: () => apiClient.get('/api/v1/notifications/settings'),
    updateSettings: (data: any) =>
      apiClient.put('/api/v1/notifications/settings', data),
  },

  // Support
  support: {
    getTickets: () => apiClient.get('/api/v1/support/tickets'),
    createTicket: (data: { subject: string; message: string }) =>
      apiClient.post('/api/v1/support/tickets', data),
    getTicket: (ticketId: string) =>
      apiClient.get(`/api/v1/support/tickets/${ticketId}`),
    sendMessage: (ticketId: string, message: string) =>
      apiClient.post(`/api/v1/support/tickets/${ticketId}/messages`, { message }),
  },
}

export default apiClient
