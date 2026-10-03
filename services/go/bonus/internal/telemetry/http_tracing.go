package telemetry

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// RequestIDHeader is the platform-wide request correlation header.
const RequestIDHeader = "X-Request-ID"

// HTTP span attribute keys (semantic conventions for HTTP spans).
const (
	attrHTTPMethod     = attribute.Key("http.request.method")
	attrHTTPResponseSt = attribute.Key("http.response.status_code")
	attrHTTPRoute      = attribute.Key("http.route")
	attrURLPath        = attribute.Key("url.path")
	attrUserAgent      = attribute.Key("user_agent.original")
)

// TracingMiddleware creates one SERVER span per HTTP request and extracts the
// upstream W3C trace context (so a trace started at the edge continues here —
// never start a new trace mid-flow).
//
// Fiber resolves the matched route only AFTER the handler chain runs, so the
// span is created with a provisional name and renamed to
// "<METHOD> <route pattern>" once the route is known. That keeps Jaeger
// aggregated per endpoint instead of one span per URL.
func TracingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		carrier := propagation.HeaderCarrier{}
		c.Request().Header.VisitAll(func(k, v []byte) {
			carrier[string(k)] = append(carrier[string(k)], string(v))
		})
		ctx := otel.GetTextMapPropagator().Extract(c.UserContext(), carrier)

		ctx, span := StartServerSpan(ctx, c.Method(),
			attrHTTPMethod.String(c.Method()),
			attrURLPath.String(c.Path()),
			attrUserAgent.String(c.Get("User-Agent")),
		)
		defer span.End()

		// Publish the span context so handlers (and logs) can reference the
		// same trace.
		c.SetUserContext(ctx)

		err := c.Next()

		route := routePattern(c)
		status := c.Response().StatusCode()
		if err != nil {
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			} else {
				status = fiber.StatusInternalServerError
			}
		}
		span.SetName(c.Method() + " " + route)
		span.SetAttributes(
			attrHTTPRoute.String(route),
			attrHTTPResponseSt.Int(status),
		)
		EndSpan(span, err)
		return err
	}
}

// RequestIDMiddleware implements the platform logging standard: every request
// carries a request_id, it is either taken from the incoming X-Request-ID or
// generated, echoed in the response, and exposed via Locals for handlers and
// logs. Correlation with traces comes free via the traceparent header.
func RequestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := strings.TrimSpace(c.Get(RequestIDHeader))
		if reqID == "" || len(reqID) > 128 {
			// Generate when missing, and sanitise unbounded client input:
			// the value is echoed back and written to logs.
			reqID = uuid.NewString()
		}
		c.Locals("request_id", reqID)
		c.Set(RequestIDHeader, reqID)
		// Propagate whatever the downstream chain returns: swallowing it here
		// would bypass Fiber's error handler and leave the response unbuilt.
		return c.Next()
	}
}

// RequestID reads the request id set by RequestIDMiddleware.
func RequestID(c *fiber.Ctx) string {
	v, _ := c.Locals("request_id").(string)
	return v
}

// CurrentTraceID returns the trace id for the given Fiber context, or "".
func CurrentTraceID(c *fiber.Ctx) string {
	sc := oteltrace.SpanContextFromContext(c.UserContext())
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

// LogFieldsFor returns zap fields for request correlation, so every log line
// can carry request_id and trace_id as required by the logging standard.
// Exported so the handler layer does not duplicate key names.
func LogFieldsFor(c *fiber.Ctx) []zap.Field {
	fields := []zap.Field{zap.String("request_id", RequestID(c))}
	if tid := CurrentTraceID(c); tid != "" {
		fields = append(fields, zap.String("trace_id", tid))
	}
	return fields
}
