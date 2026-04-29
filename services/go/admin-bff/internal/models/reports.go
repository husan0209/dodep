package models

type FinancialReportRow struct {
	Period       string `json:"period"`
	Deposits     string `json:"deposits"`
	Withdrawals  string `json:"withdrawals"`
	NetRevenue   string `json:"net_revenue"`
	Chargebacks  string `json:"chargebacks"`
	Status       string `json:"status"`
}

type FinancialReportResponse struct {
	TotalDeposits     string               `json:"total_deposits"`
	TotalWithdrawals  string               `json:"total_withdrawals"`
	NetRevenue        string               `json:"net_revenue"`
	TotalChargebacks  string               `json:"total_chargebacks"`
	Rows              []FinancialReportRow `json:"rows"`
}

type PlayerAnalyticsRow struct {
	UserID      string  `json:"user_id"`
	MetricValue float64 `json:"metric_value"`
	Segment     *string `json:"segment,omitempty"`
	LastActive  string  `json:"last_active"`
}

type PlayerAnalyticsResponse struct {
	Count       int                  `json:"count"`
	Avg         float64              `json:"avg"`
	TopSegment  string               `json:"top_segment"`
	Rows        []PlayerAnalyticsRow `json:"rows"`
}

type GameAnalyticsRow struct {
	GameID       string  `json:"game_id"`
	GameName     string  `json:"game_name"`
	Provider     string  `json:"provider"`
	GGR          string  `json:"ggr"`
	Rounds       int64   `json:"rounds"`
	UniquePlayers int64  `json:"unique_players"`
	ActualRTP    float64 `json:"actual_rtp"`
	TheoreticalRTP float64 `json:"theoretical_rtp"`
}

type GameAnalyticsResponse struct {
	Rows []GameAnalyticsRow `json:"rows"`
}

type ComplianceReportRow struct {
	ID           string `json:"id"`
	ReportDate   string `json:"report_date"`
	Type         string `json:"type"`
	FlaggedCount int    `json:"flagged_count"`
	ResolvedCount int   `json:"resolved_count"`
	RiskLevel    string `json:"risk_level"`
	GeneratedBy  string `json:"generated_by"`
}

type ComplianceReportResponse struct {
	Flagged   int                     `json:"flagged"`
	Resolved  int                     `json:"resolved"`
	Pending   int                     `json:"pending"`
	HighRisk  int                     `json:"high_risk"`
	Rows      []ComplianceReportRow `json:"rows"`
}
