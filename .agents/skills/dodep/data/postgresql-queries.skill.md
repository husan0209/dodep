#34 postgresql-queries.skill.md
Markdown

# postgresql-queries.skill.md

## РОЛЬ
Ты — Backend Developer, пишущий SQL-запросы и работающий с PostgreSQL
через SQLx (Rust) и GORM/sqlc (Go) для гемблинг-платформы.

## КОНТЕКСТ
- PostgreSQL 16 + Citus (distributed tables)
- PgBouncer в transaction mode (нет prepared statements на уровне сессии)
- Миллиарды строк в транзакционных таблицах
- Финансовые операции: нулевая терпимость к ошибкам
- p99 query latency: < 10ms

## ПРАВИЛА ЗАПРОСОВ

### 1. Всегда параметризованные запросы
```rust
// ✅ Rust (SQLx) — compile-time проверка
let user = sqlx::query_as!(
    User,
    r#"
    SELECT id, email, status as "status: UserStatus", 
           kyc_level, currency_code, created_at
    FROM users
    WHERE id = $1 AND status != 'closed'
    "#,
    user_id
)
.fetch_optional(&pool)
.await?;

// ❌ НИКОГДА: string interpolation
let query = format!("SELECT * FROM users WHERE id = {}", user_id);
Go

// ✅ Go (sqlc) — типобезопасные запросы
// query.sql:
// -- name: GetUser :one
// SELECT id, email, status, kyc_level, currency_code, created_at
// FROM users WHERE id = $1 AND status != 'closed';

user, err := queries.GetUser(ctx, userID)

// ✅ Go (GORM) — параметры
var user User
db.Where("id = ? AND status != ?", userID, "closed").First(&user)

// ❌ НИКОГДА
db.Raw(fmt.Sprintf("SELECT * FROM users WHERE id = %d", userID))
2. Явные колонки вместо SELECT *
SQL

-- ❌ ПЛОХО
SELECT * FROM users WHERE id = $1;

-- ✅ ПРАВИЛЬНО: только нужные колонки
SELECT id, email, status, kyc_level, currency_code
FROM users WHERE id = $1;

-- Почему: меньше данных по сети, не сломается при ALTER TABLE,
-- Citus не передаёт лишнее между нодами
3. Cursor-based pagination
SQL

-- ❌ ПЛОХО: OFFSET на больших таблицах
SELECT * FROM bets 
ORDER BY placed_at DESC 
LIMIT 20 OFFSET 100000;
-- Сканирует 100020 строк, O(N)

-- ✅ ПРАВИЛЬНО: cursor-based
SELECT id, user_id, stake, odds, status, placed_at
FROM bets
WHERE user_id = $1
  AND placed_at < $2          -- cursor: timestamp последней записи
ORDER BY placed_at DESC
LIMIT 20;

-- Для первой страницы: $2 = NOW()
-- Для следующей: $2 = placed_at последней записи из предыдущей страницы
Rust

// Rust — cursor pagination
#[derive(Deserialize)]
pub struct CursorParams {
    pub cursor: Option<DateTime<Utc>>,  // None = первая страница
    pub limit: i64,                      // default 20, max 100
}

pub async fn get_bet_history(
    pool: &PgPool,
    user_id: i64,
    params: &CursorParams,
) -> Result<Vec<Bet>> {
    let limit = params.limit.min(100);
    let cursor = params.cursor.unwrap_or(Utc::now());

    let bets = sqlx::query_as!(
        Bet,
        r#"
        SELECT id, user_id, stake, odds, 
               status as "status: BetStatus", placed_at
        FROM bets
        WHERE user_id = $1 AND placed_at < $2
        ORDER BY placed_at DESC
        LIMIT $3
        "#,
        user_id,
        cursor,
        limit
    )
    .fetch_all(pool)
    .await?;

    Ok(bets)
}
ТРАНЗАКЦИИ
Wallet Debit — полный паттерн
Rust

// ✅ Полная транзакция с optimistic locking
pub async fn debit_wallet(
    pool: &PgPool,
    user_id: i64,
    currency: &str,
    amount: Decimal,
    idempotency_key: Uuid,
    reference_type: &str,
    reference_id: i64,
) -> Result<Transaction> {
    // Retry loop для optimistic locking
    for attempt in 0..3 {
        let mut tx = pool.begin().await?;

        // 1. Получить текущий баланс с версией
        let wallet = sqlx::query_as!(
            Wallet,
            r#"
            SELECT id, balance, locked_balance, version
            FROM wallets
            WHERE user_id = $1 AND currency_code = $2
            FOR UPDATE  -- pessimistic lock внутри транзакции
            "#,
            user_id,
            currency
        )
        .fetch_optional(&mut *tx)
        .await?
        .ok_or(Error::WalletNotFound)?;

        // 2. Проверить достаточность средств
        let available = wallet.balance - wallet.locked_balance;
        if available < amount {
            tx.rollback().await?;
            return Err(Error::InsufficientBalance {
                required: amount,
                available,
            });
        }

        // 3. Обновить баланс с optimistic locking
        let rows = sqlx::query!(
            r#"
            UPDATE wallets
            SET balance = balance - $1,
                version = version + 1,
                updated_at = NOW()
            WHERE id = $2 AND version = $3
            "#,
            amount,
            wallet.id,
            wallet.version
        )
        .execute(&mut *tx)
        .await?
        .rows_affected();

        if rows == 0 {
            tx.rollback().await?;
            if attempt < 2 {
                tokio::time::sleep(Duration::from_millis(10 * (attempt + 1) as u64)).await;
                continue;  // retry
            }
            return Err(Error::ConcurrentModification);
        }

        // 4. Записать транзакцию (idempotent)
        let transaction = sqlx::query_as!(
            Transaction,
            r#"
            INSERT INTO wallet_transactions (
                user_id, wallet_id, type, amount,
                balance_before, balance_after,
                idempotency_key, reference_type, reference_id
            ) VALUES ($1, $2, 'debit', $3, $4, $5, $6, $7, $8)
            ON CONFLICT (idempotency_key) DO NOTHING
            RETURNING id, balance_after, created_at
            "#,
            user_id,
            wallet.id,
            amount,
            wallet.balance,                  // balance_before
            wallet.balance - amount,         // balance_after
            idempotency_key,
            reference_type,
            reference_id
        )
        .fetch_optional(&mut *tx)
        .await?;

        // 5. Commit
        tx.commit().await?;

        return match transaction {
            Some(t) => Ok(t),
            None => {
                // Idempotency: запрос уже был обработан
                get_transaction_by_key(pool, idempotency_key).await
            }
        };
    }

    Err(Error::MaxRetriesExceeded)
}
Batch Operations
SQL

-- ✅ ПРАВИЛЬНО: массовый settle ставок одной транзакцией
WITH settled_bets AS (
    UPDATE bets
    SET status = CASE
            WHEN selection_result = 'won' THEN 'won'::bet_status_enum
            WHEN selection_result = 'lost' THEN 'lost'::bet_status_enum
            ELSE 'void'::bet_status_enum
        END,
        actual_win = CASE
            WHEN selection_result = 'won' THEN stake * odds
            WHEN selection_result = 'void' THEN stake
            ELSE 0
        END,
        settled_at = NOW()
    WHERE event_id = $1
      AND status = 'active'
    RETURNING id, user_id, stake, actual_win, status
)
SELECT * FROM settled_bets;

-- Далее: batch credit wallets для выигрышей
Bulk Insert
Rust

// ✅ ПРАВИЛЬНО: bulk insert через unnest
pub async fn insert_bet_selections(
    tx: &mut PgConnection,
    bet_id: i64,
    user_id: i64,
    selections: &[SelectionInput],
) -> Result<()> {
    let event_ids: Vec<i64> = selections.iter().map(|s| s.event_id).collect();
    let market_ids: Vec<i64> = selections.iter().map(|s| s.market_id).collect();
    let outcome_ids: Vec<i64> = selections.iter().map(|s| s.outcome_id).collect();
    let odds: Vec<Decimal> = selections.iter().map(|s| s.odds).collect();

    sqlx::query!(
        r#"
        INSERT INTO bet_selections (bet_id, user_id, event_id, market_id, outcome_id, odds)
        SELECT $1, $2, * FROM UNNEST($3::bigint[], $4::bigint[], $5::bigint[], $6::numeric[])
        "#,
        bet_id,
        user_id,
        &event_ids,
        &market_ids,
        &outcome_ids,
        &odds
    )
    .execute(&mut *tx)
    .await?;

    Ok(())
}
REPORTING QUERIES
Финансовый отчёт
SQL

-- Дневной отчёт GGR (Gross Gaming Revenue)
SELECT
    date_trunc('day', placed_at) AS day,
    COUNT(*) AS total_bets,
    SUM(stake) AS total_stakes,
    SUM(actual_win) AS total_payouts,
    SUM(stake) - SUM(actual_win) AS ggr,
    ROUND(
        (SUM(stake) - SUM(actual_win)) / NULLIF(SUM(stake), 0) * 100, 
        2
    ) AS margin_pct,
    COUNT(DISTINCT user_id) AS unique_players
FROM bets
WHERE placed_at >= $1
  AND placed_at < $2
  AND status IN ('won', 'lost')
GROUP BY date_trunc('day', placed_at)
ORDER BY day DESC;
Reconciliation
SQL

-- ✅ Сверка: materialized balance vs calculated balance
SELECT
    w.user_id,
    w.currency_code,
    w.balance AS materialized_balance,
    COALESCE(SUM(
        CASE WHEN t.type IN ('deposit', 'bet_win', 'bet_refund', 'bonus_credit', 'adjustment_credit')
             THEN t.amount
             ELSE -t.amount
        END
    ), 0) AS calculated_balance,
    w.balance - COALESCE(SUM(
        CASE WHEN t.type IN ('deposit', 'bet_win', 'bet_refund', 'bonus_credit', 'adjustment_credit')
             THEN t.amount
             ELSE -t.amount
        END
    ), 0) AS discrepancy
FROM wallets w
LEFT JOIN wallet_transactions t ON t.wallet_id = w.id AND t.status = 'completed'
GROUP BY w.user_id, w.currency_code, w.balance
HAVING ABS(w.balance - COALESCE(SUM(
    CASE WHEN t.type IN ('deposit', 'bet_win', 'bet_refund', 'bonus_credit', 'adjustment_credit')
         THEN t.amount
         ELSE -t.amount
    END
), 0)) > 0.01;
-- Если есть строки → ALARM
ИНДЕКСЫ
SQL

-- ✅ Частичные индексы — маленькие и быстрые
CREATE INDEX idx_bets_unsettled 
    ON bets (event_id, placed_at)
    WHERE status IN ('pending', 'active');
-- Только ~5% строк, но 90% запросов

-- ✅ Covering index — без обращения к таблице
CREATE INDEX idx_bets_user_history 
    ON bets (user_id, placed_at DESC)
    INCLUDE (stake, odds, status, actual_win);
-- SELECT stake, odds, status FROM bets WHERE user_id=X → index-only scan

-- ✅ BRIN для time-series
CREATE INDEX idx_transactions_created_brin
    ON wallet_transactions USING BRIN (created_at)
    WITH (pages_per_range = 32);
-- Крошечный индекс для огромных таблиц

-- ✅ GIN для JSONB поиска
CREATE INDEX idx_users_metadata_gin
    ON users USING GIN (metadata jsonb_path_ops);
-- SELECT * FROM users WHERE metadata @> '{"vip": true}';

-- ❌ ПЛОХО: индекс на каждую колонку
-- ❌ ПЛОХО: индекс с низкой селективностью
CREATE INDEX idx_users_status ON users (status);
-- status имеет 5 значений → индекс бесполезен для 'active' (80% строк)

-- ✅ ПРАВИЛЬНО: частичный индекс на редкие статусы
CREATE INDEX idx_users_blocked ON users (id) WHERE status = 'blocked';
EXPLAIN ANALYZE
SQL

-- Всегда проверяй план запроса перед production
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT id, stake, odds, status
FROM bets
WHERE user_id = 12345
  AND placed_at > NOW() - INTERVAL '30 days'
ORDER BY placed_at DESC
LIMIT 20;

-- Что искать в плане:
-- ✅ Index Scan / Index Only Scan — хорошо
-- ❌ Seq Scan на большой таблице — плохо
-- ❌ Sort (external merge) — нужен индекс
-- ❌ Nested Loop с большим количеством итераций — N+1
-- ✅ Bitmap Index Scan — нормально для IN / OR условий
АНТИПАТТЕРНЫ
SQL

-- ❌ ПЛОХО: COUNT(*) на огромных таблицах для пагинации
SELECT COUNT(*) FROM bets WHERE user_id = $1;
-- Сканирует миллионы строк

-- ✅ ПРАВИЛЬНО: приблизительный count или cursor pagination без count
SELECT reltuples::bigint AS estimate
FROM pg_class WHERE relname = 'bets';
-- Или: не показывай total, используй "Load more"

-- ❌ ПЛОХО: OR на разных колонках
SELECT * FROM users WHERE email = $1 OR phone = $1;
-- Не использует индексы эффективно

-- ✅ ПРАВИЛЬНО: UNION ALL
SELECT * FROM users WHERE email = $1
UNION ALL
SELECT * FROM users WHERE phone = $1 AND email != $1;

-- ❌ ПЛОХО: NOT IN с подзапросом
SELECT * FROM users WHERE id NOT IN (SELECT user_id FROM blocked_users);

-- ✅ ПРАВИЛЬНО: NOT EXISTS или LEFT JOIN
SELECT u.* FROM users u
LEFT JOIN blocked_users b ON b.user_id = u.id
WHERE b.user_id IS NULL;

-- ❌ ПЛОХО: функция на индексированной колонке
SELECT * FROM users WHERE LOWER(email) = 'test@test.com';

-- ✅ ПРАВИЛЬНО: functional index или хранить lowercase
CREATE INDEX idx_users_email_lower ON users (LOWER(email));
-- Или: хранить email уже в lowercase
GO (GORM) ПАТТЕРНЫ
Go

// ✅ ПРАВИЛЬНО: scopes для переиспользования
func ActiveUsers(db *gorm.DB) *gorm.DB {
    return db.Where("status = ?", "active")
}

func KYCVerified(level int) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Where("kyc_level >= ?", level)
    }
}

// Использование:
db.Scopes(ActiveUsers, KYCVerified(2)).Find(&users)

// ✅ ПРАВИЛЬНО: select конкретные поля
db.Select("id, email, status, balance").Find(&users)

// ✅ ПРАВИЛЬНО: preload с условием
db.Preload("Wallet", "currency_code = ?", "USD").Find(&user)

// ❌ ПЛОХО: N+1
for _, user := range users {
    db.Where("user_id = ?", user.ID).Find(&wallet)  // N запросов!
}

// ✅ ПРАВИЛЬНО: batch preload
db.Preload("Wallets").Find(&users)