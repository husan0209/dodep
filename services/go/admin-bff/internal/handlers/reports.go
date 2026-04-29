package handlers

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/client"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type reportsService struct {
	db  *gorm.DB
	log *zap.Logger
	ch  analyticsQueryClient
}

type reportDayTotals struct {
	deposits    float64
	withdrawals float64
	chargebacks float64
}

type playerAnalyticsStats struct {
	totalAmount float64
	count       int64
	lastActive  time.Time
}

type gameAnalyticsStats struct {
	gameID         string
	gameName       string
	provider       string
	betAmount      float64
	winAmount      float64
	rounds         int64
	uniquePlayers  map[int64]struct{}
	actualRTP      float64
	theoreticalRTP float64
}

type analyticsQueryClient interface {
	FinancialReportRows(ctx context.Context, from, to time.Time) ([]client.FinancialReportRow, error)
	CasinoAnalyticsRows(ctx context.Context, from, to time.Time) ([]client.CasinoAnalyticsRow, error)
}

func newReportsService(db *gorm.DB, log *zap.Logger) *reportsService {
	ch, err := client.NewClickHouseClient()
	if err != nil && !client.IsClickHouseNotConfigured(err) {
		log.Warn("clickhouse analytics unavailable, falling back to postgres", zap.Error(err))
	}
	return &reportsService{db: db, log: log, ch: ch}
}

func RegisterReportsRoutes(router fiber.Router, db *gorm.DB, log *zap.Logger) {
	svc := newReportsService(db, log)
	router.Get("/payments/financial-report", svc.financialReport)
	router.Get("/users/analytics", svc.playerAnalytics)
	router.Get("/games/analytics", svc.gameAnalytics)
	router.Get("/payments/compliance-report", svc.complianceReport)
}

func (s *reportsService) financialReport(c *fiber.Ctx) error {
	from := c.Query("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.Query("to", time.Now().Format("2006-01-02"))

	fromT, _ := time.Parse("2006-01-02", from)
	toT, _ := time.Parse("2006-01-02", to)
	toT = toT.Add(24 * time.Hour)

	buckets, totalDeposits, totalWithdrawals, totalChargebacks := s.collectFinancialTotals(c.Context(), fromT, toT)
	var rows []models.FinancialReportRow
	var totalNet float64
	for d := fromT; d.Before(toT); d = d.Add(24 * time.Hour) {
		period := d.Format("2006-01-02")
		bucket := buckets[period]
		if bucket == nil {
			bucket = &reportDayTotals{}
		}
		dep := bucket.deposits
		wd := bucket.withdrawals
		cb := bucket.chargebacks
		net := dep - wd - cb
		totalNet += net
		rows = append(rows, models.FinancialReportRow{
			Period:      period,
			Deposits:    formatMoney(dep),
			Withdrawals: formatMoney(wd),
			NetRevenue:  formatMoney(net),
			Chargebacks: formatMoney(cb),
			Status:      "finalized",
		})
	}

	return c.JSON(fiber.Map{
		"data": models.FinancialReportResponse{
			TotalDeposits:    formatMoney(totalDeposits),
			TotalWithdrawals: formatMoney(totalWithdrawals),
			NetRevenue:       formatMoney(totalNet),
			TotalChargebacks: formatMoney(totalChargebacks),
			Rows:             rows,
		},
	})
}

func (s *reportsService) collectFinancialTotals(ctx context.Context, fromT, toT time.Time) (map[string]*reportDayTotals, float64, float64, float64) {
	if s.ch != nil {
		if rows, err := s.ch.FinancialReportRows(ctx, fromT, toT); err == nil {
			buckets := make(map[string]*reportDayTotals, len(rows))
			var totalDeposits, totalWithdrawals float64
			for _, row := range rows {
				bucket := ensureDayTotals(buckets, row.Period)
				bucket.deposits += row.Deposits
				bucket.withdrawals += row.Withdrawals
				totalDeposits += row.Deposits
				totalWithdrawals += row.Withdrawals
			}
			chargebackBuckets, totalChargebacks := s.collectChargebacks(ctx, fromT, toT)
			for period, bucket := range chargebackBuckets {
				target := ensureDayTotals(buckets, period)
				target.chargebacks += bucket.chargebacks
			}
			return buckets, totalDeposits, totalWithdrawals, totalChargebacks
		} else if !client.IsClickHouseNotConfigured(err) {
			s.log.Warn("clickhouse financial report query failed, using postgres fallback", zap.Error(err))
		}
	}
	return s.collectFinancialTotalsFallback(ctx, fromT, toT)
}

func (s *reportsService) collectFinancialTotalsFallback(ctx context.Context, fromT, toT time.Time) (map[string]*reportDayTotals, float64, float64, float64) {
	buckets := map[string]*reportDayTotals{}
	var totalDeposits, totalWithdrawals, totalChargebacks float64

	var deposits []models.Deposit
	if err := s.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&deposits).Error; err != nil {
		s.log.Error("financial report deposits fallback failed", zap.Error(err))
	} else {
		for _, deposit := range deposits {
			amount := parseMoney(deposit.Amount)
			bucket := ensureDayTotals(buckets, deposit.CreatedAt.Format("2006-01-02"))
			bucket.deposits += amount
			totalDeposits += amount
		}
	}

	var withdrawals []models.Withdrawal
	if err := s.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&withdrawals).Error; err != nil {
		s.log.Error("financial report withdrawals fallback failed", zap.Error(err))
	} else {
		for _, withdrawal := range withdrawals {
			amount := parseMoney(withdrawal.Amount)
			bucket := ensureDayTotals(buckets, withdrawal.CreatedAt.Format("2006-01-02"))
			bucket.withdrawals += amount
			totalWithdrawals += amount
		}
	}

	chargebackBuckets, totalChargebackAmount := s.collectChargebacks(ctx, fromT, toT)
	for period, bucket := range chargebackBuckets {
		target := ensureDayTotals(buckets, period)
		target.chargebacks += bucket.chargebacks
	}
	totalChargebacks += totalChargebackAmount

	return buckets, totalDeposits, totalWithdrawals, totalChargebacks
}

func (s *reportsService) collectChargebacks(ctx context.Context, fromT, toT time.Time) (map[string]*reportDayTotals, float64) {
	if s.db == nil {
		return map[string]*reportDayTotals{}, 0
	}
	var chargebacks []models.Chargeback
	if err := s.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&chargebacks).Error; err != nil {
		s.log.Error("financial report chargebacks failed", zap.Error(err))
		return map[string]*reportDayTotals{}, 0
	}
	buckets := map[string]*reportDayTotals{}
	var totalChargebacks float64
	for _, chargeback := range chargebacks {
		amount := parseMoney(chargeback.Amount)
		bucket := ensureDayTotals(buckets, chargeback.CreatedAt.Format("2006-01-02"))
		bucket.chargebacks += amount
		totalChargebacks += amount
	}
	return buckets, totalChargebacks
}

func (s *reportsService) playerAnalytics(c *fiber.Ctx) error {
	from := c.Query("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.Query("to", time.Now().Format("2006-01-02"))
	_ = c.Query("metric", "ltv")

	fromT, _ := time.Parse("2006-01-02", from)
	toT, _ := time.Parse("2006-01-02", to)
	toT = toT.Add(24 * time.Hour)

	rows, avg, err := s.collectPlayerAnalytics(c.Context(), fromT, toT)
	if err != nil {
		s.log.Error("player analytics failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "database error"})
	}

	return c.JSON(fiber.Map{
		"data": models.PlayerAnalyticsResponse{
			Count:      len(rows),
			Avg:        avg,
			TopSegment: "high",
			Rows:       rows,
		},
	})
}

func (s *reportsService) collectPlayerAnalytics(ctx context.Context, fromT, toT time.Time) ([]models.PlayerAnalyticsRow, float64, error) {
	var deposits []models.Deposit
	if err := s.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&deposits).Error; err != nil {
		return nil, 0, fmt.Errorf("query deposits: %w", err)
	}

	agg := make(map[int64]*playerAnalyticsStats)
	for _, deposit := range deposits {
		stats := agg[deposit.PlayerID]
		if stats == nil {
			stats = &playerAnalyticsStats{}
			agg[deposit.PlayerID] = stats
		}
		amount := parseMoney(deposit.Amount)
		stats.totalAmount += amount
		stats.count++
		if deposit.CreatedAt.After(stats.lastActive) {
			stats.lastActive = deposit.CreatedAt
		}
	}

	rows := make([]models.PlayerAnalyticsRow, 0, len(agg))
	var avg float64
	for playerID, stats := range agg {
		avg += stats.totalAmount
		segment := "low"
		if stats.totalAmount > 1000 {
			segment = "high"
		} else if stats.totalAmount > 200 {
			segment = "medium"
		}
		rows = append(rows, models.PlayerAnalyticsRow{
			UserID:      itoa64(playerID),
			MetricValue: stats.totalAmount,
			Segment:     &segment,
			LastActive:  stats.lastActive.Format("2006-01-02 15:04"),
		})
	}
	if len(agg) > 0 {
		avg = avg / float64(len(agg))
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].MetricValue > rows[j].MetricValue
	})

	return rows, avg, nil
}

func (s *reportsService) gameAnalytics(c *fiber.Ctx) error {
	from := c.Query("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.Query("to", time.Now().Format("2006-01-02"))

	fromT, _ := time.Parse("2006-01-02", from)
	toT, _ := time.Parse("2006-01-02", to)
	toT = toT.Add(24 * time.Hour)

	rows, avgRTP, varianceCount, err := s.collectGameAnalytics(c.Context(), fromT, toT)
	if err != nil {
		s.log.Error("game analytics failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "database error"})
	}

	return c.JSON(fiber.Map{
		"data": models.GameAnalyticsResponse{Rows: rows},
		"meta": fiber.Map{
			"avg_rtp":        avgRTP,
			"variance_count": varianceCount,
		},
	})
}

func (s *reportsService) collectGameAnalytics(ctx context.Context, fromT, toT time.Time) ([]models.GameAnalyticsRow, float64, float64, error) {
	theoreticalRTP := s.loadTheoreticalRTPMap(ctx)
	if s.ch != nil {
		if rows, err := s.ch.CasinoAnalyticsRows(ctx, fromT, toT); err == nil {
			return s.enrichClickHouseGameAnalytics(rows, theoreticalRTP), computeAverageRTP(rows), computeVarianceCount(rows, theoreticalRTP), nil
		} else if !client.IsClickHouseNotConfigured(err) {
			s.log.Warn("clickhouse game analytics query failed, using postgres fallback", zap.Error(err))
		}
	}
	return s.collectGameAnalyticsFallback(ctx, fromT, toT, theoreticalRTP)
}

func (s *reportsService) enrichClickHouseGameAnalytics(rows []client.CasinoAnalyticsRow, theoreticalRTP map[string]float64) []models.GameAnalyticsRow {
	result := make([]models.GameAnalyticsRow, 0, len(rows))
	for _, row := range rows {
		theoretical := lookupTheoreticalRTP(theoreticalRTP, row.GameName, row.Provider, strconv.FormatUint(uint64(row.GameID), 10))
		if theoretical == 0 {
			theoretical = row.ActualRTP
		}
		result = append(result, models.GameAnalyticsRow{
			GameID:         strconv.FormatUint(uint64(row.GameID), 10),
			GameName:       row.GameName,
			Provider:       row.Provider,
			GGR:            formatMoney(row.GGR),
			Rounds:         row.Rounds,
			UniquePlayers:  row.UniquePlayers,
			ActualRTP:      row.ActualRTP,
			TheoreticalRTP: theoretical,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return parseMoney(result[i].GGR) > parseMoney(result[j].GGR)
	})
	return result
}

func (s *reportsService) collectGameAnalyticsFallback(ctx context.Context, fromT, toT time.Time, theoreticalRTP map[string]float64) ([]models.GameAnalyticsRow, float64, float64, error) {
	var rounds []models.CasinoGameRound
	if err := s.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&rounds).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("query casino rounds: %w", err)
	}

	var games []models.CasinoGame
	if err := s.db.WithContext(ctx).Select("id", "name", "display_name", "provider_name", "rtp").Find(&games).Error; err != nil {
		return nil, 0, 0, fmt.Errorf("query casino games: %w", err)
	}

	gameLookup := make(map[string]models.CasinoGame, len(games))
	for _, game := range games {
		gameLookup[normalizeAnalyticsKey(game.ID)] = game
	}

	type roundStats struct {
		gameID        string
		betAmount     float64
		winAmount     float64
		rounds        int64
		uniquePlayers map[int64]struct{}
	}
	agg := make(map[string]*roundStats)
	for _, round := range rounds {
		stats := agg[round.GameID]
		if stats == nil {
			stats = &roundStats{gameID: round.GameID, uniquePlayers: map[int64]struct{}{}}
			agg[round.GameID] = stats
		}
		stats.betAmount += parseMoney(round.BetAmount)
		stats.winAmount += parseMoney(round.WinAmount)
		stats.rounds++
		stats.uniquePlayers[round.UserID] = struct{}{}
	}

	rows := make([]models.GameAnalyticsRow, 0, len(agg))
	var avgRTP, varianceCount float64
	for gameID, stats := range agg {
		game := gameLookup[normalizeAnalyticsKey(gameID)]
		gameName := game.Name
		if gameName == "" {
			gameName = gameID
		}
		provider := game.ProviderName
		if provider == "" {
			provider = "unknown"
		}
		if game.DisplayName != nil && *game.DisplayName != "" {
			gameName = *game.DisplayName
		}
		actualRTP := 0.0
		if stats.betAmount > 0 {
			actualRTP = (stats.winAmount / stats.betAmount) * 100
		}
		theoretical := lookupTheoreticalRTP(theoreticalRTP, game.ID, game.ExternalID, gameName, provider)
		if theoretical == 0 {
			theoretical = actualRTP
		}
		ggr := stats.betAmount - stats.winAmount
		avgRTP += actualRTP
		if math.Abs(actualRTP-theoretical) > 5.0 {
			varianceCount++
		}
		rows = append(rows, models.GameAnalyticsRow{
			GameID:         gameID,
			GameName:       gameName,
			Provider:       provider,
			GGR:            formatMoney(ggr),
			Rounds:         stats.rounds,
			UniquePlayers:  int64(len(stats.uniquePlayers)),
			ActualRTP:      actualRTP,
			TheoreticalRTP: theoretical,
		})
	}
	if len(agg) > 0 {
		avgRTP = avgRTP / float64(len(agg))
	}
	sort.Slice(rows, func(i, j int) bool {
		return parseMoney(rows[i].GGR) > parseMoney(rows[j].GGR)
	})
	return rows, avgRTP, varianceCount, nil
}

func (s *reportsService) loadTheoreticalRTPMap(ctx context.Context) map[string]float64 {
	if s.db == nil {
		return map[string]float64{}
	}
	var games []models.CasinoGame
	if err := s.db.WithContext(ctx).Select("id", "external_id", "name", "display_name", "provider_name", "rtp").Find(&games).Error; err != nil {
		s.log.Error("load casino games for rtp lookup failed", zap.Error(err))
		return map[string]float64{}
	}
	lookup := make(map[string]float64, len(games)*4)
	for _, game := range games {
		lookup[normalizeAnalyticsKey(game.ID)] = game.Rtp
		lookup[normalizeAnalyticsKey(game.ExternalID)] = game.Rtp
		lookup[normalizeAnalyticsKey(game.Name)] = game.Rtp
		lookup[normalizeAnalyticsKey(game.ProviderName, game.Name)] = game.Rtp
		lookup[normalizeAnalyticsKey(game.ProviderName, game.ExternalID)] = game.Rtp
		if game.DisplayName != nil && *game.DisplayName != "" {
			lookup[normalizeAnalyticsKey(*game.DisplayName)] = game.Rtp
			lookup[normalizeAnalyticsKey(game.ProviderName, *game.DisplayName)] = game.Rtp
		}
	}
	return lookup
}

func computeAverageRTP(rows []client.CasinoAnalyticsRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	var total float64
	for _, row := range rows {
		total += row.ActualRTP
	}
	return total / float64(len(rows))
}

func computeVarianceCount(rows []client.CasinoAnalyticsRow, theoreticalRTP map[string]float64) float64 {
	var varianceCount float64
	for _, row := range rows {
		theoretical := lookupTheoreticalRTP(theoreticalRTP, row.GameName, row.Provider, strconv.FormatUint(uint64(row.GameID), 10))
		if theoretical == 0 {
			theoretical = row.ActualRTP
		}
		if math.Abs(row.ActualRTP-theoretical) > 5.0 {
			varianceCount++
		}
	}
	return varianceCount
}

func lookupTheoreticalRTP(lookup map[string]float64, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := lookup[normalizeAnalyticsKey(key)]; ok {
			return value
		}
	}
	return 0
}

func (s *reportsService) complianceReport(c *fiber.Ctx) error {
	from := c.Query("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.Query("to", time.Now().Format("2006-01-02"))
	reportType := c.Query("type", "aml")

	fromT, _ := time.Parse("2006-01-02", from)
	toT, _ := time.Parse("2006-01-02", to)
	toT = toT.Add(24 * time.Hour)

	var flagged, resolved, pending, highRisk int64
	if reportType == "aml" || reportType == "all" {
		var alerts []models.RiskAlert
		if err := s.db.WithContext(c.Context()).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&alerts).Error; err != nil {
			s.log.Error("compliance risk alerts failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		for _, alert := range alerts {
			switch strings.ToLower(alert.Status) {
			case "open", "new":
				flagged++
				pending++
			case "resolved", "dismissed":
				resolved++
			default:
				flagged++
			}
			if strings.EqualFold(alert.Severity, "high") || strings.EqualFold(alert.Severity, "critical") {
				highRisk++
			}
		}
	}

	if reportType == "kyc" || reportType == "all" {
		var reviews []models.KycReview
		if err := s.db.WithContext(c.Context()).Where("created_at >= ? AND created_at < ?", fromT, toT).Find(&reviews).Error; err != nil {
			s.log.Error("compliance kyc failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "database error"})
		}
		for _, review := range reviews {
			switch strings.ToLower(review.Status) {
			case "pending", "in_review":
				pending++
				flagged++
			case "approved":
				resolved++
			default:
				flagged++
			}
		}
	}

	rows := []models.ComplianceReportRow{{
		ID:            "summary",
		ReportDate:    to,
		Type:          reportType,
		FlaggedCount:  int(flagged),
		ResolvedCount: int(resolved),
		RiskLevel: func() string {
			if highRisk > 0 {
				return "high"
			}
			return "low"
		}(),
		GeneratedBy: "system",
	}}

	return c.JSON(fiber.Map{
		"data": models.ComplianceReportResponse{
			Flagged:  int(flagged),
			Resolved: int(resolved),
			Pending:  int(pending),
			HighRisk: int(highRisk),
			Rows:     rows,
		},
	})
}

func ensureDayTotals(buckets map[string]*reportDayTotals, period string) *reportDayTotals {
	bucket := buckets[period]
	if bucket == nil {
		bucket = &reportDayTotals{}
		buckets[period] = bucket
	}
	return bucket
}

func normalizeAnalyticsKey(parts ...string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, "::")
}

func formatMoney(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func parseMoney(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}
