---

# SKILL #54 — casino-integration.skill.md

```markdown
# casino-integration.skill.md
# GAMBLING PLATFORM — CASINO GAME INTEGRATION
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent, Frontend Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Casino games are NOT built by us. We integrate via an aggregator
(SoftSwiss, Slotegrator, or similar) that connects to 50+ game
providers (Pragmatic Play, NetEnt, Evolution, etc.).

Casino Service is written in Go.
Our role: Wallet API (aggregator calls US), game catalog, sessions.

# ============================================================
# SECTION 2: ARCHITECTURE
# ============================================================

```text
┌─────────┐       ┌──────────────┐       ┌──────────────┐
│  Player  │──────▶│  Our Casino  │──────▶│  Aggregator  │
│  (Web/   │  HTTP │  Service     │  HTTP │  (SoftSwiss)  │
│   Mobile)│◀──────│  (Go)        │◀──────│              │
└─────────┘       └──────┬───────┘       └──────┬───────┘
                         │                       │
                         │ gRPC                   │ API
                         ▼                       ▼
                  ┌──────────────┐       ┌──────────────┐
                  │   Wallet     │       │   Game       │
                  │   Service    │       │   Providers  │
                  │   (Rust)     │       │   (50+)      │
                  └──────────────┘       └──────────────┘

TWO API DIRECTIONS:
  1. WE → AGGREGATOR: Get game list, launch game URL
  2. AGGREGATOR → US: Wallet operations (bet, win, rollback)
============================================================
SECTION 3: WALLET API (aggregator calls us)
============================================================
ENDPOINTS
Go

// These endpoints are called BY the aggregator, not by our frontend.
// They MUST respond in < 50ms (aggregator SLA).

// GET balance
POST /api/v1/casino/wallet/balance
  Input:  { player_id, currency, game_id, session_id }
  Output: { balance: "150.00" }

// Place bet (debit)
POST /api/v1/casino/wallet/bet
  Input:  { player_id, currency, amount, game_id, 
            round_id, transaction_id }
  Output: { balance: "140.00", transaction_id: "our_tx_123" }

// Award win (credit)  
POST /api/v1/casino/wallet/win
  Input:  { player_id, currency, amount, game_id,
            round_id, transaction_id, reference_transaction_id }
  Output: { balance: "180.00", transaction_id: "our_tx_124" }

// Rollback (cancel previous bet/win)
POST /api/v1/casino/wallet/rollback
  Input:  { player_id, currency, amount, game_id,
            round_id, transaction_id, reference_transaction_id }
  Output: { balance: "150.00" }
IMPLEMENTATION RULES
text

RULE 1: IDEMPOTENCY by transaction_id
  If we receive the same transaction_id twice → return same response
  Check DragonflyDB first, then PostgreSQL UNIQUE constraint

RULE 2: ORDERING — bet BEFORE win
  A win always references a bet (reference_transaction_id)
  If win arrives before bet (network race) → retry or queue

RULE 3: ROLLBACK reverses a specific transaction
  Rollback of bet → credit amount back
  Rollback of win → debit amount back
  Rollback of rollback → not allowed (return error)

RULE 4: ROUND SEMANTICS
  A round = one spin/hand/game
  A round can have: 1 bet + 0-N wins + 0-N rollbacks
  Round is "closed" when no activity for 60 seconds

RULE 5: RESPONSE TIME < 50ms
  Aggregator has strict SLA
  If we respond slow → game freezes for player
  Use DragonflyDB cache for balance reads
Go

// Handler for casino bet (debit)
func (h *CasinoHandler) Bet(c *fiber.Ctx) error {
    var req CasinoBetRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(CasinoError{Code: "INVALID_REQUEST"})
    }

    // Verify aggregator authentication (API key / signature)
    if !h.verifyAggregatorAuth(c) {
        return c.Status(401).JSON(CasinoError{Code: "UNAUTHORIZED"})
    }

    // Map external player_id to internal user_id
    userID, err := h.mapPlayerID(c.Context(), req.PlayerID)
    if err != nil {
        return c.Status(404).JSON(CasinoError{Code: "PLAYER_NOT_FOUND"})
    }

    // Check responsible gambling (session time, loss limits)
    if err := h.checkLimits(c.Context(), userID); err != nil {
        return c.Status(403).JSON(CasinoError{Code: "PLAYER_LIMITED"})
    }

    // Debit wallet via gRPC (idempotent by transaction_id)
    result, err := h.walletClient.Debit(c.Context(), &wallet.DebitRequest{
        UserId:         userID,
        CurrencyCode:   req.Currency,
        Amount:         req.Amount,
        IdempotencyKey: req.TransactionID,
        ReferenceType:  "casino_bet",
        ReferenceId:    req.RoundID,
    })
    if err != nil {
        if isInsufficientBalance(err) {
            return c.Status(402).JSON(CasinoError{Code: "INSUFFICIENT_FUNDS"})
        }
        return c.Status(500).JSON(CasinoError{Code: "INTERNAL_ERROR"})
    }

    // Track game session
    h.trackSession(c.Context(), userID, req.GameID, req.SessionID, req.Amount)

    return c.JSON(CasinoBetResponse{
        Balance:       result.NewBalance,
        TransactionID: result.TransactionId,
    })
}
============================================================
SECTION 4: GAME CATALOG
============================================================
text

SYNC: Daily full sync + webhook for new games

GAME MODEL:
  id, external_id, provider_name
  name, type (slot/table/live/crash)
  category, tags, themes
  rtp (theoretical return to player), volatility
  min_bet, max_bet
  thumbnail_url, banner_url
  enabled, featured, new, popular
  countries_blocked[]
  desktop_supported, mobile_supported

GAME TYPES:
  Slots:      Pragmatic Play, NetEnt, Play'n GO, Microgaming
  Live Casino: Evolution, Pragmatic Live, Ezugi
  Table Games: Blackjack, Roulette, Baccarat (provider + in-house)
  Crash Games: Spribe (Aviator), Smartsoft, Turbo Games
  
SEARCH & FILTER:
  By provider, category, theme, popularity
  Full-text search on name
  Cache game list in DragonflyDB (TTL: 10 min)
  Total: 3000-5000 games
============================================================
SECTION 5: GAME SESSIONS
============================================================
text

TRACKING per session:
  session_id, user_id, game_id, provider
  started_at, ended_at
  total_bet, total_win, rounds_played
  device, ip_address

RESPONSIBLE GAMBLING integration:
  Reality check popup: every 30/60 min (configurable per user)
  Session time limit: force close game when reached
  Loss limit: if net loss exceeds limit → block further bets

RTP MONITORING:
  Track actual RTP per game per period
  If actual RTP deviates > 5% from theoretical over 10K+ rounds:
    → Alert for investigation (possible game malfunction)
  Monthly RTP reports for regulators
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER build casino games yourself (use certified providers via aggregator)
❌ NEVER skip aggregator auth verification on wallet endpoints
❌ NEVER allow game launch without active session + KYC check
❌ NEVER cache balance for casino wallet API (always real-time from wallet service)
❌ NEVER ignore rollback requests (they fix accounting errors)
❌ NEVER expose internal user_id to aggregator (use mapped player_id)
❌ NEVER skip reality check popup (regulatory requirement)
❌ NEVER serve blocked games to restricted countries
============================================================
SECTION 7: TESTING
============================================================
text

MUST TEST:
  ✅ Bet → Win → balance correct
  ✅ Bet → Rollback → balance restored
  ✅ Duplicate transaction_id returns same response (idempotency)
  ✅ Win without prior bet → handled (queue or reject)
  ✅ Response time < 50ms under load
  ✅ Game catalog search returns results < 200ms
  ✅ Session tracking records all rounds
  ✅ Reality check triggers at configured interval
  ✅ Country-blocked games not visible to blocked users