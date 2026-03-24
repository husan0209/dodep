use sqlx::{PgPool, Postgres, Transaction};
use uuid::Uuid;
use rust_decimal::Decimal;

use crate::domain::bet::*;
use crate::domain::selection::{Selection, SelectionRow};

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

    pub async fn create_bet(
        &self,
        params: CreateBetParams,
    ) -> Result<Bet, sqlx::Error> {
        let mut tx = self.pool.begin().await?;

        let row = sqlx::query_as!(
            BetRow,
            r#"
            INSERT INTO bets (
                user_id, bet_type, status, stake, odds,
                potential_win, actual_win, currency_code,
                sport_id, event_id, idempotency_key,
                ip_address, device_fingerprint
            )
            VALUES ($1, $2, 'pending', $3, $4, $5, 0, $6, $7, $8, $9, $10, $11)
            ON CONFLICT (idempotency_key) DO NOTHING
            RETURNING
                id, user_id,
                bet_type as "bet_type: BetType",
                status as "status: BetStatus",
                stake, potential_win, actual_win,
                odds, currency_code,
                sport_id, event_id, idempotency_key,
                ip_address, device_fingerprint,
                placed_at, settled_at
            "#,
            params.user_id.0,
            params.bet_type as BetType,
            params.stake,
            params.combined_odds,
            params.potential_win,
            params.currency_code,
            params.sport_id,
            params.event_id.map(|e| e.0),
            params.idempotency_key,
            params.ip_address,
            params.device_fingerprint,
        )
        .fetch_one(&mut *tx)
        .await?;

        for sel in &params.selections {
            sqlx::query!(
                r#"
                INSERT INTO bet_selections (
                    bet_id, user_id, event_id, market_id,
                    selection_id, odds
                )
                VALUES ($1, $2, $3, $4, $5, $6)
                "#,
                row.id,
                params.user_id.0,
                sel.event_id.0,
                sel.market_id.0,
                sel.outcome_id.0,
                sel.odds,
            )
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
        let row = sqlx::query_as!(
            BetRow,
            r#"
            SELECT
                id, user_id,
                bet_type as "bet_type: BetType",
                status as "status: BetStatus",
                stake, potential_win, actual_win,
                odds, currency_code,
                sport_id, event_id, idempotency_key,
                ip_address, device_fingerprint,
                placed_at, settled_at
            FROM bets
            WHERE id = $1 AND user_id = $2
            "#,
            bet_id.0,
            user_id.0,
        )
        .fetch_optional(&self.pool)
        .await?;

        match row {
            Some(r) => {
                let mut bet = Bet::from(r);
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
        let total: i64 = sqlx::query_scalar(
            "SELECT COUNT(*) FROM bets WHERE user_id = $1",
        )
        .bind(user_id.0)
        .fetch_one(&self.pool)
        .await?
        .unwrap_or(0);

        let rows = if let Some(status_filter) = status {
            sqlx::query_as!(
                BetRow,
                r#"
                SELECT
                    id, user_id,
                    bet_type as "bet_type: BetType",
                    status as "status: BetStatus",
                    stake, potential_win, actual_win,
                    odds, currency_code,
                    sport_id, event_id, idempotency_key,
                    ip_address, device_fingerprint,
                    placed_at, settled_at
                FROM bets
                WHERE user_id = $1
                  AND status = $2
                  AND ($3::bigint IS NULL OR id < $3)
                ORDER BY id DESC
                LIMIT $4
                "#,
                user_id.0,
                status_filter as BetStatus,
                cursor,
                limit,
            )
            .fetch_all(&self.pool)
            .await?
        } else {
            sqlx::query_as!(
                BetRow,
                r#"
                SELECT
                    id, user_id,
                    bet_type as "bet_type: BetType",
                    status as "status: BetStatus",
                    stake, potential_win, actual_win,
                    odds, currency_code,
                    sport_id, event_id, idempotency_key,
                    ip_address, device_fingerprint,
                    placed_at, settled_at
                FROM bets
                WHERE user_id = $1
                  AND ($2::bigint IS NULL OR id < $2)
                ORDER BY id DESC
                LIMIT $3
                "#,
                user_id.0,
                cursor,
                limit,
            )
            .fetch_all(&self.pool)
            .await?
        };

        let mut bets = Vec::with_capacity(rows.len());
        for row in rows {
            let mut bet = Bet::from(row);
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
        let row = sqlx::query_as!(
            BetRow,
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
            RETURNING
                id, user_id,
                bet_type as "bet_type: BetType",
                status as "status: BetStatus",
                stake, potential_win, actual_win,
                odds, currency_code,
                sport_id, event_id, idempotency_key,
                ip_address, device_fingerprint,
                placed_at, settled_at
            "#,
            bet_id.0,
            from_status as BetStatus,
            to_status as BetStatus,
            actual_win,
        )
        .fetch_optional(&mut **tx)
        .await?;

        row.map(Bet::from).ok_or(sqlx::Error::RowNotFound)
    }

    async fn get_bet_selections(&self, bet_id: BetId) -> Result<Vec<Selection>, sqlx::Error> {
        let rows = sqlx::query_as!(
            SelectionRow,
            r#"
            SELECT
                id, bet_id, event_id, market_id,
                outcome_id, odds,
                status, result, created_at, settled_at
            FROM bet_selections
            WHERE bet_id = $1
            ORDER BY id
            "#,
            bet_id.0,
        )
        .fetch_all(&self.pool)
        .await?;

        Ok(rows.into_iter().map(Selection::from).collect())
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
