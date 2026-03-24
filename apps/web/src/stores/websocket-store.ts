import { create } from 'zustand'

interface OddsUpdate {
  eventId: string
  selectionId: string
  oldOdds: number
  newOdds: number
  timestamp: string
}

interface BetSettlement {
  betId: string
  status: 'won' | 'lost' | 'void' | 'refund'
  winAmount?: number
}

interface WebSocketState {
  isConnected: boolean
  lastMessage: any | null
  oddsUpdates: OddsUpdate[]
  betSettlements: BetSettlement[]
  
  // Actions
  connect: () => void
  disconnect: () => void
  subscribe: (channel: string) => void
  unsubscribe: (channel: string) => void
  send: (message: any) => void
}

let ws: WebSocket | null = null
let reconnectTimeout: NodeJS.Timeout | null = null

export const useWebSocketStore = create<WebSocketState>((set, get) => ({
  isConnected: false,
  lastMessage: null,
  oddsUpdates: [],
  betSettlements: [],

  connect: () => {
    const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
    
    try {
      ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        set({ isConnected: true })
        console.log('WebSocket connected')
      }

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data)
          
          set({ lastMessage: message })

          // Handle different message types
          switch (message.type) {
            case 'odds_update':
              set((state) => ({
                oddsUpdates: [message.data, ...state.oddsUpdates].slice(0, 100),
              }))
              break
            case 'bet_settlement':
              set((state) => ({
                betSettlements: [message.data, ...state.betSettlements].slice(0, 50),
              }))
              break
            case 'notification':
              // Handle real-time notification
              break
          }
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      ws.onclose = () => {
        set({ isConnected: false })
        console.log('WebSocket disconnected')

        // Attempt to reconnect
        reconnectTimeout = setTimeout(() => {
          get().connect()
        }, 5000)
      }

      ws.onerror = (error) => {
        console.error('WebSocket error:', error)
      }
    } catch (error) {
      console.error('Failed to connect WebSocket:', error)
    }
  },

  disconnect: () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    
    if (ws) {
      ws.close()
      ws = null
    }
    
    set({ isConnected: false })
  },

  subscribe: (channel: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'subscribe', channel }))
    }
  },

  unsubscribe: (channel: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'unsubscribe', channel }))
    }
  },

  send: (message: any) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message))
    }
  },
}))

// Auto-connect on mount (client-side only)
if (typeof window !== 'undefined') {
  // Connect when needed, not automatically
  // useWebSocketStore.getState().connect()
}
