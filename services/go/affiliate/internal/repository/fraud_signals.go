package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opus-casino/affiliate/internal/domain"
)

// Signal queries for the automatic anti-fraud engine (internal/fraud).
//
// Kept in a dedicated file so the anti-fraud read path stays separable from the
// transactional write path in gorm_repository.go.

// GetClickByClickID resolves a tracked click. Returns (nil, nil) when the click
// is unknown, which the engine treats as a missing signal rather than an error.
func (r *GormAffiliateRepository) GetClickByClickID(ctx context.Context, clickID string) (*domain.AffiliateClick, error) {
	var model affiliateClickModel
	err := r.db.WithContext(ctx).Where("click_id = ?", clickID).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get click by click id: %w", err)
	}
	return &domain.AffiliateClick{
		ID:                model.ID,
		AffiliateID:       model.AffiliateID,
		LinkID:            model.LinkID,
		ClickID:           model.ClickID,
		IPHash:            model.IPHash,
		UserAgentHash:     model.UserAgentHash,
		DeviceFingerprint: model.DeviceFingerprint,
		CountryCode:       model.CountryCode,
		LandingPage:       model.LandingPage,
		CreatedAt:         model.CreatedAt,
	}, nil
}

// CountClicksSince counts clicks for one affiliate inside a window. Backs the
// click-velocity rule (bot farms / click inflation).
func (r *GormAffiliateRepository) CountClicksSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&affiliateClickModel{}).
		Where("affiliate_id = ? AND created_at >= ?", affiliateID, since).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count clicks since: %w", err)
	}
	return n, nil
}

// CountAttributionsSince counts attributed signups for one affiliate inside a
// window. Backs the registration-velocity rule (farmed accounts).
func (r *GormAffiliateRepository) CountAttributionsSince(ctx context.Context, affiliateID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&affiliateAttributionModel{}).
		Where("affiliate_id = ? AND created_at >= ?", affiliateID, since).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count attributions since: %w", err)
	}
	return n, nil
}

// CountReferredUsersByDevice counts DISTINCT referred users attributed through
// clicks sharing one device fingerprint. Backs multi-accounting detection.
func (r *GormAffiliateRepository) CountReferredUsersByDevice(ctx context.Context, affiliateID uuid.UUID, deviceFP string, since time.Time) (int64, error) {
	return r.countReferredUsersByClickAttribute(ctx, affiliateID, "c.device_fingerprint", deviceFP, since)
}

// CountReferredUsersByIP counts DISTINCT referred users attributed through
// clicks sharing one IP hash. Backs IP-cluster detection (VPN exits, NAT).
func (r *GormAffiliateRepository) CountReferredUsersByIP(ctx context.Context, affiliateID uuid.UUID, ipHash string, since time.Time) (int64, error) {
	return r.countReferredUsersByClickAttribute(ctx, affiliateID, "c.ip_hash", ipHash, since)
}

// countReferredUsersByClickAttribute is the shared implementation for the
// device/IP rules: join attributions to the click that produced them, then
// count distinct referred users.
func (r *GormAffiliateRepository) countReferredUsersByClickAttribute(
	ctx context.Context,
	affiliateID uuid.UUID,
	column string,
	value string,
	since time.Time,
) (int64, error) {
	// column is a package-internal constant, never user input.
	var n int64
	err := r.db.WithContext(ctx).
		Table("affiliate_attributions AS a").
		Joins("JOIN affiliate_clicks AS c ON c.click_id = a.click_id").
		Where("a.affiliate_id = ?", affiliateID).
		Where(column+" = ?", value).
		Where("a.created_at >= ?", since).
		Distinct("a.referred_user_id").
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count referred users by %s: %w", column, err)
	}
	return n, nil
}
