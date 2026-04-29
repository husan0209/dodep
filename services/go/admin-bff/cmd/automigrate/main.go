package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/config"
	"github.com/opus-casino/admin-bff/internal/models"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()

	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal("db connect", zap.Error(err))
	}
	defer dbPool.Close()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("gorm open", zap.Error(err))
	}

	if err := db.AutoMigrate(
		&models.KycDocument{},
		&models.KycReview{},
		&models.ScreeningResult{},
		&models.SofRequest{},
		&models.SofDocument{},
		&models.RgAlert{},
		&models.RgLimit{},
		&models.KycTeamMetric{},
		&models.Chargeback{},
		&models.CryptoWallet{},
		&models.P2PTransaction{},
		&models.ReconciliationRecord{},
		&models.PaymentMethodConfig{},
		&models.Deposit{},
		&models.Withdrawal{},
		&models.Bonus{},
		&models.PlayerBonus{},
		&models.BonusActivation{},
		&models.WageringMonitor{},
		&models.SupportTicket{},
		&models.SupportMessage{},
		&models.TicketLink{},
		&models.SLAConfig{},
		&models.AgentWorkload{},
		&models.RiskAlert{},
		&models.RiskRule{},
		&models.RiskAuditLog{},
		&models.RiskWatchlistEntry{},
		&models.RiskRuleWhitelist{},
		&models.SportsEvent{},
		&models.SportsMarket{},
		&models.OddsOverride{},
		&models.MarginSetting{},
		&models.StakeLimit{},
		&models.LiabilitySnapshot{},
		&models.CasinoGame{},
		&models.CasinoProvider{},
		&models.RtpConfig{},
		&models.JackpotPool{},
		&models.ProviderSettlement{},
		&models.RtpAuditLog{},
		&models.Affiliate{},
		&models.AffiliatePayout{},
		&models.FraudFlag{},
		&models.PostbackLog{},
		&models.AdminUser{},
		&models.AdminSession{},
		&models.AuditLog{},
		&models.SystemSetting{},
		&models.IPWhitelistEntry{},
		&models.RegulatoryReport{},
		&models.SARReport{},
		&models.PlayerComplaint{},
		&models.JurisdictionGGR{},
		&models.TaxConfig{},
	); err != nil {
		log.Fatal("auto-migrate", zap.Error(err))
	}

	fmt.Println("AutoMigrate completed")
}
