use rust_decimal::Decimal;
use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;

use crate::domain::bet::*;
use crate::domain::selection::{Selection, SelectionRow};

/// Column list shared by every `bets` read.
///
/// The `sqlx::query_as!` macros were compile-time checked against a live
/// database, which makes `cargo build` fail on CI (no DB service, no committed
/// `.sqlx` cache). The row structs already derive `sqlx::FromRow`, so these
/// queries are resolved at runtime instead.
const BET_COLUMNS: &str = "id, user_id, bet_type, status, stake, potential_win, actual_win, \
                          odds, currency_code, sport_id, event_id, idempotency_key, \
                          ip_address, device_fingerprint, placed_at, settled_at";

#[derive(Clone)]
pub struct BetRepository {
    pool: PgPool,
}

impl BetRepository {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    pub fn pool_ref(&self) -> &PgPool {
        &self.pool
    }

    /// Decode a `bets` row into `BetRow`.
    fn map_bet_row(row: &sqlx::postgres::PgRow) -> Result<BetRow, sqlx::Error> {
        use sqlx::Row as _;
        Ok(BetRow {
            id: row.try_get("id")?,
            user_id: row.try_get("user_id")?,
            bet_type: row.try_get("bet_type")?,
            status: row.try_get("status")?,
            stake: row.try_get("stake")?,
            potential_win: row.try_get("potential_win")?,
            actual_win: row.try_get("actual_win")?,
            odds: row.try_get("odds")?,
            currency_code: row.try_get("currency_code")?,
            sport_id: row.try_get("sport_id")?,
            event_id: row.try_get("event_id")?,
            idempotency_key: row.try_get("idempotency_key")?,
            ip_address: row.try_get("ip_address")?,
            device_fingerprint: row.try_get("device_fingerprint")?,
            placed_at: row.try_get("placed_at")?,
            settled_at: row.try_get("settled_at")?,
        })
    }

    pub async fn create_bet(&self, params: CreateBetParams) -> Result<Bet, sqlx::Error> {
        let mut tx = self.pool.begin().await?;

        let sql = format!(
            r#"
            INSERT INTO bets (
                user_id, bet_type, status, stake, odds,
                potential_win, actual_win, currency_code,
                sport_id, event_id, idempotency_key,
                ip_address, device_fingerprint
            )
            VALUES ($1, $2, 'pending', $3, $4, $5, 0, $6, $7, $8, $9, $10, $11)
            ON CONFLICT (idempotency_key) DO NOTHING
            RETURNING {BET_COLUMNS}
            "#
        );

        let maybe_row = sqlx::query(&sql)
            .bind(params.user_id.0)
            .bind(bet_type_db_value(params.bet_type))
            .bind(params.stake)
            .bind(params.combined_odds)
            .bind(params.potential_win)
            .bind(&params.currency_code)
            .bind(params.sport_id)
            .bind(params.event_id.map(|e| e.0))
            .bind(params.idempotency_key)
            .bind(&params.ip_address)
            .bind(&params.device_fingerprint)
            .fetch_optional(&mut *tx)
            .await?
            .as_ref()
            .map(Self::map_bet_row)
            .transpose()?;

        let row = match maybe_row {
            Some(r) => r,
            None => {
                // Conflict occurred — fetch the existing bet by idempotency_key
                let sql = format!(
                    "SELECT {BET_COLUMNS} FROM bets WHERE idempotency_key = $1 AND user_id = $2"
                );
                let row = sqlx::query(&sql)
                    .bind(params.idempotency_key)
                    .bind(params.user_id.0)
                    .fetch_one(&mut *tx)
                    .await?;
                Self::map_bet_row(&row)?
            }
        };

        for sel in &params.selections {
            sqlx::query(
                r#"
                INSERT INTO bet_selections (
                    bet_id, user_id, event_id, market_id,
                    selection_id, odds
                )
                VALUES ($1, $2, $3, $4, $5, $6)
                "#,
            )
            .bind(row.id)
            .bind(params.user_id.0)
            .bind(sel.event_id.0)
            .bind(sel.market_id.0)
            .bind(sel.outcome_id.0)
            .bind(sel.odds)
            .execute(&mut *tx)
            .await?;
        }

        tx.commit().await?;

        let mut bet = Bet::from(row);
        bet.selections = params
            .selections
            .into_iter()
            .map(|s| Selection {
                id: None,
                bet_id: bet.id,
                event_id: s.event_id,
                market_id: s.market_id,
                outcome_id: s.outcome_id,
                odds: s.odds,
                event_name: None,
                market_name: None,
                result: None,
                created_at: None,
                settled_at: None,
            })
            .collect();

        Ok(bet)
    }

    pub async fn get_bet_by_id(
        &self,
        bet_id: BetId,
        user_id: UserId,
    ) -> Result<Option<Bet>, sqlx::Error> {
        let sql = format!("SELECT {BET_COLUMNS} FROM bets WHERE id = $1 AND user_id = $2");
        let row = sqlx::query(&sql)
            .bind(bet_id.0)
            .bind(user_id.0)
            .fetch_optional(&self.pool)
            .await?;

        match row {
            Some(row) => {
                let mut bet = Bet::from(Self::map_bet_row(&row)?);
                let selections = self.get_bet_selections(bet.id).await?;
                bet.selections = selections;
                Ok(Some(bet))
            }
            None => Ok(None),
        }
    }

    pub async fn get_user_bets(
        &self,
        user_id: UserId,
        limit: i64,
        cursor: Option<i64>,
        status: Option<BetStatus>,
    ) -> Result<(Vec<Bet>, i64), sqlx::Error> {
        // `COUNT(*)` yields `int8`; `Option` because the aggregate is never NULL
        // but sqlx needs the fallback type spelled out for the runtime decoder.
        let total: i64 =
            sqlx::query_scalar::<_, Option<i64>>("SELECT COUNT(*) FROM bets WHERE user_id = $1")
                .bind(user_id.0)
                .fetch_one(&self.pool)
                .await?
                .unwrap_or(0);

        let status_filter = status.map(bet_status_db_value);

        // `$2::text IS NULL` lets a single statement cover both the filtered and
        // unfiltered case without changing the parameter types.
        let sql = format!(
            r#"
            SELECT {BET_COLUMNS}
            FROM bets
            WHERE user_id = $1
              AND ($2::bet_type_enum IS NULL OR status = $2)
              AND ($3::bigint IS NULL OR id < $3)
            ORDER BY id DESC
            LIMIT $4
            "#
        );

        let rows = sqlx::query(&sql)
            .bind(user_id.0)
            .bind(status_filter)
            .bind(cursor)
            .bind(limit)
            .fetch_all(&self.pool)
            .await?;

        let mut bets = Vec::with_capacity(rows.len());
        for row in &rows {
            let mut bet = Bet::from(Self::map_bet_row(row)?);
            let selections = self.get_bet_selections(bet.id).await?;
            bet.selections = selections;
            bets.push(bet);
        }

        Ok((bets, total))
    }

    pub async fn update_bet_status(
        &self,
        tx: &mut Transaction<'_, Postgres>,
        bet_id: BetId,
        from_status: BetStatus,
        to_status: BetStatus,
        actual_win: Option<Decimal>,
    ) -> Result<Bet, sqlx::Error> {
        let sql = format!(
            r#"
            UPDATE bets
            SET
                status = $3,
                actual_win = COALESCE($4, actual_win),
                settled_at = CASE
                    WHEN $3 IN ('won', 'lost', 'void', 'cashout')
                    THEN NOW()
                    ELSE settled_at
                END,
                updated_at = NOW()
            WHERE id = $1 AND status = $2
            RETURNING {BET_COLUMNS}
            "#
        );

        let row = sqlx::query(&sql)
            .bind(bet_id.0)
            .bind(bet_status_db_value(from_status))
            .bind(bet_status_db_value(to_status))
            .bind(actual_win)
            .fetch_optional(&mut **tx)
            .await?;

        row.as_ref()
            .map(Self::map_bet_row)
            .transpose()?
            .map(Bet::from)
            .ok_or(sqlx::Error::RowNotFound)
    }

    async fn get_bet_selections(&self, bet_id: BetId) -> Result<Vec<Selection>, sqlx::Error> {
        let rows = sqlx::query_as::<_, SelectionRow>(
            r#"
            SELECT
                id, bet_id, event_id, market_id,
                outcome_id, odds,
                status, result, created_at, settled_at
            FROM bet_selections
            WHERE bet_id = $1
            ORDER BY id
            "#,
        )
        .bind(bet_id.0)
        .fetch_all(&self.pool)
        .await?;

        Ok(rows.into_iter().map(Selection::from).collect())
    }
}

/// The `bets.status` column is a Postgres enum (`bet_status_enum`), so binds
/// must carry the enum's wire value rather than the Rust `Debug` output.
fn bet_status_db_value(status: BetStatus) -> &'static str {
    match status {
        BetStatus::Pending => "pending",
        BetStatus::Active => "active",
        BetStatus::Won => "won",
        BetStatus::Lost => "lost",
        BetStatus::Void => "void",
        BetStatus::Cashout => "cashout",
        BetStatus::Rejected => "rejected",
    }
}

/// `bets.bet_type` is a `bet_type_enum` column.
fn bet_type_db_value(bet_type: BetType) -> &'static str {
    match bet_type {
        BetType::Single => "single",
        BetType::Accumulator => "accumulator",
        BetType::System => "system",
        BetType::Chain => "chain",
    }
}

pub struct CreateBetParams {
    pub user_id: UserId,
    pub bet_type: BetType,
    pub stake: Decimal,
    pub combined_odds: Decimal,
    pub potential_win: Decimal,
    pub currency_code: String,
    pub sport_id: Option<i32>,
    pub event_id: Option<EventId>,
    pub selections: Vec<CreateSelectionParams>,
    pub idempotency_key: Uuid,
    pub ip_address: Option<String>,
    pub device_fingerprint: Option<String>,
}

pub struct CreateSelectionParams {
    pub event_id: EventId,
    pub market_id: MarketId,
    pub outcome_id: OutcomeId,
    pub odds: Decimal,
}
