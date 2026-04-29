package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type ClickHouseClient struct {
	db *sql.DB
}

type FinancialReportRow struct {
	Period      string
	Deposits    float64
	Withdrawals float64
}

type CasinoAnalyticsRow struct {
	GameID        uint32
	GameName      string
	Provider      string
	GGR           float64
	Rounds        int64
	UniquePlayers int64
	ActualRTP     float64
}

func NewClickHouseClient() (*ClickHouseClient, error) {
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		return nil, ErrNotConfigured
	}

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &ClickHouseClient{db: db}, nil
}

func (c *ClickHouseClient) FinancialReportRows(ctx context.Context, from, to time.Time) ([]FinancialReportRow, error) {
	if c == nil || c.db == nil {
		return nil, ErrNotConfigured
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT
			toString(report_date) AS period,
			toFloat64(sum(total_deposits)) AS deposits,
			toFloat64(sum(total_withdrawals)) AS withdrawals
		FROM financial_reports
		WHERE report_date >= ? AND report_date < ?
		GROUP BY report_date
		ORDER BY report_date ASC
	`, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("query financial reports: %w", err)
	}
	defer rows.Close()

	var result []FinancialReportRow
	for rows.Next() {
		var row FinancialReportRow
		if err := rows.Scan(&row.Period, &row.Deposits, &row.Withdrawals); err != nil {
			return nil, fmt.Errorf("scan financial report row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate financial reports: %w", err)
	}
	return result, nil
}

func (c *ClickHouseClient) CasinoAnalyticsRows(ctx context.Context, from, to time.Time) ([]CasinoAnalyticsRow, error) {
	if c == nil || c.db == nil {
		return nil, ErrNotConfigured
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT
			game_id,
			any(game_name) AS game_name,
			any(provider) AS provider,
			toFloat64(sum(bet_amount) - sum(win_amount)) AS ggr,
			sum(rounds_played) AS rounds,
			uniqExact(user_id) AS unique_players,
			if(sum(bet_amount) > 0, (sum(win_amount) / sum(bet_amount)) * 100, 0) AS actual_rtp
		FROM casino_rounds
		WHERE event_time >= ? AND event_time < ?
		GROUP BY game_id
		ORDER BY ggr DESC
		LIMIT 200
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query casino analytics: %w", err)
	}
	defer rows.Close()

	var result []CasinoAnalyticsRow
	for rows.Next() {
		var row CasinoAnalyticsRow
		var uniquePlayers uint64
		if err := rows.Scan(&row.GameID, &row.GameName, &row.Provider, &row.GGR, &row.Rounds, &uniquePlayers, &row.ActualRTP); err != nil {
			return nil, fmt.Errorf("scan casino analytics row: %w", err)
		}
		row.UniquePlayers = int64(uniquePlayers)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate casino analytics: %w", err)
	}
	return result, nil
}

func (c *ClickHouseClient) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func IsClickHouseNotConfigured(err error) bool {
	return errors.Is(err, ErrNotConfigured)
}
