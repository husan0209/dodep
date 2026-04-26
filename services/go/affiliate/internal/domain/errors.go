package domain

import "errors"

var (
	ErrEnrollmentAlreadyPending = errors.New("affiliate enrollment already pending")
	ErrEnrollmentNotFound       = errors.New("affiliate enrollment not found")
	ErrAffiliateNotFound        = errors.New("affiliate profile not found")
	ErrAffiliateAlreadyExists   = errors.New("affiliate profile already exists for user")
	ErrAffiliateInactive        = errors.New("affiliate profile is not active")
	ErrAffiliateLinkNotFound    = errors.New("affiliate link not found")
	ErrSelfReferral             = errors.New("self-referral is not allowed")
	ErrAttributionAlreadyBound  = errors.New("referred user is already bound to an affiliate")
	ErrAffiliateKYCRequired     = errors.New("affiliate kyc verification required")
	ErrAffiliateFraudBlocked    = errors.New("affiliate payout blocked by fraud review")
	ErrMinPayoutNotReached      = errors.New("minimum payout threshold not reached")
	ErrInvalidPayoutAmount      = errors.New("invalid affiliate payout amount")
	ErrPayoutMethodNotFound     = errors.New("affiliate payout method not found")
	ErrPayoutNotFound           = errors.New("affiliate payout not found")
	ErrInvalidPayoutStatus      = errors.New("invalid affiliate payout status transition")
	ErrInvalidCommissionAmount  = errors.New("invalid affiliate commission amount")
)
