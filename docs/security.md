# Безопасность

## Аутентификация

### Password Hashing

- Алгоритм: **Argon2id**
- Parameters:
  - Memory: 64 MB
  - Iterations: 3
  - Parallelism: 4

### JWT Tokens

- Access token: Ed25519 signed, 15 минут
- Refresh token: Opaque, 7 дней
- Хранение: HttpOnly cookies

### 2FA

- TOTP (RFC 6238)
- WebAuthn (FIDO2)
- SMS fallback

## Шифрование

### In Transit

- TLS 1.3 (минимум)
- HSTS enabled
- Certificates: Let's Encrypt (auto-renewal)

### At Rest

- Database: AES-256-GCM
- S3: Server-side encryption
- Backups: Encrypted

### Key Management

- HashiCorp Vault
- AWS CloudHSM для master keys
- Auto-rotation каждые 90 дней

## API Protection

### Rate Limiting

Token Bucket алгоритм:

| Endpoint | Limit |
|----------|-------|
| /api/auth/login | 5/min/IP |
| /api/auth/register | 3/min/IP |
| /api/bets/place | 10/sec/user |
| /api/wallet/* | 20/sec/user |
| General API | 100/sec/IP |

### WAF (CloudFlare)

Правила:
- SQL Injection protection
- XSS protection
- Path traversal protection
- Custom rules для gambling patterns

### Bot Detection

CloudFlare Bot Management:
- JS fingerprinting
- Behavioral analysis
- CAPTCHA для подозрительных

## Compliance

### KYC/AML

- Provider: Sumsub
- AML screening: ComplyAdvantage
- PEP/Sanctions check
- Ongoing monitoring

### RNG Certification

- Provider: GLI / eCOGRA
- Annual audit
- Statistical testing

### GDPR

- Data minimization
- Right to erasure
- Data portability
- Consent management

## Security Scanning

### CI/CD

- **Trivy** — vulnerability scanning
- **Semgrep** — SAST
- **OWASP ZAP** — DAST

### Container Security

- Base images: Distroless
- Non-root users
- Read-only root filesystem
- Dropped capabilities

### Network Security

- mTLS между сервисами (Istio)
- Network policies
- Private subnets
- No direct internet access

## Incident Response

### Security Contacts

- Security team: security@opus-casino.com
- Emergency: +1-XXX-XXX-XXXX

### Runbook

1. Detect → 2. Contain → 3. Eradicate → 4. Recover → 5. Lessons learned

### Logging

Все security события логируются:
- Authentication attempts
- Authorization failures
- Rate limit hits
- WAF blocks
- Admin actions
