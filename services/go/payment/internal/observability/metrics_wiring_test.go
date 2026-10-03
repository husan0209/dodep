package observability

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/shopspring/decimal"
)

// These tests pin the properties the alerting rules depend on. The metrics are
// process-global (promauto on the default registry), so every assertion is
// relative to a value read immediately before the action under test.

// counterValue reads payment_deposits_total for one label set.
func counterValue(t *testing.T, status, currency, kycLevel string) float64 {
	t.Helper()
	return testutil.ToFloat64(DepositCounter.WithLabelValues(status, currency, kycLevel))
}

func withdrawalCounterValue(t *testing.T, status, currency, kycLevel string) float64 {
	t.Helper()
	return testutil.ToFloat64(WithdrawalCounter.WithLabelValues(status, currency, kycLevel))
}

func errorValue(t *testing.T, errorType, operation string) float64 {
	t.Helper()
	return testutil.ToFloat64(ErrorCounter.WithLabelValues(errorType, operation))
}

// histogramTotals returns the sample count and sum of a histogram across all
// its series. testutil.ToFloat64 panics on histograms, so this reads the
// exposition format directly.
func histogramTotals(t *testing.T, name string) (count uint64, sum float64) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			if h := m.GetHistogram(); h != nil {
				count += h.GetSampleCount()
				sum += h.GetSampleSum()
			}
		}
	}
	return count, sum
}

func TestRecordDeposit_IncrementsCounterAndObservesAmountOnce(t *testing.T) {
	const (
		status   = DepositStatusPending
		currency = "BTC"
		kyc      = "2"
	)
	exposed := namespace + "_" + MetricDepositAmountUSD

	beforeCounters := counterValue(t, status, currency, kyc)
	beforeCount, beforeSum := histogramTotals(t, exposed)

	RecordDeposit(status, currency, 2, decimal.NewFromInt(250))

	if got := counterValue(t, status, currency, kyc); got != beforeCounters+1 {
		t.Errorf("deposits_total{status=%s,currency=%s,kyc_level=%s} = %v, want %v",
			status, currency, kyc, got, beforeCounters+1)
	}

	afterCount, afterSum := histogramTotals(t, exposed)
	if afterCount != beforeCount+1 {
		t.Errorf("%s sample count = %d, want %d (one observation per request)", exposed, afterCount, beforeCount+1)
	}
	if afterSum != beforeSum+250 {
		t.Errorf("%s sum = %v, want %v", exposed, afterSum, beforeSum+250)
	}
}

// A deposit is counted once per request, so the amount histogram must see the
// same money exactly once. Later lifecycle transitions deliberately skip it.
func TestRecordDepositTransition_CountsWithoutObservingAmount(t *testing.T) {
	const (
		status   = DepositStatusCompleted
		currency = "ETH"
		kyc      = KYCUnknownLabel
	)
	exposed := namespace + "_" + MetricDepositAmountUSD

	beforeCounters := counterValue(t, status, currency, kyc)
	beforeCount, beforeSum := histogramTotals(t, exposed)

	RecordDepositTransition(status, currency, KYCLevelUnknown)

	if got := counterValue(t, status, currency, kyc); got != beforeCounters+1 {
		t.Errorf("deposits_total{status=%s} = %v, want %v", status, got, beforeCounters+1)
	}
	afterCount, afterSum := histogramTotals(t, exposed)
	if afterCount != beforeCount || afterSum != beforeSum {
		t.Errorf("%s changed on a lifecycle transition: count %d->%d sum %v->%v",
			exposed, beforeCount, afterCount, beforeSum, afterSum)
	}
}

func TestRecordWithdrawalTransition_CountsWithoutObservingAmount(t *testing.T) {
	const currency = "LTC"
	exposed := namespace + "_" + MetricWithdrawalAmountUSD

	beforeProcessing := withdrawalCounterValue(t, WithdrawalStatusProcessing, currency, "3")
	beforeFailed := withdrawalCounterValue(t, WithdrawalStatusFailed, currency, KYCUnknownLabel)
	beforeCount, beforeSum := histogramTotals(t, exposed)

	RecordWithdrawal(WithdrawalStatusProcessing, currency, 3, decimal.NewFromInt(75))
	RecordWithdrawalTransition(WithdrawalStatusFailed, currency, KYCLevelUnknown)

	if got := withdrawalCounterValue(t, WithdrawalStatusProcessing, currency, "3"); got != beforeProcessing+1 {
		t.Errorf("withdrawals_total{status=processing} = %v, want %v", got, beforeProcessing+1)
	}
	if got := withdrawalCounterValue(t, WithdrawalStatusFailed, currency, KYCUnknownLabel); got != beforeFailed+1 {
		t.Errorf("withdrawals_total{status=failed} = %v, want %v", got, beforeFailed+1)
	}

	afterCount, afterSum := histogramTotals(t, exposed)
	if afterCount != beforeCount+1 {
		t.Errorf("%s sample count = %d, want %d (one request, one observation)", exposed, afterCount, beforeCount+1)
	}
	if afterSum != beforeSum+75 {
		t.Errorf("%s sum = %v, want %v", exposed, afterSum, beforeSum+75)
	}
}

func TestKYCLevelLabel_IsBounded(t *testing.T) {
	cases := map[int]string{
		0:               "0",
		1:               "1",
		2:               "2",
		3:               "3",
		4:               KYCUnknownLabel,
		99:              KYCUnknownLabel,
		KYCLevelUnknown: KYCUnknownLabel,
	}
	for level, want := range cases {
		if got := kycLevelLabel(level); got != want {
			t.Errorf("kycLevelLabel(%d) = %q, want %q", level, got, want)
		}
	}
}

// The KYC level arrives over gRPC. An unmapped value must collapse to one
// label, otherwise a bad upstream response would mint a time series per
// request and the metric would become a memory leak.
func TestKYCLevelLabel_OutOfRangeDoesNotMintSeries(t *testing.T) {
	const currency = "XMR"

	// Create the series once so the next call only exercises the mapping.
	RecordDepositTransition(DepositStatusPending, currency, KYCLevelUnknown)
	before := kycLabelValues(t, MetricDepositsTotal, currency)

	// A level that is neither 0..3 nor the sentinel: it must land on the same
	// "unknown" series rather than becoming a new label value.
	RecordDepositTransition(DepositStatusPending, currency, 42)
	RecordDepositTransition(DepositStatusPending, currency, 43)
	after := kycLabelValues(t, MetricDepositsTotal, currency)

	if len(before) != len(after) {
		t.Errorf("out-of-range KYC levels minted new series for %s: %v -> %v", currency, before, after)
	}
	for _, v := range after {
		if v != KYCUnknownLabel {
			t.Errorf("kyc_level %q leaked through for an out-of-range level", v)
		}
	}
}

// kycLabelValues returns the distinct kyc_level values exposed for a currency.
func kycLabelValues(t *testing.T, metric, currency string) []string {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var out []string
	for _, f := range families {
		if f.GetName() != namespace+"_"+metric {
			continue
		}
		for _, m := range f.GetMetric() {
			cur, kyc := "", ""
			for _, l := range m.GetLabel() {
				switch l.GetName() {
				case LabelCurrency:
					cur = l.GetValue()
				case LabelKYCLevel:
					kyc = l.GetValue()
				}
			}
			if cur == currency {
				out = append(out, kyc)
			}
		}
	}
	return out
}

func TestRecordError_Increments(t *testing.T) {
	before := errorValue(t, "limit_exceeded", OperationDeposit)

	RecordError("limit_exceeded", OperationDeposit)

	if got := errorValue(t, "limit_exceeded", OperationDeposit); got != before+1 {
		t.Errorf("errors_total{error_type=limit_exceeded,operation=deposit} = %v, want %v",
			got, before+1)
	}
}

func TestRecordProviderLatency_RepeatsDoNotMintSeries(t *testing.T) {
	// The first observation for an operation creates its series; measure from
	// there so the assertion is about accumulation, not creation.
	RecordProviderLatency(ProviderOpCreatePayment, 0.5)
	beforeSeries := testutil.CollectAndCount(ProviderLatency)
	beforeCount, beforeSum := histogramTotals(t, namespace+"_"+MetricProviderLatency)

	RecordProviderLatency(ProviderOpCreatePayment, 1.5)
	RecordProviderLatency(ProviderOpCreatePayment, 2.5)

	// Observations accumulate into an existing series. Under load, one series
	// per call would be a leak.
	if after := testutil.CollectAndCount(ProviderLatency); after != beforeSeries {
		t.Errorf("provider_latency series count changed on repeat observation: %d -> %d", beforeSeries, after)
	}

	// Every call must be recorded exactly once, or a latency quantile would
	// silently under-report.
	afterCount, afterSum := histogramTotals(t, namespace+"_"+MetricProviderLatency)
	if afterCount != beforeCount+2 {
		t.Errorf("provider_latency sample count = %d, want %d (one per call)", afterCount, beforeCount+2)
	}
	if afterSum != beforeSum+1.5+2.5 {
		t.Errorf("provider_latency sum = %v, want %v", afterSum, beforeSum+1.5+2.5)
	}
}

// promauto prepends the namespace, so the exposed series is
// payment_provider_latency_seconds. Alerting rules reference these names, so a
// rename here silently breaks every dashboard.
func TestExposedMetricNames_CarryNamespaceExactlyOnce(t *testing.T) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	seen := map[string]bool{}
	for _, f := range families {
		if !strings.HasPrefix(f.GetName(), namespace+"_") {
			continue
		}
		if strings.Contains(f.GetName(), namespace+"_"+namespace+"_") {
			t.Errorf("%q looks double-prefixed", f.GetName())
		}
		seen[f.GetName()] = true
	}

	for _, want := range []string{
		namespace + "_" + MetricDepositsTotal,
		namespace + "_" + MetricWithdrawalsTotal,
		namespace + "_" + MetricErrorsTotal,
		namespace + "_" + MetricProviderLatency,
	} {
		if !seen[want] {
			t.Errorf("series %q is not exposed on the default registry", want)
		}
	}
}

func TestMetricNames_AreValidPrometheusNames(t *testing.T) {
	for _, name := range []string{
		MetricDepositsTotal,
		MetricWithdrawalsTotal,
		MetricDepositAmountUSD,
		MetricWithdrawalAmountUSD,
		MetricProviderLatency,
		MetricErrorsTotal,
	} {
		if !isValidMetricName(namespace + "_" + name) {
			t.Errorf("%q is not a valid Prometheus metric name", namespace+"_"+name)
		}
		if name != strings.ToLower(name) {
			t.Errorf("%q must be snake_case", name)
		}
	}
}

func isValidMetricName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == ':':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// Guard the shape the Prometheus exposition relies on: a counter with the
// documented labels, and histograms (not summaries) so quantiles are
// aggregatable across replicas.
func TestDepositCounter_HasExpectedLabelSchema(t *testing.T) {
	var m dto.Metric
	if err := DepositCounter.WithLabelValues("schema-probe", "BTC", "0").Write(&m); err != nil {
		t.Fatalf("write metric: %v", err)
	}

	want := map[string]bool{LabelStatus: false, LabelCurrency: false, LabelKYCLevel: false}
	for _, l := range m.GetLabel() {
		if _, ok := want[l.GetName()]; !ok {
			t.Errorf("unexpected label %q on deposits_total", l.GetName())
			continue
		}
		want[l.GetName()] = true
	}
	for name, found := range want {
		if !found {
			t.Errorf("deposits_total is missing label %q", name)
		}
	}
}
