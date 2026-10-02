package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/opus-casino/affiliate/internal/domain"
	"github.com/opus-casino/affiliate/internal/fraud"
	"github.com/opus-casino/affiliate/internal/repository"
)

// fraudStore adapts the repository to the anti-fraud engine's SignalStore.
// The engine depends on the narrow interface, so rule logic stays free of GORM.
type fraudStore struct {
	repo repository.AffiliateRepository
}

func (f fraudStore) GetClickByClickID(ctx context.Context, clickID string) (*domain.AffiliateClick, error) {
	return f.repo.GetClickByClickID(ctx, clickID)
}

func (f fraudStore) CountClicksSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	return f.repo.CountClicksSince(ctx, affiliateID, since)
}

func (f fraudStore) CountAttributionsSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	return f.repo.CountAttributionsSince(ctx, affiliateID, since)
}

func (f fraudStore) CountReferredUsersByDevice(ctx context.Context, affiliateID uuid.UUID, deviceFP string, since time.Time) (int64, error) {
	return f.repo.CountReferredUsersByDevice(ctx, affiliateID, deviceFP, since)
}

func (f fraudStore) CountReferredUsersByIP(ctx context.Context, affiliateID uuid.UUID, ipHash string, since time.Time) (int64, error) {
	return f.repo.CountReferredUsersByIP(ctx, affiliateID, ipHash, since)
}

// ScreenAttribution runs the automatic anti-fraud engine over a freshly created
// attribution and persists one open fraud flag per finding.
//
// Called by BindReferredUser right after the attribution is written. Two
// deliberate properties:
//
//   - It never returns an error to the caller. Fraud screening must not break a
//     legitimate signup; findings become reviewable flags instead.
//   - It never hard-blocks. Open flags already gate payouts through the payout
//     eligibility check, so a finding tightens payout control while a risk
//     manager decides.
//
// A critical finding is logged for suspension review but not applied
// automatically: suspending an affiliate on a single signal would let one
// false positive stop real earnings.
func (s *AffiliateService) ScreenAttribution(ctx context.Context, attribution *domain.AffiliateAttribution) {
	if attribution == nil {
		return
	}

	verdict, err := fraud.Evaluate(ctx, fraudStore{repo: s.repo}, s.fraudCfg, fraud.Input{
		AffiliateID:    attribution.AffiliateID,
		ReferredUserID: attribution.ReferredUserID,
		ClickID:        attribution.ClickID,
		EvaluatedAt:    time.Now().UTC(),
	})
	if err != nil {
		s.logger.Warn("affiliate fraud evaluation failed",
			zap.String("attribution_id", attribution.ID.String()),
			zap.Error(err),
		)
		return
	}
	if verdict == nil {
		return
	}

	for _, finding := range verdict.Findings {
		_, flagErr := s.FlagAffiliateFraud(ctx, FlagAffiliateFraudInput{
			AffiliateID:    attribution.AffiliateID,
			ReferredUserID: attribution.ReferredUserID,
			FlagType:       finding.RuleID,
			Severity:       finding.Severity,
			Details:        finding.Details,
		})
		if flagErr != nil {
			s.logger.Warn("affiliate fraud flag creation failed",
				zap.String("rule_id", finding.RuleID),
				zap.Error(flagErr),
			)
		}
	}

	s.logger.Info("affiliate fraud flags raised",
		zap.String("affiliate_id", attribution.AffiliateID.String()),
		zap.Int("flags", len(verdict.Findings)),
		zap.String("max_severity", string(verdict.MaxSeverity)),
		zap.Bool("recommend_suspension", verdict.RecommendSuspension),
	)
}
