package telemetry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opus-casino/proto/gen/go/bonus/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
)

// scrape renders the registry into Prometheus text exposition format.
func scrape(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(raw)
}

// ── Business metrics ─────────────────────────────────────────────────────────

func TestNew_RegistersAllCollectors(t *testing.T) {
	// GatherAndCount counts *registered* collectors, so it works before any
	// observation is made (an un-observed *Vec exposes nothing).
	reg := prometheus.NewRegistry()
	New(reg)
	// Describe-based check: works before any observation (Gather would be
	// empty because *Vec collectors have no children yet).
	names := registeredNames(t, reg)
	for _, name := range expectedMetricNames {
		assert.Contains(t, names, name, "collector %s must be registered", name)
	}
	assert.Len(t, names, len(expectedMetricNames))
}

// expectedMetricNames is the full instrumentation contract of the service.
var expectedMetricNames = []string{
	"bonus_http_requests_total",
	"bonus_http_request_duration_seconds",
	"bonus_grpc_requests_total",
	"bonus_grpc_request_duration_seconds",
	"bonus_bonuses_awarded_total",
	"bonus_bonuses_award_skipped_total",
	"bonus_wagers_recorded_total",
	"bonus_wagering_completed_total",
	"bonus_conversion_credits_total",
	"bonus_payment_events_total",
	"bonus_wagering_progress_ratio",
}

// registeredNames reads metric names from a registry's descriptors.
func registeredNames(t *testing.T, reg *prometheus.Registry) []string {
	t.Helper()
	ch := make(chan *prometheus.Desc, 64)
	go func() {
		reg.Describe(ch)
		close(ch)
	}()
	var names []string
	for d := range ch {
		names = append(names, metricNameFromDesc(d))
	}
	return names
}

// metricNameFromDesc extracts the metric name from a Desc string
// (`Desc{fqName: "bonus_http_requests_total", ...}`).
func metricNameFromDesc(d *prometheus.Desc) string {
	s := d.String()
	const key = `fqName: "`
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func TestNew_ExposesExpectedMetricNamesAfterUse(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.ObserveHTTP("GET", "/health", 200, time.Millisecond)
	m.ObserveGRPC("bonus.v1.BonusService/GetBonus", "OK", time.Millisecond)
	m.BonusAwarded("welcome", "USD", "10")
	m.BonusAwardSkipped("already_awarded")
	m.WagerRecorded("recorded")
	m.WageringCompleted("welcome", "USD", "1")
	m.ConversionCredit("credited")
	m.PaymentEvent("awarded")

	out := scrape(t, reg)
	for _, name := range expectedMetricNames {
		assert.Contains(t, out, name, "metric %s must be exposed", name)
	}
}

func TestBonusAwarded_CountsByTypeAndCurrency(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.BonusAwarded("welcome", "USD", "100.00")
	m.BonusAwarded("welcome", "USD", "50.00")

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_bonuses_awarded_total{currency="USD",type="welcome"} 2`)
}

func TestBonusAwardSkipped_ReasonLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.BonusAwardSkipped("already_awarded")
	m.BonusAwardSkipped("already_awarded")
	m.BonusAwardSkipped("invalid_amount")

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_bonuses_award_skipped_total{reason="already_awarded"} 2`)
	assert.Contains(t, out, `bonus_bonuses_award_skipped_total{reason="invalid_amount"} 1`)
}

func TestConversionCredit_OutcomesAreDistinct(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.ConversionCredit("credited")
	m.ConversionCredit("failed")
	m.ConversionCredit("skipped_no_wallet")

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_conversion_credits_total{result="credited"} 1`)
	assert.Contains(t, out, `bonus_conversion_credits_total{result="failed"} 1`)
	assert.Contains(t, out, `bonus_conversion_credits_total{result="skipped_no_wallet"} 1`)
}

func TestWageringCompleted_RecordsRatioHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.WageringCompleted("welcome", "USD", "1.25")
	m.WageringCompleted("welcome", "USD", "0.5")
	m.WageringCompleted("welcome", "USD", "not-a-number") // ignored, must not panic

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_wagering_completed_total{currency="USD",type="welcome"} 3`)
	assert.Contains(t, out, `bonus_wagering_progress_ratio_count{type="welcome"} 2`)
}

func TestPaymentEvent_Outcomes(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.PaymentEvent("awarded")
	m.PaymentEvent("skipped_not_first")
	m.PaymentEvent("skipped_not_first")
	m.PaymentEvent("malformed")
	m.PaymentEvent("failed")

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_payment_events_total{result="awarded"} 1`)
	assert.Contains(t, out, `bonus_payment_events_total{result="skipped_not_first"} 2`)
	assert.Contains(t, out, `bonus_payment_events_total{result="malformed"} 1`)
	assert.Contains(t, out, `bonus_payment_events_total{result="failed"} 1`)
}

func TestMetrics_NilReceiverIsSafe(t *testing.T) {
	// A nil *Metrics must be a no-op, not a panic: it lets callers wire
	// metrics optionally without branching everywhere.
	var m *Metrics
	assert.NotPanics(t, func() {
		m.ObserveHTTP("GET", "/health", 200, time.Millisecond)
		m.ObserveGRPC("m", "OK", time.Millisecond)
		m.BonusAwarded("welcome", "USD", "1")
		m.BonusAwardSkipped("r")
		m.WagerRecorded("r")
		m.WageringCompleted("welcome", "USD", "1")
		m.ConversionCredit("credited")
		m.PaymentEvent("awarded")
	})
}

func TestNew_TolerantToSharedRegistry(t *testing.T) {
	// Registering twice on one registry must not panic (tests may rebuild
	// Metrics against the default registry).
	reg := prometheus.NewRegistry()
	assert.NotPanics(t, func() {
		New(reg)
		New(reg)
	})
}

// ── HTTP middleware ──────────────────────────────────────────────────────────

func TestHTTPMiddleware_UsesRoutePatternNotRawPath(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	app := fiber.New()
	app.Use(HTTPMiddleware(m))
	app.Get("/api/v1/bonuses/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": c.Params("id")})
	})

	for _, id := range []string{
		"3f2a1b4c-0001-4a1b-8c1d-000000000001",
		"3f2a1b4c-0002-4a1b-8c1d-000000000002",
		"3f2a1b4c-0003-4a1b-8c1d-000000000003",
	} {
		resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/bonuses/"+id, nil), -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	out := scrape(t, reg)
	// The label must be the route pattern, so all 3 requests collapse into
	// one series instead of three.
	assert.Contains(t, out, `bonus_http_requests_total{method="GET",route="/api/v1/bonuses/:id",status="200"} 3`)
	assert.NotContains(t, out, "3f2a1b4c", "raw IDs must never appear in labels")
}

func TestHTTPMiddleware_UnmatchedRouteCollapses(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	app := fiber.New()
	app.Use(HTTPMiddleware(m))
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })

	for _, p := range []string{"/nope/1", "/nope/2", "/nope/3"} {
		resp, err := app.Test(httptest.NewRequest("GET", p, nil), -1)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	}

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_http_requests_total{method="GET",route="unmatched",status="404"} 3`)
}

func TestHTTPMiddleware_RecordsErrorStatusFromFiberError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	app := fiber.New()
	app.Use(HTTPMiddleware(m))
	app.Get("/boom", func(_ *fiber.Ctx) error { return fiber.ErrTeapot })

	_, err := app.Test(httptest.NewRequest("GET", "/boom", nil), -1)
	require.NoError(t, err)

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_http_requests_total{method="GET",route="/boom",status="418"} 1`)
}

func TestHTTPMiddleware_ObservesDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	app := fiber.New()
	app.Use(HTTPMiddleware(m))
	app.Get("/slow", func(c *fiber.Ctx) error {
		time.Sleep(5 * time.Millisecond)
		return c.SendString("ok")
	})
	_, err := app.Test(httptest.NewRequest("GET", "/slow", nil), -1)
	require.NoError(t, err)

	out := scrape(t, reg)
	assert.Contains(t, out, `bonus_http_request_duration_seconds_count{method="GET",route="/slow"} 1`)
}

func TestNormalizeRoute(t *testing.T) {
	assert.Equal(t, "unmatched", NormalizeRoute(""))
	assert.Equal(t, "unmatched", NormalizeRoute("   "))
	assert.Equal(t, "/health", NormalizeRoute("/health"))
}

// ── gRPC interceptors ────────────────────────────────────────────────────────

func newTestGRPCServer(t *testing.T, reg *prometheus.Registry) (pb.BonusServiceClient, func()) {
	t.Helper()
	lis, err := netListen()
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		UnaryRecoveryInterceptor(zap.NewNop()),
		UnaryLoggingInterceptor(zap.NewNop()),
		UnaryMetricsInterceptor(New(reg)),
	))
	// Empty bonus ID in GetBonus → InvalidArgument, which is what we assert on.
	pb.RegisterBonusServiceServer(srv, &stubBonusServer{})
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecureCreds()))
	require.NoError(t, err)
	return pb.NewBonusServiceClient(conn), func() {
		_ = conn.Close()
		srv.Stop()
	}
}

func TestUnaryMetricsInterceptor_RecordsCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	client, stop := newTestGRPCServer(t, reg)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetBonus(ctx, &pb.GetBonusRequest{
		UserId:  &commonv1.UserId{Value: "1"},
		BonusId: "",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())

	out := scrape(t, reg)
	assert.Contains(t, out, "bonus_grpc_requests_total")
	assert.Contains(t, out, `code="InvalidArgument"`)
	assert.Contains(t, out, "bonus_grpc_request_duration_seconds_count")
}

func TestUnaryRecoveryInterceptor_ConvertsPanicToInternal(t *testing.T) {
	reg := prometheus.NewRegistry()
	client, stop := newTestGRPCServer(t, reg)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.GetPromotions(ctx, &pb.GetPromotionsRequest{}) // panics in stub
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.NotContains(t, st.Message(), "boom", "internal details must not leak")
}

func TestCodeName_BoundedSet(t *testing.T) {
	assert.Equal(t, codes.OK.String(), CodeName(nil))
	assert.Equal(t, codes.NotFound.String(), CodeName(status.Error(codes.NotFound, "x")))
	assert.Equal(t, codes.Unknown.String(), CodeName(errors.New("plain")))
}

func TestShortMethod(t *testing.T) {
	assert.Equal(t, "GetBonus", ShortMethod("bonus.v1.BonusService/GetBonus"))
	assert.Equal(t, "weird", ShortMethod("weird"))
}

func TestChainUnary_AppliesInOrder(t *testing.T) {
	var order []string
	mk := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			order = append(order, name)
			return handler(ctx, req)
		}
	}
	_, err := ChainUnary(mk("outer"), mk("inner"))(
		context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/x/y"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			order = append(order, "handler")
			return nil, nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"outer", "inner", "handler"}, order)
}

// ── Promhttp wiring ──────────────────────────────────────────────────────────

func TestPromhttp_HandlerForExposesBusinessMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	m.ConversionCredit("credited")

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()
	body, err := httpGet(srv.URL)
	require.NoError(t, err)
	assert.True(t, strings.Contains(body, "bonus_conversion_credits_total"), "exposition must contain bonus metrics")
}
