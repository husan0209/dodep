package event

// Topic constants for the affiliate domain. Dotted names match the existing
// outbox topics (affiliate.commission.accrued, affiliate.payout.requested,
// ...) so current consumers keep working; new topics are additive.
const (
	TopicCommissionAccrued  = "affiliate.commission.accrued"
	TopicCommissionReleased = "affiliate.commission.released"
	TopicCommissionReversed = "affiliate.commission.reversed"
	TopicPayoutRequested    = "affiliate.payout.requested"
	TopicPayoutApproved     = "affiliate.payout.approved"
	TopicPayoutPaid         = "affiliate.payout.paid"
	TopicPayoutRejected     = "affiliate.payout.rejected"
	TopicFraudFlagged       = "affiliate.fraud.flagged"
	TopicAdjustmentCreated  = "affiliate.adjustment.created"
)

// CommissionPayload is the outbox payload for accrued / released / reversed
// commission events. Amounts are decimal strings (shopspring format), never
// floats — see CONVENTIONS.md NEVER-6.
func CommissionPayload(earningID, affiliateID string, referredUserID int64, commissionAmount, commissionRate, ngrAmount string, status, idempotencyKey string, holdUntil any) map[string]any {
	return map[string]any{
		"earning_id":        earningID,
		"affiliate_id":      affiliateID,
		"referred_user_id":  referredUserID,
		"commission_amount": commissionAmount,
		"commission_rate":   commissionRate,
		"ngr_amount":        ngrAmount,
		"status":            status,
		"hold_until":        holdUntil,
		"idempotency_key":   idempotencyKey,
	}
}

// PayoutPayload is the outbox payload for requested / approved / paid /
// rejected payout events. Kept separate from player-wallet flows: affiliate
// payouts run through their own financial contour.
func PayoutPayload(payoutID, affiliateID, amount, currency, methodID, status, idempotencyKey string) map[string]any {
	return map[string]any{
		"payout_id":       payoutID,
		"affiliate_id":    affiliateID,
		"amount":          amount,
		"currency":        currency,
		"method_id":       methodID,
		"status":          status,
		"idempotency_key": idempotencyKey,
	}
}

// FraudPayload is the outbox payload for fraud-flag events.
func FraudPayload(flagID, affiliateID string, referredUserID int64, flagType, severity, status string) map[string]any {
	return map[string]any{
		"fraud_flag_id":    flagID,
		"affiliate_id":     affiliateID,
		"referred_user_id": referredUserID,
		"flag_type":        flagType,
		"severity":         severity,
		"status":           status,
	}
}
