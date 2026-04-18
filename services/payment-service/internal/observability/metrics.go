package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "payment"
	subsystem = ""
)

// Metric names following OpenMetrics conventions
const (
	MetricDepositsTotal       = "deposits_total"
	MetricWithdrawalsTotal    = "withdrawals_total"
	MetricDepositAmountUSD    = "deposit_amount_usd"
	MetricWithdrawalAmountUSD = "withdrawal_amount_usd"
	MetricProviderLatency     = "provider_latency_seconds"
	MetricErrorsTotal         = "errors_total"
)

// Label names
const (
	LabelStatus    = "status"
	LabelCurrency  = "currency"
	LabelKYCLevel  = "kyc_level"
	LabelErrorType = "error_type"
	LabelOperation = "operation"
)

var (
	// DepositCounter tracks the total number of deposit operations.
	// Labels: status (pending, completed, failed), currency (BTC, ETH, USDT, etc.), kyc_level (0-3)
	DepositCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      MetricDepositsTotal,
			Help:      "Total number of deposit operations by status, currency, and KYC level.",
		},
		[]string{LabelStatus, LabelCurrency, LabelKYCLevel},
	)

	// WithdrawalCounter tracks the total number of withdrawal operations.
	// Labels: status (processing, completed, failed), currency (BTC, ETH, USDT, etc.), kyc_level (0-3)
	WithdrawalCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      MetricWithdrawalsTotal,
			Help:      "Total number of withdrawal operations by status, currency, and KYC level.",
		},
		[]string{LabelStatus, LabelCurrency, LabelKYCLevel},
	)

	// DepositAmountUSD tracks deposit amounts in USD.
	// Uses histogram to show distribution of deposit values.
	DepositAmountUSD = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      MetricDepositAmountUSD,
			Help:      "Distribution of deposit amounts in USD.",
			// Buckets optimized for payment amounts: small ($10) to large ($50,000)
			Buckets: []float64{
				10, 25, 50, 100, 250, 500,
				1000, 2500, 5000, 10000, 25000, 50000,
			},
		},
	)

	// WithdrawalAmountUSD tracks withdrawal amounts in USD.
	// Uses histogram to show distribution of withdrawal values.
	WithdrawalAmountUSD = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      MetricWithdrawalAmountUSD,
			Help:      "Distribution of withdrawal amounts in USD.",
			// Buckets optimized for withdrawal amounts
			Buckets: []float64{
				10, 25, 50, 100, 250, 500,
				1000, 2500, 5000, 10000, 25000, 50000,
			},
		},
	)

	// ProviderLatency tracks NOWPayments API call latency in seconds.
	// Labels: operation (create_payment, create_payout, get_rate, get_currencies)
	ProviderLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      MetricProviderLatency,
			Help:      "Latency of NOWPayments API calls in seconds.",
			// Buckets optimized for API latency: 10ms to 30s
			Buckets: []float64{
				0.01, 0.025, 0.05, 0.1, 0.25, 0.5,
				1, 2.5, 5, 10, 30,
			},
		},
		[]string{LabelOperation},
	)

	// ErrorCounter tracks payment operation errors.
	// Labels: error_type (kyc_required, limit_exceeded, insufficient_balance, provider_error, etc.),
	// operation (deposit, withdrawal, webhook)
	ErrorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      MetricErrorsTotal,
			Help:      "Total number of payment operation errors by type and operation.",
		},
		[]string{LabelErrorType, LabelOperation},
	)
)

// RegisterMetrics ensures all metrics are registered with the default registry.
// This is called automatically via promauto, but this function can be used
// to explicitly register metrics in tests or custom registries.
func RegisterMetrics(registry prometheus.Registerer) {
	registry.MustRegister(
		DepositCounter,
		WithdrawalCounter,
		DepositAmountUSD,
		WithdrawalAmountUSD,
		ProviderLatency,
		ErrorCounter,
	)
}

// RecordDeposit records a deposit operation metric.
// status: pending, completed, failed, expired
// currency: BTC, ETH, USDT_ERC20, USDT_TRC20, USDC, LTC, BCH
// kycLevel: 0, 1, 2, 3
// amountUSD: deposit amount in USD
func RecordDeposit(status, currency string, kycLevel int, amountUSD float64) {
	DepositCounter.WithLabelValues(status, currency, kycLevelLabel(kycLevel)).Inc()
	DepositAmountUSD.Observe(amountUSD)
}

// RecordWithdrawal records a withdrawal operation metric.
// status: processing, completed, failed, cancelled
// currency: BTC, ETH, USDT_ERC20, USDT_TRC20, USDC, LTC, BCH
// kycLevel: 0, 1, 2, 3
// amountUSD: withdrawal amount in USD
func RecordWithdrawal(status, currency string, kycLevel int, amountUSD float64) {
	WithdrawalCounter.WithLabelValues(status, currency, kycLevelLabel(kycLevel)).Inc()
	WithdrawalAmountUSD.Observe(amountUSD)
}

// RecordProviderLatency records NOWPayments API call latency.
// operation: create_payment, create_payout, get_rate, get_currencies, verify_webhook
// latencySeconds: duration of the API call in seconds
func RecordProviderLatency(operation string, latencySeconds float64) {
	ProviderLatency.WithLabelValues(operation).Observe(latencySeconds)
}

// RecordError records a payment operation error.
// errorType: kyc_required, limit_exceeded, insufficient_balance, provider_error,
// invalid_address, currency_not_supported, webhook_invalid, wallet_unavailable
// operation: deposit, withdrawal, webhook
func RecordError(errorType, operation string) {
	ErrorCounter.WithLabelValues(errorType, operation).Inc()
}

// kycLevelLabel converts KYC level int to string label.
func kycLevelLabel(level int) string {
	switch level {
	case 0, 1, 2, 3:
		return string(rune('0' + level))
	default:
		return "unknown"
	}
}
