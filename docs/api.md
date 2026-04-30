# API Документация

## Base URL

- Production: `https://api.opus-casino.com`
- Staging: `https://api-staging.opus-casino.com`
- Development: `http://localhost:8080`

## Аутентификация

Все запросы к API требуют JWT токен в заголовке:

```
Authorization: Bearer <access_token>
```

## Auth Service

### POST /api/v1/auth/register

Регистрация нового пользователя.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "username": "username",
  "country": "US",
  "currency": "USD"
}
```

**Response:**
```json
{
  "user_id": "uuid",
  "tokens": {
    "access_token": "jwt",
    "refresh_token": "jwt",
    "expires_in": 900
  }
}
```

### POST /api/v1/auth/login

Вход в систему.

### POST /api/v1/auth/refresh

Обновление access токена.

### POST /api/v1/auth/logout

Выход из системы.

## Betting Service

### POST /api/v1/bets/place

Разместить ставку.

**Request:**
```json
{
  "event_id": "event_123",
  "market_id": "market_456",
  "selection_id": "selection_789",
  "stake": {
    "value": "100.00",
    "currency": "USD"
  },
  "odds": "1.50"
}
```

**Response:**
```json
{
  "bet_id": "uuid",
  "status": "accepted",
  "potential_win": {
    "value": "150.00",
    "currency": "USD"
  }
}
```

### GET /api/v1/bets/:id

Получить информацию о ставке.

### GET /api/v1/bets/user/:user_id

Получить ставки пользователя.

## Wallet Service

### GET /api/v1/wallet/balance

Получить баланс кошелька.

### POST /api/v1/wallet/deposit

Пополнить кошелек.

### POST /api/v1/wallet/withdraw

Вывести средства.

### GET /api/v1/wallet/transactions

История транзакций.

## WebSocket API

### Подключение

```
wss://ws.opus-casino.com/ws
```

### Сообщения

**Подписка на odds:**
```json
{
  "type": "subscribe",
  "channel": "odds",
  "event_ids": ["event_1", "event_2"]
}
```

**Получение обновления:**
```json
{
  "type": "odds_update",
  "event_id": "event_1",
  "market_id": "market_1",
  "odds": "1.55",
  "timestamp": 1234567890
}
```
