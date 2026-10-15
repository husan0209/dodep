

Начинаю. 4 файла в формате B (400-600 строк каждый).

---

# SKILL #53 — wallet-financial-ops.skill.md

```markdown
# wallet-financial-ops.skill.md
# GAMBLING PLATFORM — WALLET & FINANCIAL OPERATIONS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Rust Core Agent, Go Payment Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

You handle REAL MONEY. Every line of code is auditable.
Every bug is a financial loss or regulatory violation.

Wallet Service is written in Rust (critical path).
Payment Service is written in Go (PSP integrations).

# ============================================================
# SECTION 2: DOUBLE-ENTRY BOOKKEEPING
# ============================================================

## PRINCIPLE

Every financial movement creates TWO entries that sum to zero.
Money never appears or disappears — it moves between accounts.

## ACCOUNT TYPES

```text
ASSETS (platform holds):
  user_wallet:{user_id}:{currency}   — player balance
  house_revenue                       — platform earnings
  payment_gateway_transit             — in-flight deposits/withdrawals
  tax_reserve                         — withheld taxes

RULES:
  SUM(all debits) = SUM(all credits) — ALWAYS
  If this equation breaks → P1 alert, halt operations
```

## TRANSACTION RECORD

```rust
pub struct LedgerEntry {
    pub id: i64,
    pub transaction_id: i64,        // groups debit+credit pair
    pub account_type: AccountType,
    pub account_id: String,          // e.g., "user_wallet:12345:USD"
    pub entry_type: EntryType,       // Debit or Credit
    pub amount: Decimal,             // always positive
    pub balance_after: Decimal,
    pub reference_type: String,      // "bet", "deposit", "bonus"
    pub reference_id: i64,
    pub idempotency_key: Uuid,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Copy)]
pub enum EntryType {
    Debit,   // money leaves account
    Credit,  // money enters account
}
```

## EXAMPLE FLOWS

```text
DEPOSIT $100:
  Debit:  payment_gateway_transit  $100
  Credit: user_wallet:123:USD      $100

BET PLACEMENT $50:
  Debit:  user_wallet:123:USD      $50  (locked balance)
  Credit: house_hold:bets          $50

BET WON $125 (stake $50 × odds 2.50):
  Debit:  house_revenue            $125
  Credit: user_wallet:123:USD      $125

BET LOST:
  Debit:  house_hold:bets          $50
  Credit: house_revenue            $50

WITHDRAWAL $200:
  Debit:  user_wallet:123:USD      $200
  Credit: payment_gateway_transit  $200
```

# ============================================================
# SECTION 3: WALLET OPERATIONS
# ============================================================

## BALANCE MODEL

```rust
pub struct Wallet {
    pub user_id: i64,
    pub currency_code: String,
    pub balance: Decimal,        // available for use
    pub locked_balance: Decimal, // reserved (active bets)
    pub bonus_balance: Decimal,  // bonus funds (separate tracking)
    pub version: i64,            // optimistic locking
}

// INVARIANTS (enforced at DB level):
// balance >= 0
// locked_balance >= 0
// balance >= locked_balance
// bonus_balance >= 0
```

## CORE OPERATIONS

```rust
/// All wallet operations follow this pattern:
/// 1. Check idempotency cache
/// 2. Load wallet with version
/// 3. Validate business rules
/// 4. Update wallet with optimistic lock
/// 5. Insert ledger entries
/// 6. Commit transaction
/// 7. Cache idempotency result
/// 8. Publish event

pub async fn debit(
    &self,
    user_id: i64,
    amount: Decimal,
    idempotency_key: Uuid,
    reference: Reference,
) -> Result<Transaction, WalletError> {
    // 1. Idempotency
    if let Some(cached) = self.cache.get_idempotent(&idempotency_key).await? {
        return Ok(cached);
    }

    // 2-6. In database transaction
    let mut tx = self.pool.begin().await?;

    let wallet = sqlx::query_as!(Wallet,
        "SELECT * FROM wallets WHERE user_id = $1 AND currency_code = $2 FOR UPDATE",
        user_id, reference.currency
    ).fetch_one(&mut *tx).await?;

    // 3. Validate
    if wallet.balance < amount {
        return Err(WalletError::InsufficientBalance {
            required: amount,
            available: wallet.balance,
        });
    }

    // 4. Update with optimistic lock
    let rows = sqlx::query!(
        "UPDATE wallets SET balance = balance - $1, version = version + 1
         WHERE user_id = $2 AND currency_code = $3 AND version = $4",
        amount, user_id, reference.currency, wallet.version
    ).execute(&mut *tx).await?.rows_affected();

    if rows == 0 {
        return Err(WalletError::ConcurrencyConflict);
    }

    // 5. Insert ledger
    let txn = insert_ledger_pair(&mut tx, /* ... */).await?;

    // 6. Commit
    tx.commit().await?;

    // 7-8. Cache + event (non-critical)
    let _ = self.cache.set_idempotent(&idempotency_key, &txn).await;
    let _ = self.producer.publish("wallet.debited", &txn).await;

    Ok(txn)
}
```

## LOCK / UNLOCK (for bets)

```text
LOCK: Move funds from balance → locked_balance
  Used when bet is placed. Funds reserved but not spent yet.
  
  UPDATE wallets SET
    balance = balance - $amount,
    locked_balance = locked_balance + $amount,
    version = version + 1
  WHERE user_id = $1 AND version = $2 AND balance >= $amount

UNLOCK: Move funds from locked_balance → balance
  Used when bet is cancelled/rejected.
  
  UPDATE wallets SET
    balance = balance + $amount,
    locked_balance = locked_balance - $amount,
    version = version + 1
  WHERE user_id = $1 AND version = $2 AND locked_balance >= $amount

SETTLE: Resolve locked funds
  Won:  locked_balance -= stake, balance += payout
  Lost: locked_balance -= stake (house keeps it)
  Void: locked_balance -= stake, balance += stake (return)
```

# ============================================================
# SECTION 4: CONCURRENCY CONTROL
# ============================================================

```text
PROBLEM: 10 concurrent requests to debit same wallet.
SOLUTION: Optimistic locking + DB constraints.

FLOW:
  1. SELECT balance, version FROM wallets WHERE user_id = $1
  2. Check: balance >= amount
  3. UPDATE wallets SET balance = balance - $amount, version = version + 1
     WHERE user_id = $1 AND version = $expected_version
  4. If rows_affected = 0 → retry (max 3 times)
  5. If still fails → return ConcurrencyConflict error

WHY NOT pessimistic locking (SELECT FOR UPDATE)?
  - Fine for low-concurrency operations (settlements)
  - Too slow for high-concurrency (live bet placement)
  - Optimistic locking avoids holding DB row locks

SAFETY NET (database constraints):
  CHECK (balance >= 0)
  CHECK (locked_balance >= 0)
  CHECK (balance >= locked_balance)
  These NEVER allow negative balance, even if application has bug.
```

# ============================================================
# SECTION 5: RECONCILIATION
# ============================================================

```text
WHAT: Compare materialized balance with calculated balance.
WHY: Catch any discrepancy from bugs, race conditions, or fraud.
WHEN: Every hour (automated), daily (full report).

CALCULATION:
  expected_balance = SUM(credits) - SUM(debits) for this wallet
  actual_balance = wallets.balance + wallets.locked_balance

  IF abs(expected - actual) > $0.01:
    → P1 ALERT
    → Freeze wallet
    → Manual investigation

IMPLEMENTATION:
  SELECT
    w.user_id,
    w.balance + w.locked_balance as actual,
    COALESCE(SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END), 0) as expected
  FROM wallets w
  LEFT JOIN ledger_entries le ON le.account_id = 'user_wallet:' || w.user_id || ':' || w.currency_code
  GROUP BY w.user_id, w.balance, w.locked_balance
  HAVING ABS((w.balance + w.locked_balance) - (
    COALESCE(SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE 0 END), 0)
  )) > 0.01
```

# ============================================================
# SECTION 6: PAYMENT INTEGRATION
# ============================================================

```text
DEPOSIT FLOW:
  1. User selects amount + method → POST /api/v1/payments/deposit
  2. Check KYC level limits, responsible gambling limits
  3. Create pending payment record in DB
  4. Call PSP API → get redirect URL / payment widget
  5. User completes payment on PSP side
  6. PSP sends webhook → POST /api/v1/payments/webhook/{provider}
  7. Verify webhook signature
  8. Credit wallet (idempotent by PSP transaction ID)
  9. Publish payments.completed event
  10. Notify user

WITHDRAWAL FLOW:
  1. User requests withdrawal → POST /api/v1/payments/withdraw
  2. Check KYC level >= 2
  3. Check wagering requirements (bonus)
  4. Check withdrawal limits
  5. Lock funds in wallet
  6. Run fraud/risk check
  7. Auto-approve OR queue for manual review
  8. On approval: call PSP API to send funds
  9. On PSP success: debit wallet (finalize)
  10. On PSP failure: unlock funds, notify user

IDEMPOTENCY:
  - Deposit: PSP transaction_id as idempotency key
  - Withdrawal: our generated UUID as idempotency key
  - Webhook: PSP event_id checked (process only once)
```

# ============================================================
# SECTION 7: ANTI-PATTERNS
# ============================================================

```text
❌ NEVER use float64/f64 for money → USE Decimal (rust_decimal / shopspring)
❌ NEVER update balance without version check → USE optimistic locking
❌ NEVER debit without checking balance first → DB CHECK constraint is safety net
❌ NEVER process webhook without signature verification
❌ NEVER return "balance" in error messages to unauthenticated users
❌ NEVER log full card numbers or bank accounts
❌ NEVER allow withdrawal without KYC Level 2+
❌ NEVER skip idempotency on any write operation
❌ NEVER do wallet operations outside a DB transaction
❌ NEVER trust client-submitted amounts (recalculate server-side)
```

# ============================================================
# SECTION 8: TESTING REQUIREMENTS
# ============================================================

```text
MUST TEST:
  ✅ Concurrent debits never overdraft (spawn 100 tasks, total > balance)
  ✅ Idempotency: same key returns same result, balance changes once
  ✅ Double-entry: SUM(all entries) = 0 after any operation
  ✅ Lock/Unlock/Settle cycle is consistent
  ✅ Reconciliation detects injected discrepancy
  ✅ Negative balance impossible under any code path
  ✅ Webhook replay does not double-credit
  ✅ Wallet creation is atomic with user creation
```
```

---

