## #28 nextjs-api-integration.skill.md

```markdown
# nextjs-api-integration.skill.md

## РОЛЬ
Ты создаёшь слой API-интеграции между Next.js фронтендом
и backend-сервисами гемблинг-платформы.

## КОНТЕКСТ
- Backend API: REST + JSON (внешний), WebSocket (real-time)
- Аутентификация: JWT (access 15min + refresh 7d)
- API Gateway: Kong (rate limiting, routing)
- Все ответы: единый формат { data, error, meta }

## СТРУКТУРА
src/lib/
├── api/
│ ├── client.ts # Axios/fetch instance
│ ├── interceptors.ts # Auth, error, retry
│ ├── types.ts # API response types
│ │
│ ├── auth.ts # Auth endpoints
│ ├── users.ts # User endpoints
│ ├── bets.ts # Betting endpoints
│ ├── wallet.ts # Wallet endpoints
│ ├── payments.ts # Payment endpoints
│ ├── games.ts # Casino endpoints
│ ├── bonuses.ts # Bonus endpoints
│ └── kyc.ts # KYC endpoints
│
├── ws/
│ ├── client.ts # WebSocket manager
│ ├── types.ts # WS message types
│ └── handlers.ts # Message handlers

text


## API CLIENT

```tsx
// lib/api/client.ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL;

interface ApiResponse<T> {
  data: T;
  meta?: {
    page?: number;
    total?: number;
    cursor?: string;
  };
}

interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  request_id: string;
}

class ApiClient {
  private baseUrl: string;
  
  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }
  
  private async request<T>(
    method: string,
    path: string,
    options: {
      body?: unknown;
      params?: Record<string, string>;
      headers?: Record<string, string>;
    } = {}
  ): Promise<ApiResponse<T>> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (options.params) {
      Object.entries(options.params).forEach(([k, v]) =>
        url.searchParams.set(k, v)
      );
    }
    
    const { getAccessToken, refreshTokens, clearAuth } = 
      useAuthStore.getState();
    
    let accessToken = getAccessToken();
    
    const makeRequest = async (token: string | null) => {
      const response = await fetch(url.toString(), {
        method,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...options.headers,
        },
        body: options.body ? JSON.stringify(options.body) : undefined,
      });
      return response;
    };
    
    let response = await makeRequest(accessToken);
    
    // Auto-refresh token на 401
    if (response.status === 401 && accessToken) {
      try {
        await refreshTokens();
        accessToken = useAuthStore.getState().accessToken;
        response = await makeRequest(accessToken);
      } catch {
        clearAuth();
        window.location.href = '/login';
        throw new Error('Session expired');
      }
    }
    
    if (!response.ok) {
      const error: ApiError = await response.json().catch(() => ({
        code: 'UNKNOWN_ERROR',
        message: response.statusText,
        request_id: response.headers.get('x-request-id') ?? 'unknown',
      }));
      throw new ApiClientError(response.status, error);
    }
    
    return response.json();
  }
  
  get<T>(path: string, params?: Record<string, string>) {
    return this.request<T>('GET', path, { params });
  }
  
  post<T>(path: string, body?: unknown) {
    return this.request<T>('POST', path, { body });
  }
  
  patch<T>(path: string, body?: unknown) {
    return this.request<T>('PATCH', path, { body });
  }
  
  delete<T>(path: string) {
    return this.request<T>('DELETE', path);
  }
}

export class ApiClientError extends Error {
  constructor(
    public status: number,
    public error: ApiError
  ) {
    super(error.message);
    this.name = 'ApiClientError';
  }
  
  get isNotFound() { return this.status === 404; }
  get isUnauthorized() { return this.status === 401; }
  get isForbidden() { return this.status === 403; }
  get isValidation() { return this.status === 422; }
  get isRateLimited() { return this.status === 429; }
}

export const api = new ApiClient(API_BASE!);
ТИПИЗИРОВАННЫЕ API МОДУЛИ
React

// lib/api/bets.ts
import { api } from './client';

export interface PlaceBetRequest {
  betType: 'single' | 'accumulator' | 'system';
  selections: {
    eventId: number;
    marketId: number;
    outcomeId: number;
    odds: number;
  }[];
  stake: number;
  currency: string;
  acceptOddsChanges: 'none' | 'higher' | 'any';
  idempotencyKey: string;
}

export interface BetResponse {
  id: number;
  status: 'accepted' | 'rejected';
  actualOdds: number;
  potentialWin: number;
  rejectionReason?: string;
}

export const betsApi = {
  placeBet: (data: PlaceBetRequest) =>
    api.post<BetResponse>('/api/v1/bets', data).then(r => r.data),
    
  getActive: () =>
    api.get<Bet[]>('/api/v1/bets/active').then(r => r.data),
    
  getHistory: (filters: BetFilters) =>
    api.get<Bet[]>('/api/v1/bets/history', toQueryParams(filters)),
    
  cashout: (betId: number) =>
    api.post<CashoutResponse>(`/api/v1/bets/${betId}/cashout`).then(r => r.data),
    
  getCashoutValue: (betId: number) =>
    api.get<{ amount: number }>(`/api/v1/bets/${betId}/cashout-value`)
      .then(r => r.data),
};
React

// lib/api/wallet.ts
export const walletApi = {
  getBalance: () =>
    api.get<WalletBalance>('/api/v1/wallet/balance').then(r => r.data),
    
  getTransactions: (filters: TxFilters) =>
    api.get<Transaction[]>('/api/v1/wallet/transactions', 
      toQueryParams(filters)),
};

// lib/api/auth.ts
export const authApi = {
  login: (data: LoginRequest) =>
    api.post<AuthTokens>('/api/v1/auth/login', data).then(r => r.data),
    
  register: (data: RegisterRequest) =>
    api.post<AuthTokens>('/api/v1/auth/register', data).then(r => r.data),
    
  refresh: (refreshToken: string) =>
    api.post<AuthTokens>('/api/v1/auth/refresh', { refreshToken })
      .then(r => r.data),
      
  logout: () => api.post('/api/v1/auth/logout'),
};
WEBSOCKET CLIENT
React

// lib/ws/client.ts
type MessageHandler = (data: any) => void;

class WebSocketManager {
  private ws: WebSocket | null = null;
  private subscriptions = new Map<string, Set<MessageHandler>>();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 1000;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  
  connect(token: string) {
    if (this.ws?.readyState === WebSocket.OPEN) return;
    
    this.ws = new WebSocket(
      `${process.env.NEXT_PUBLIC_WS_URL}?token=${token}`
    );
    
    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.startHeartbeat();
      // Переподписаться на все каналы
      this.subscriptions.forEach((_, channel) => {
        this.send({ action: 'subscribe', channel });
      });
    };
    
    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.type === 'pong') return;
      
      const handlers = this.subscriptions.get(msg.channel);
      handlers?.forEach((handler) => handler(msg.data));
    };
    
    this.ws.onclose = () => {
      this.stopHeartbeat();
      this.tryReconnect(token);
    };
  }
  
  subscribe(channel: string, handler: MessageHandler) {
    if (!this.subscriptions.has(channel)) {
      this.subscriptions.set(channel, new Set());
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.send({ action: 'subscribe', channel });
      }
    }
    this.subscriptions.get(channel)!.add(handler);
    
    // Возвращаем функцию отписки
    return () => {
      const handlers = this.subscriptions.get(channel);
      handlers?.delete(handler);
      if (handlers?.size === 0) {
        this.subscriptions.delete(channel);
        this.send({ action: 'unsubscribe', channel });
      }
    };
  }
  
  private send(data: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }
  
  private startHeartbeat() {
    this.heartbeatInterval = setInterval(() => {
      this.send({ type: 'ping' });
    }, 30_000);
  }
  
  private stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
    }
  }
  
  private tryReconnect(token: string) {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;
    
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts);
    this.reconnectAttempts++;
    
    setTimeout(() => this.connect(token), Math.min(delay, 30_000));
  }
  
  disconnect() {
    this.stopHeartbeat();
    this.ws?.close();
    this.ws = null;
    this.subscriptions.clear();
  }
}

export const wsManager = new WebSocketManager();
IDEMPOTENCY KEY
React

// lib/utils/idempotency.ts
import { v4 as uuidv4 } from 'uuid';

// Генерировать idempotency key для финансовых операций
export function generateIdempotencyKey(): string {
  return uuidv4();
}

// Хук для использования в формах
export function useIdempotencyKey() {
  const [key, setKey] = useState(generateIdempotencyKey);
  const regenerate = useCallback(() => setKey(uuidv4()), []);
  return { key, regenerate };
}
ERROR HANDLING
React

// lib/api/errors.ts
import { ApiClientError } from './client';

export function getErrorMessage(error: unknown): string {
  if (error instanceof ApiClientError) {
    // Маппинг серверных кодов на пользовательские сообщения
    const messages: Record<string, string> = {
      'WALLET_INSUFFICIENT_BALANCE': 'Недостаточно средств',
      'BET_ODDS_CHANGED': 'Коэффициенты изменились',
      'BET_EVENT_SUSPENDED': 'Событие приостановлено',
      'BET_MARKET_CLOSED': 'Рынок закрыт',
      'AUTH_INVALID_CREDENTIALS': 'Неверный email или пароль',
      'AUTH_ACCOUNT_LOCKED': 'Аккаунт заблокирован',
      'RATE_LIMITED': 'Слишком много запросов',
    };
    return messages[error.error.code] ?? error.error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'Произошла неизвестная ошибка';
}
АНТИПАТТЕРНЫ
React

// ❌ ПЛОХО: fetch напрямую в компоненте
function Profile() {
  useEffect(() => {
    fetch('/api/v1/users/me')
      .then(r => r.json())
      .then(setUser);
  }, []);
}

// ✅ ПРАВИЛЬНО: через api модуль + TanStack Query
function Profile() {
  const { data: user } = useUser();
}

// ❌ ПЛОХО: хардкод URL
fetch('https://api.example.com/v1/bets')

// ✅ ПРАВИЛЬНО: через env + client
api.get('/api/v1/bets')

// ❌ ПЛОХО: не обрабатывать 401/refresh
const response = await fetch(url, { headers: { Authorization: token } });

// ✅ ПРАВИЛЬНО: автоматический refresh в interceptor (см. client.ts)

// ❌ ПЛОХО: финансовые операции без idempotency key
api.post('/api/v1/bets', { stake: 100 })

// ✅ ПРАВИЛЬНО: всегда передавать idempotency key
api.post('/api/v1/bets', { stake: 100, idempotencyKey: generateIdempotencyKey() })