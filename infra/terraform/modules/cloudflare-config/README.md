# CloudFlare Configuration Module

Production-ready CloudFlare модуль для Opus Casino с DNS, WAF, Rate Limiting, SSL/TLS и Bot Management.

## Особенности

- **DNS Management** — управление DNS записями
- **WAF (Web Application Firewall)** — OWASP Top 10 + custom rules
- **Rate Limiting** — защита от DDoS и brute force
- **SSL/TLS** — Full (Strict) mode с Origin CA
- **Bot Fight Mode** — автоматическая детекция ботов
- **Load Balancer** — geo-based routing + health checks
- **Logpush** — логирование в S3

## Использование

```hcl
module "cloudflare" {
  source = "../modules/cloudflare-config"

  account_id = var.cloudflare_account_id
  domain     = "opus-casino.com"
  plan       = "business"

  # DNS Records
  dns_records = {
    www = {
      name    = "@"
      type    = "A"
      value   = "203.0.113.10"
      proxied = true
    }
    api = {
      name    = "api"
      type    = "CNAME"
      value   = "lb.opus-casino.com"
      proxied = true
    }
    admin = {
      name    = "admin"
      type    = "A"
      value   = "203.0.113.20"
      proxied = true
    }
  }

  # SSL/TLS
  enable_origin_ca   = true
  origin_hostnames   = ["opus-casino.com", "*.opus-casino.com", "api.opus-casino.com"]

  # Blocked Countries (gambling restrictions)
  blocked_countries = ["KP", "IR", "SY", "CU"]

  # Trusted ASNs (office IPs, partners)
  trusted_asns = [15169, 16509]  # Google, Amazon

  # Rate Limits
  rate_limits = {
    login_per_minute      = 10
    registration_per_hour = 5
    api_per_minute        = 100
    bet_per_10_seconds    = 5
    withdrawal_per_hour   = 10
  }

  # Bot Fight Mode
  enable_bot_fight_mode = true

  # Load Balancer
  enable_load_balancer = true
  primary_origin       = "203.0.113.10"
  secondary_origin     = "203.0.113.11"
  fallback_origin      = "203.0.113.20"
  api_origins          = ["203.0.113.30", "203.0.113.31"]

  # Logpush
  enable_logpush   = true
  logpush_bucket   = "production-opus-logs"
  logpush_region   = "us-east-1"

  tags = {
    Project = "opus-casino"
  }
}
```

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                      CloudFlare Edge                         │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  DNS + SSL/TLS (Full Strict)                         │   │
│  │                                                       │   │
│  │  opus-casino.com ──► 203.0.113.10 (proxied)          │   │
│  │  api.opus-casino.com ──► lb.opus-casino.com          │   │
│  │  admin.opus-casino.com ──► 203.0.113.20              │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  WAF Rules                                           │   │
│  │                                                       │   │
│  │  ┌─────────────────┐  ┌─────────────────┐           │   │
│  │  │  OWASP Top 10   │  │  Custom Rules   │           │   │
│  │  │  - SQLi         │  │  - Bot UAs      │           │   │
│  │  │  - XSS          │  │  - Auth protect │           │   │
│  │  │  - Path Traversal│ │  - Header check │           │   │
│  │  └─────────────────┘  └─────────────────┘           │   │
│  │                                                       │   │
│  │  ┌─────────────────┐  ┌─────────────────┐           │   │
│  │  │  Rate Limiting  │  │  Bot Fight Mode │           │   │
│  │  │  - Login: 10/min│  │  - Auto detect  │           │   │
│  │  │  - API: 100/min │  │  - Challenge    │           │   │
│  │  │  - Bets: 5/10s  │  │  - Block        │           │   │
│  │  └─────────────────┘  └─────────────────┘           │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Load Balancer (Geo + Health Checks)                 │   │
│  │                                                       │   │
│  │  Primary Pool:    203.0.113.10, 203.0.113.11         │   │
│  │  API Pool:        203.0.113.30, 203.0.113.31         │   │
│  │  Fallback Pool:   203.0.113.20                       │   │
│  │                                                       │   │
│  │  Health Check: /health (30s interval)                │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Logpush → S3                                        │   │
│  │  (all HTTP requests с detailed fields)               │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## WAF Rules

### OWASP Protection

- **SQL Injection** — блокировка UNION SELECT, INSERT INTO и т.д.
- **XSS** — блокировка `<script>`, `javascript:`
- **Path Traversal** — блокировка `../`, `..\\`
- **Blocked Countries** — гео-блокировка по списку

### Custom Gambling Protection

```hcl
# Block suspicious user agents
rules {
  action = "block"
  expression = "(http.user_agent contains \"python-requests\") or (http.user_agent contains \"curl\")"
}

# Challenge on auth endpoints
rules {
  action = "challenge"
  expression = "(http.request.uri.path contains \"/api/auth/\")"
}

# Block requests without standard headers
rules {
  action = "block"
  expression = "(not http.request.headers[\"accept\"][0])"
}
```

## Rate Limiting

| Endpoint | Limit | Период | Block |
|----------|-------|--------|-------|
| `/api/auth/login` | 10 | 1 мин | 5 мин |
| `/api/auth/register` | 5 | 1 час | 1 час |
| `/api/*` (authenticated) | 100 | 1 мин | 1 мин |
| `/api/bets/place` | 5 | 10 сек | 30 сек |
| `/api/wallet/withdraw` | 10 | 1 час | 1 час |

## SSL/TLS Configuration

### Recommended Settings

```hcl
always_use_https         = "on"
automatic_https_rewrites = "on"
tls_1_3                  = "on"
min_tls_version          = "1.3"
```

### Origin CA Certificate

CloudFlare Origin CA — бесплатный SSL сертификат для backend:

```hcl
enable_origin_ca   = true
origin_hostnames   = ["opus-casino.com", "*.opus-casino.com"]
```

Срок действия: 365 дней (автоматически renew через Terraform).

## Load Balancer

### Конфигурация

```hcl
enable_load_balancer = true
primary_origin       = "203.0.113.10"  # US-East
secondary_origin     = "203.0.113.11"  # US-West
fallback_origin      = "203.0.113.20"  # DR site
api_origins          = ["203.0.113.30", "203.0.113.31"]
```

### Steering Policy

- **Geo** — маршрутизация по геолокации
- **Least Outstanding Requests** — балансировка по загрузке
- **Session Affinity** — cookie-based (30 мин)

### Health Checks

```hcl
check_regions = ["WNAM", "ENAM", "WEU"]
interval      = 30s
timeout       = 5s
retries       = 3
path          = "/health"
```

## Logpush

### Поля для логирования

- RayID, ClientIP, ClientRequestHost, ClientRequestMethod
- ClientRequestURI, ClientRequestProtocol, ClientRequestUserAgent
- EdgeStartTimestamp, EdgeEndTimestamp, EdgeResponseStatus
- EdgeCacheStatus, SecurityLevel, SmartType
- OriginResponseStatus, ClientSSLProtocol
- SecurityRuleID, SecurityRuleMessage, SecurityRuleActions
- ClientBotScore, ClientVerifiedBot, ClientCountry

### S3 Destination

```
s3://production-opus-logs?region=us-east-1&accountid=<account_id>
```

## Стоимость (Business Plan)

```
CloudFlare Business Plan:                     $200/месяц

Load Balancer:
  3 pools × 2 origins × $10                  = $60/месяц
  Health checks (3 regions × 100K checks)    = $30/месяц
  Requests (10M requests)                    = $50/месяц

Logpush:
  100GB logs → S3                            = $0 (CloudFlare free)
  S3 storage + transfer                      = ~$5/месяц

─────────────────────────────────────────────────────
Итого: ~$345/месяц
```

## Best Practices

### Безопасность

1. WAF на maximum protection для production
2. Rate limiting на всех критичных endpoints
3. Bot Fight Mode включён
4. Geo-blocking для restricted countries
5. TLS 1.3 minimum

### Производительность

1. Aggressive cache level для статики
2. Brotli compression включён
3. Minify для CSS/JS/HTML
4. Always Use HTTPS

### Надёжность

1. Load Balancer с health checks
2. Fallback pool для failover
3. Session affinity для stateful connections
4. Multiple origins в разных регионах

## DNS Records Examples

```hcl
dns_records = {
  # Main website
  root = {
    name    = "@"
    type    = "A"
    value   = "203.0.113.10"
    proxied = true
  }
  
  # API (через load balancer)
  api = {
    name    = "api"
    type    = "CNAME"
    value   = "lb.opus-casino.com"
    proxied = true
  }
  
  # Admin panel
  admin = {
    name    = "admin"
    type    = "A"
    value   = "203.0.113.20"
    proxied = true
  }
  
  # WebSocket gateway
  ws = {
    name    = "ws"
    type    = "CNAME"
    value   = "ws-gateway.opus-casino.com"
    proxied = false  # WebSocket не через proxy
  }
  
  # MX records для email
  mx = {
    name    = "@"
    type    = "MX"
    value   = "mx1.emailprovider.com"
    ttl     = 3600
    proxied = false
  }
}
```

## Blocked Countries

Список стран для блокировки (gambling restrictions):

```hcl
blocked_countries = [
  "KP",  # North Korea
  "IR",  # Iran
  "SY",  # Syria
  "CU",  # Cuba
  "US",  # USA (если нет лицензии)
  "FR",  # France (требуется лицензия)
  "IT",  # Italy (требуется лицензия)
  "ES",  # Spain (требуется лицензия)
]
```
