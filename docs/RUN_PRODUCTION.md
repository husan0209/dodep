# Running Opus Casino in Production

Single-VPS deployment via Docker Compose.

## Requirements

| Item | Version |
|------|---------|
| Server OS | Ubuntu 22.04 LTS (or Debian 12) |
| Docker Engine | 24+ |
| Docker Compose | 2.x (plugin) |
| RAM | 16 GB minimum, 32 GB recommended |
| CPU | 8 vCPU |
| Disk | 100 GB NVMe SSD |

---

## 1 — Clone & Configure

```bash
git clone https://github.com/husan0209/dodep.git /opt/opus-casino
cd /opt/opus-casino

# Copy and fill in production secrets
cp .env.production.example .env.production
nano .env.production      # Fill in all __CHANGE_ME__ values
```

**Required secrets to set:**
- `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `CLICKHOUSE_PASSWORD`
- `JWT_ED25519_PRIVATE_KEY`, `JWT_ED25519_PUBLIC_KEY` — generate with:
  ```bash
  openssl genpkey -algorithm ed25519 -out ed25519-priv.pem
  openssl pkey -in ed25519-priv.pem -pubout -outform DER | base64 -w0
  openssl pkey -in ed25519-priv.pem -outform DER | base64 -w0
  ```
- `NOWPAYMENTS_API_KEY`, `NOWPAYMENTS_IPN_SECRET`, `NOWPAYMENTS_IPN_CALLBACK_URL`
- `PRAGMATIC_AGENT_ID`, `PRAGMATIC_SECRET_KEY` (and other providers)
- `APP_DOMAIN` — your actual domain (e.g. `casino.example.com`)

---

## 2 — SSL Certificates

```bash
# Using Let's Encrypt (certbot)
apt install -y certbot
certbot certonly --standalone -d casino.example.com -d www.casino.example.com

# Copy certs to nginx volume
cp /etc/letsencrypt/live/casino.example.com/fullchain.pem \
   /var/lib/docker/volumes/opus-casino_nginx_certs/_data/fullchain.pem
cp /etc/letsencrypt/live/casino.example.com/privkey.pem \
   /var/lib/docker/volumes/opus-casino_nginx_certs/_data/privkey.pem
```

---

## 3 — CloudFlare Setup

1. Point DNS A-record `casino.example.com` → VPS IP.
2. Enable CloudFlare proxy (orange cloud) — WAF + DDoS included on free tier.
3. SSL/TLS mode: **Full (Strict)**.
4. Create Page Rule: `casino.example.com/api/*` → Cache Level: Bypass.

---

## 4 — First Launch

```bash
cd /opt/opus-casino

# Start all services
make prod-up

# Check status
make prod-ps

# Wait ~2 minutes for services to be healthy, then run migrations
make prod-migrate

# Seed reference data (currencies, countries, KYC limits, providers)
make prod-seed

# Verify smoke
make smoke
```

---

## 5 — Enable Slot Providers (in order)

Enable one at a time and monitor for 24h before enabling the next:

```bash
# 1. PG Soft
# Set PGSOFT_ENABLED=true in .env.production + restart casino
docker compose -f infra/docker/docker-compose.prod.yml --env-file .env.production \
  restart casino

# 2. Pragmatic Play — repeat after PG Soft is stable
# 3. Amatic
# 4. Amusnet
```

Monitor in Grafana at `http://grafana.casino.example.com` (login: admin / see GRAFANA_ADMIN_PASSWORD):
- `casino_callbacks_total{provider="pgsoft", status="ok"}`
- `casino_signature_invalid_total`
- `casino_replay_window_violations_total`

---

## 6 — Health Verification

```bash
# All services healthy
curl https://casino.example.com/api/v1/auth/health
curl https://casino.example.com/api/v1/payments/healthz

# NOWPayments connectivity
curl -H "x-api-key: YOUR_KEY" https://api.nowpayments.io/v1/status
```

---

## 7 — Ongoing Operations

```bash
make prod-logs          # Stream all logs
make prod-backup        # Manual DB backup
make prod-ps            # Service status
make prod-restart       # Restart all services

# Update to latest build
git pull
make prod-build
make prod-up            # Rolling restart
```

---

## 8 — Monitoring URLs

| Service | URL |
|---------|-----|
| Grafana | http://grafana.casino.example.com |
| Jaeger | http://VPS_IP:16686 (restrict to VPN) |
| Prometheus | http://VPS_IP:9090 (restrict to VPN) |

---

## 9 — Backup & Recovery

Backups run every 6 hours automatically to MinIO (`opus-casino-backups` bucket).

**Restore from backup:**
```bash
# List available backups
docker compose exec backup mc ls minio/opus-casino-backups/postgres/

# Download and restore
docker compose exec backup mc cp minio/opus-casino-backups/postgres/BACKUP_FILE.sql.gz /tmp/
gunzip /tmp/BACKUP_FILE.sql.gz
PGPASSWORD=$POSTGRES_PASSWORD psql -h localhost -U $POSTGRES_USER $POSTGRES_DB < /tmp/BACKUP_FILE.sql
```
