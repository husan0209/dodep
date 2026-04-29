package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opus-casino/admin-bff/internal/client"
	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
)

type fakeAnalyticsClient struct {
	financialRows []client.FinancialReportRow
	financialErr  error
}

func (f *fakeAnalyticsClient) FinancialReportRows(ctx context.Context, from, to time.Time) ([]client.FinancialReportRow, error) {
	return f.financialRows, f.financialErr
}

func (f *fakeAnalyticsClient) CasinoAnalyticsRows(ctx context.Context, from, to time.Time) ([]client.CasinoAnalyticsRow, error) {
	return nil, nil
}

func TestFinancialReportUsesClickHouseRows(t *testing.T) {
	svc := &reportsService{
		log: zap.NewNop(),
		ch: &fakeAnalyticsClient{
			financialRows: []client.FinancialReportRow{
				{Period: "2024-01-01", Deposits: 100, Withdrawals: 40},
				{Period: "2024-01-02", Deposits: 25, Withdrawals: 5},
			},
		},
	}

	app := fiber.New()
	app.Get("/report", svc.financialReport)

	req := httptest.NewRequest(http.MethodGet, "/report?from=2024-01-01&to=2024-01-02", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	var body struct {
		Data models.FinancialReportResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Data.TotalDeposits != "125.00" {
		t.Fatalf("unexpected total deposits: %s", body.Data.TotalDeposits)
	}
	if body.Data.TotalWithdrawals != "45.00" {
		t.Fatalf("unexpected total withdrawals: %s", body.Data.TotalWithdrawals)
	}
	if body.Data.NetRevenue != "80.00" {
		t.Fatalf("unexpected net revenue: %s", body.Data.NetRevenue)
	}
	if body.Data.TotalChargebacks != "0.00" {
		t.Fatalf("unexpected total chargebacks: %s", body.Data.TotalChargebacks)
	}
	if len(body.Data.Rows) != 2 {
		t.Fatalf("unexpected row count: %d", len(body.Data.Rows))
	}
	if body.Data.Rows[0].Period != "2024-01-01" || body.Data.Rows[0].NetRevenue != "60.00" {
		t.Fatalf("unexpected first row: %+v", body.Data.Rows[0])
	}
	if body.Data.Rows[1].Period != "2024-01-02" || body.Data.Rows[1].NetRevenue != "20.00" {
		t.Fatalf("unexpected second row: %+v", body.Data.Rows[1])
	}
}
