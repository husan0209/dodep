# CORS Configuration

**Phase:** 0.4 Stabilization
**Owner:** All HTTP-facing services

This document defines the CORS allowlist contract for every Opus Casino service that exposes a browser-reachable HTTP endpoint.

---

## Contract

1. **Single source of truth:** `CORS_ORIGINS` environment variable, comma-separated.
2. **No wildcards:** `*` is explicitly stripped — it is unsafe alongside `AllowCredentials: true` and the platform sets credentials.
3. **Empty allowlist = deny browser CORS.** Server-to-server, redirect navigation (`<a href>` clicks, 302 to `/r/:code`) and WebSocket upgrades from a non-browser client are unaffected.
4. **`AllowCredentials` is enabled only when at least one origin is configured.** Browsers reject credential requests with wildcards anyway.
5. **Allowed headers** (uniform across all services):
   `Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Idempotency-Key`.

## Per-service status

| Service | Stack | Implementation | Status |
|---|---|---|---|
| `services/go/auth` | Fiber | `corsOriginsFromEnv` in `main.go` | ✅ ENV-driven |
| `services/go/admin-bff` | Fiber | `corsOriginsFromEnv` in `main.go` | ✅ ENV-driven |
| `services/go/affiliate` | Fiber | `corsOriginsFromEnv` in `main.go` | ✅ ENV-driven (was `*`) |
| `services/rust/betting-engine` | Axum + tower-http | `state.config().cors_allow_origins` | ✅ ENV-driven |
| `services/rust/websocket-gateway` | Axum + tower-http | `cors_layer_from_env()` in `api/mod.rs` | ✅ ENV-driven (was `permissive`) |

Other Go services (`payment`, `user`, `notification`, `bonus`, `casino`, `kyc`) expose **only gRPC** — no CORS surface.

## Recommended values

### Local development

```bash
# .env.dev / docker-compose.dev.yml
CORS_ORIGINS=http://localhost:3000,http://localhost:3001,http://127.0.0.1:3000,http://127.0.0.1:3001
```

### Staging

```bash
CORS_ORIGINS=https://staging.opus-casino.com,https://admin-staging.opus-casino.com
```

### Production

```bash
CORS_ORIGINS=https://opus-casino.com,https://admin.opus-casino.com
```

Set per-service via Helm `values-prod.yaml`:

```yaml
env:
  - name: CORS_ORIGINS
    valueFrom:
      configMapKeyRef:
        name: opus-casino-cors
        key: origins
```

## Verification

### Manual smoke test

```bash
# preflight from a forbidden origin → 403 (or no Access-Control-Allow-Origin)
curl -i -X OPTIONS https://api.opus-casino.com/api/v1/users/1/bets \
  -H "Origin: https://evil.com" \
  -H "Access-Control-Request-Method: POST"
# expect: no `Access-Control-Allow-Origin` header in response

# preflight from an allowed origin → 200
curl -i -X OPTIONS https://api.opus-casino.com/api/v1/users/1/bets \
  -H "Origin: https://opus-casino.com" \
  -H "Access-Control-Request-Method: POST"
# expect: `Access-Control-Allow-Origin: https://opus-casino.com`
#         `Access-Control-Allow-Credentials: true`
```

### CI grep guard

`grep -rE 'AllowOrigins.*"\*"|CorsLayer::permissive\(\)' services/` should return no hits. (Phase 0.4 sweep verified this manually; consider a CI step in Phase 1.)

## What NOT to do

- ❌ `AllowOrigins: "*"` with `AllowCredentials: true` — browsers reject this, but it indicates intent to weaken security.
- ❌ Hard-coded origins in source — environment-specific config should not require recompile.
- ❌ Mirroring `Origin` header into `Access-Control-Allow-Origin` — that bypasses the allowlist entirely. (None of our services do this; called out for code reviewers.)

## Migration notes (changes in Phase 0.4)

| File | Before | After |
|---|---|---|
| `services/go/affiliate/main.go` | `AllowOrigins: "*"` | `corsOriginsFromEnv()` (no fallback — empty in non-dev) |
| `services/go/auth/main.go` | hard-coded list of 6 localhost origins | `corsOriginsFromEnv(devDefault)` with same dev list as fallback |
| `services/go/admin-bff/main.go` | hard-coded `localhost:3001` pair | `corsOriginsFromEnv(devDefault)` with same dev list as fallback |
| `services/rust/websocket-gateway/src/api/mod.rs` | `CorsLayer::permissive()` | `cors_layer_from_env()` |
| `services/rust/betting-engine/src/api/mod.rs` | already ENV-driven, but no `"*"` filter | added explicit `s != "*"` filter and `allow_credentials(true)` |

No public API contract changed — only the rejection behavior for unauthorized origins is now consistent.
