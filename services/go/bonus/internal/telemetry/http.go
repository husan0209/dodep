package telemetry

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HTTPMiddleware instruments every Fiber request (RED metrics).
//
// Cardinality safety: the `route` label always holds the *route pattern*
// registered by the router (e.g. `/api/v1/bonuses/:id`), never the raw path,
// so a million distinct bonus IDs cannot create a million time series.
// Requests that match no route collapse into the single `unmatched` bucket.
//
// Place this middleware as early as possible (before auth) so rejected
// requests are observed too.
func HTTPMiddleware(m *Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		route := routePattern(c)
		status := c.Response().StatusCode()
		if err != nil {
			// Fiber returns errors from c.Next(); map them the same way the
			// global error handler would, without consuming the error.
			if fe, ok := err.(*fiber.Error); ok {
				status = fe.Code
			} else {
				status = fiber.StatusInternalServerError
			}
		}
		m.ObserveHTTP(c.Method(), route, status, time.Since(start))
		return err
	}
}

// routePattern resolves the matched route pattern for labelling.
// Falls back to "unmatched" so 404 storms cannot inflate cardinality.
func routePattern(c *fiber.Ctx) string {
	route := c.Route()
	if route == nil {
		return "unmatched"
	}
	pattern := route.Path
	if pattern == "" {
		return "unmatched"
	}
	// Fiber reports "/" for requests that did not match any route; anything
	// else is a real registered route.
	if pattern == "/" && c.Path() != "/" {
		return "unmatched"
	}
	return pattern
}

// StatusCode renders a Fiber status as a string (helper for tests/logs).
func StatusCode(code int) string { return strconv.Itoa(code) }

// NormalizeRoute is exported for tests and for any non-Fiber instrumentation
// that needs the same route-pattern discipline.
func NormalizeRoute(pattern string) string {
	if strings.TrimSpace(pattern) == "" {
		return "unmatched"
	}
	return pattern
}
