// Package telemetry provides Prometheus instrumentation for Bonus Service.
//
// Design rules (architecture-overview.skill.md §7 + Prometheus cardinality
// guidance):
//   - metric names are prefixed with the service name: bonus_*;
//   - RED metrics for every entry point (HTTP + gRPC) with bounded label sets;
//   - HTTP labels use the *route pattern* (`/api/v1/bonuses/:id`), never the
//     raw path, so path parameters cannot explode cardinality;
//   - business metrics cover the money-critical flow (award → wagering →
//     conversion credit), because that is what reconciliation and alerting
//     are built on.
//
// Metrics are registered against an explicit prometheus.Registerer so tests can
// use an isolated registry instead of the global default one.
package telemetry

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// latencyBuckets follows the platform latency budget buckets
// (architecture-overview.skill.md §7): 1ms … 5s.
var latencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0}

// Metrics holds every collector of the service.
//
// All label values must come from a bounded set (routes, methods, status codes,
// enums, currency codes). Never pass user input, IDs or error strings.
type Metrics struct {
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	grpcRequests *prometheus.CounterVec
	grpcDuration *prometheus.HistogramVec

	bonusesAwarded  *prometheus.CounterVec
	bonusAwardSkip  *prometheus.CounterVec
	wagersRecorded  *prometheus.CounterVec
	wageringDone    *prometheus.CounterVec
	conversionCreds *prometheus.CounterVec
	paymentEvents   *prometheus.CounterVec
	wageringRatio   *prometheus.HistogramVec
}

// New builds Metrics and registers every collector on reg.
// Passing a non-nil registry is mandatory; use prometheus.NewRegistry() in
// tests to keep collectors isolated between test cases.
func New(reg prometheus.Registerer) *Metrics {
	f := promautoFactory{reg: reg}
	return &Metrics{
		httpRequests: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_http_requests_total",
			Help: "Total HTTP requests handled by bonus-service.",
		}, []string{"method", "route", "status"}),

		httpDuration: f.histogramVec(prometheus.HistogramOpts{
			Name:    "bonus_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: latencyBuckets,
		}, []string{"method", "route"}),

		grpcRequests: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_grpc_requests_total",
			Help: "Total gRPC requests handled by bonus-service.",
		}, []string{"method", "code"}),

		grpcDuration: f.histogramVec(prometheus.HistogramOpts{
			Name:    "bonus_grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds.",
			Buckets: latencyBuckets,
		}, []string{"method"}),

		bonusesAwarded: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_bonuses_awarded_total",
			Help: "Welcome bonuses created, by bonus type and currency.",
		}, []string{"type", "currency"}),

		bonusAwardSkip: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_bonuses_award_skipped_total",
			Help: "Welcome bonus awards that did not create a bonus, by reason.",
		}, []string{"reason"}),

		wagersRecorded: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_wagers_recorded_total",
			Help: "Wager events processed, by outcome.",
		}, []string{"result"}),

		wageringDone: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_wagering_completed_total",
			Help: "Bonuses whose wagering requirement was met, by type and currency.",
		}, []string{"type", "currency"}),

		conversionCreds: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_conversion_credits_total",
			Help: "Wallet credits for completed bonus wagering, by result.",
		}, []string{"result"}),

		paymentEvents: f.counterVec(prometheus.CounterOpts{
			Name: "bonus_payment_events_total",
			Help: "Consumed payments.completed events, by outcome.",
		}, []string{"result"}),

		wageringRatio: f.histogramVec(prometheus.HistogramOpts{
			Name:    "bonus_wagering_progress_ratio",
			Help:    "Distribution of wagering completion ratio at bonus completion time (0..1+).",
			Buckets: []float64{0.01, 0.1, 0.25, 0.5, 0.75, 0.9, 1.0, 1.25, 2.0},
		}, []string{"type"}),
	}
}

// promautoFactory registers collectors on the supplied Registerer, mirroring
// promauto but without touching the global default registry.
type promautoFactory struct {
	reg prometheus.Registerer
}

func (f promautoFactory) counterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	c := prometheus.NewCounterVec(opts, labels)
	mustRegister(f.reg, c)
	return c
}

func (f promautoFactory) histogramVec(opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	h := prometheus.NewHistogramVec(opts, labels)
	mustRegister(f.reg, h)
	return h
}

func mustRegister(reg prometheus.Registerer, c prometheus.Collector) {
	if reg == nil {
		return
	}
	// AlreadyRegisteredError is tolerated so multiple Metrics can share a
	// registry (e.g. re-created app instances in tests).
	_ = reg.Register(c)
}

// ── HTTP ──────────────────────────────────────────────────────────────────────

// ObserveHTTP records one served HTTP request.
// route MUST be the route pattern, not the raw URL path.
func (m *Metrics) ObserveHTTP(method, route string, status int, d time.Duration) {
	if m == nil {
		return
	}
	m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

// ── gRPC ──────────────────────────────────────────────────────────────────────

// ObserveGRPC records one served gRPC call.
// method is the full method (e.g. bonus.v1.BonusService/GetBonus), code is the
// gRPC status code name (e.g. OK, NotFound).
func (m *Metrics) ObserveGRPC(method, code string, d time.Duration) {
	if m == nil {
		return
	}
	m.grpcRequests.WithLabelValues(method, code).Inc()
	m.grpcDuration.WithLabelValues(method).Observe(d.Seconds())
}

// ── Business: award ───────────────────────────────────────────────────────────

// BonusAwarded counts a created bonus.
func (m *Metrics) BonusAwarded(bonusType, currency, amount string) {
	if m == nil {
		return
	}
	m.bonusesAwarded.WithLabelValues(bonusType, currency).Inc()
}

// BonusAwardSkipped counts a non-created award attempt.
// reason must be a bounded enum: already_awarded | invalid_amount.
func (m *Metrics) BonusAwardSkipped(reason string) {
	if m == nil {
		return
	}
	m.bonusAwardSkip.WithLabelValues(reason).Inc()
}

// ── Business: wagering ────────────────────────────────────────────────────────

// WagerRecorded counts a wager event outcome.
// result must be a bounded enum: recorded | no_active_bonus | expired |
// credit_failed | invalid_amount.
func (m *Metrics) WagerRecorded(result string) {
	if m == nil {
		return
	}
	m.wagersRecorded.WithLabelValues(result).Inc()
}

// WageringCompleted counts a bonus whose wagering requirement was met and
// records the completion ratio for funnel analytics.
func (m *Metrics) WageringCompleted(bonusType, currency, ratio string) {
	if m == nil {
		return
	}
	m.wageringDone.WithLabelValues(bonusType, currency).Inc()
	if v, err := strconv.ParseFloat(ratio, 64); err == nil {
		m.wageringRatio.WithLabelValues(bonusType).Observe(v)
	}
}

// ── Business: conversion credit ───────────────────────────────────────────────

// ConversionCredit counts a wallet credit attempt for completed wagering.
// result must be a bounded enum: credited | skipped_no_wallet | failed.
func (m *Metrics) ConversionCredit(result string) {
	if m == nil {
		return
	}
	m.conversionCreds.WithLabelValues(result).Inc()
}

// ── Business: payment events ──────────────────────────────────────────────────

// PaymentEvent counts a consumed payments.completed event.
// result must be a bounded enum: awarded | skipped_not_first | malformed |
// failed.
func (m *Metrics) PaymentEvent(result string) {
	if m == nil {
		return
	}
	m.paymentEvents.WithLabelValues(result).Inc()
}
