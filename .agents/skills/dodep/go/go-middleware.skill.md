# SKILL #21 — go-middleware.skill.md

```markdown
# go-middleware.skill.md
# GAMBLING PLATFORM — GO MIDDLEWARE PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Fiber middleware: functions that wrap handlers.
Execute BEFORE and/or AFTER handler logic.
Applied globally or per route group.

Order matters: first applied = outermost layer.

# ============================================================
# SECTION 2: MIDDLEWARE STACK ORDER
# ============================================================

```text
REQUEST FLOW (top to bottom):
  1. Recover       — catch panics, return 500
  2. RequestID     — generate/propagate X-Request-ID
  3. Logger        — log request start/end with duration
  4. CORS          — handle preflight, set headers
  5. OTel Tracing  — OpenTelemetry span creation
  6. RateLimit     — per-IP or per-user throttling
  7. Auth          — validate JWT, set user in context
  8. HANDLER       — actual business logic
============================================================
SECTION 3: RECOVERY MIDDLEWARE
============================================================
Go

func Recover() fiber.Handler {
    return func(c *fiber.Ctx) error {
        defer func() {
            if r := recover(); r != nil {
                log.Error().
                    Interface("panic", r).
                    Str("request_id", c.Locals("request_id").(string)).
                    Str("path", c.Path()).
                    Str("method", c.Method()).
                    Msg("Panic recovered")

                // Report to Sentry
                if hub := sentry.GetHubFromContext(c.Context()); hub != nil {
                    hub.RecoverWithContext(c.Context(), r)
                }

                _ = respondError(c, fiber.StatusInternalServerError,
                    "INTERNAL_ERROR", "An internal error occurred")
            }
        }()
        return c.Next()
    }
}
============================================================
SECTION 4: REQUEST ID MIDDLEWARE
============================================================
Go

func RequestID() fiber.Handler {
    return func(c *fiber.Ctx) error {
        id := c.Get("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        c.Set("X-Request-ID", id)
        c.Locals("request_id", id)
        return c.Next()
    }
}
============================================================
SECTION 5: LOGGER MIDDLEWARE
============================================================
Go

func Logger() fiber.Handler {
    return func(c *fiber.Ctx) error {
        start := time.Now()

        // Process request
        err := c.Next()

        duration := time.Since(start)
        status := c.Response().StatusCode()

        logger := log.With().
            Str("request_id", getLocal[string](c, "request_id")).
            Str("method", c.Method()).
            Str("path", c.Path()).
            Int("status", status).
            Dur("duration", duration).
            Str("ip", c.IP()).
            Str("user_agent", c.Get("User-Agent")).
            Logger()

        // Add user_id if authenticated
        if userID, ok := c.Locals("user_id").(int64); ok {
            logger = logger.With().Int64("user_id", userID).Logger()
        }

        switch {
        case status >= 500:
            logger.Error().Msg("Request failed")
        case status >= 400:
            logger.Warn().Msg("Request error")
        default:
            logger.Info().Msg("Request completed")
        }

        return err
    }
}

func getLocal[T any](c *fiber.Ctx, key string) T {
    if val, ok := c.Locals(key).(T); ok {
        return val
    }
    var zero T
    return zero
}
============================================================
SECTION 6: AUTH MIDDLEWARE
============================================================
Go

func RequireAuth(tokenSvc *service.TokenService) fiber.Handler {
    return func(c *fiber.Ctx) error {
        auth := c.Get("Authorization")
        if auth == "" {
            return respondError(c, 401, "AUTH_UNAUTHORIZED", "Authorization header required")
        }

        if !strings.HasPrefix(auth, "Bearer ") {
            return respondError(c, 401, "AUTH_UNAUTHORIZED", "Bearer token required")
        }

        token := strings.TrimPrefix(auth, "Bearer ")

        claims, err := tokenSvc.ValidateAccessToken(token)
        if err != nil {
            return respondError(c, 401, "AUTH_TOKEN_INVALID", "Invalid or expired token")
        }

        // Set in context for handlers
        c.Locals("user", claims)
        c.Locals("user_id", claims.UserID)
        c.Locals("roles", claims.Roles)

        return c.Next()
    }
}

// Role-based access control
func RequireRole(roles ...string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        userRoles, ok := c.Locals("roles").([]string)
        if !ok {
            return respondError(c, 401, "AUTH_UNAUTHORIZED", "Not authenticated")
        }

        for _, required := range roles {
            for _, userRole := range userRoles {
                if userRole == required {
                    return c.Next()
                }
            }
        }

        return respondError(c, 403, "AUTH_FORBIDDEN", "Insufficient permissions")
    }
}

// Permission-based check
func RequirePermission(permission string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := c.Locals("user").(*domain.TokenClaims)
        if !ok {
            return respondError(c, 401, "AUTH_UNAUTHORIZED", "Not authenticated")
        }

        for _, p := range claims.Permissions {
            if p == permission || p == "all" {
                return c.Next()
            }
        }

        return respondError(c, 403, "AUTH_FORBIDDEN",
            fmt.Sprintf("Permission '%s' required", permission))
    }
}
============================================================
SECTION 7: RATE LIMITING MIDDLEWARE
============================================================
Go

import "github.com/gofiber/fiber/v2/middleware/limiter"

// Per-IP rate limiter for public endpoints
func RateLimitByIP(max int, window time.Duration) fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        max,
        Expiration: window,
        KeyGenerator: func(c *fiber.Ctx) string {
            return "rl:ip:" + c.IP()
        },
        LimitReached: func(c *fiber.Ctx) error {
            c.Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
            return respondError(c, 429, "RATE_LIMITED", "Too many requests")
        },
    })
}

// Per-user rate limiter for authenticated endpoints
func RateLimitByUser(max int, window time.Duration) fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        max,
        Expiration: window,
        KeyGenerator: func(c *fiber.Ctx) string {
            if userID, ok := c.Locals("user_id").(int64); ok {
                return fmt.Sprintf("rl:user:%d", userID)
            }
            return "rl:ip:" + c.IP() // fallback
        },
        LimitReached: func(c *fiber.Ctx) error {
            c.Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
            return respondError(c, 429, "RATE_LIMITED", "Too many requests")
        },
    })
}
============================================================
SECTION 8: ROUTER WITH MIDDLEWARE
============================================================
Go

func SetupRoutes(app *fiber.App, h *Handler, tokenSvc *service.TokenService) {
    // Global middleware (applies to ALL routes)
    app.Use(Recover())
    app.Use(RequestID())
    app.Use(Logger())
    app.Use(otelfiber.Middleware()) // OpenTelemetry

    // Health (no auth, no rate limit)
    app.Get("/healthz", h.Liveness)
    app.Get("/readyz", h.Readiness)
    app.Get("/metrics", h.Metrics)

    v1 := app.Group("/api/v1")

    // Public routes (rate limited by IP)
    auth := v1.Group("/auth")
    auth.Post("/register", RateLimitByIP(5, time.Hour), h.Register)
    auth.Post("/login", RateLimitByIP(10, time.Minute), h.Login)
    auth.Post("/refresh", RateLimitByIP(30, time.Minute), h.RefreshTokens)
    auth.Post("/forgot-password", RateLimitByIP(3, time.Hour), h.ForgotPassword)

    // Authenticated routes
    protected := v1.Group("", RequireAuth(tokenSvc))
    protected.Post("/auth/logout", h.Logout)
    protected.Get("/users/me", h.GetProfile)
    protected.Patch("/users/me", h.UpdateProfile)

    // Admin routes (auth + role)
    admin := v1.Group("/admin",
        RequireAuth(tokenSvc),
        RequireRole("admin", "super_admin"),
    )
    admin.Get("/users", h.AdminListUsers)
    admin.Get("/users/:id", h.AdminGetUser)
    admin.Post("/users/:id/block", RequirePermission("user.block"), h.AdminBlockUser)
}
============================================================
SECTION 9: CORS MIDDLEWARE
============================================================
Go

import "github.com/gofiber/fiber/v2/middleware/cors"

func CORSConfig() fiber.Handler {
    return cors.New(cors.Config{
        AllowOrigins:     "https://web.platform.com,https://admin.platform.com",
        AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
        AllowHeaders:     "Authorization,Content-Type,X-Request-ID,Accept-Language",
        AllowCredentials: true,
        MaxAge:           3600,
    })
}
============================================================
SECTION 10: ANTI-PATTERNS
============================================================
text

❌ NEVER put business logic in middleware (auth check OK, business rules NOT OK)
❌ NEVER skip Recover middleware (unhandled panic = crash)
❌ NEVER use c.Locals with unchecked type assertions
   ✅ Use helper: getLocal[int64](c, "user_id")
❌ NEVER hardcode rate limits — use configuration
❌ NEVER apply auth middleware to health/metrics endpoints
❌ NEVER log request body in production (may contain passwords)
❌ NEVER trust X-Forwarded-For without proxy configuration
   ✅ Configure app.Config.ProxyHeader and TrustedProxies
❌ NEVER skip CORS for API endpoints (browser security)
❌ NEVER apply rate limiting after auth (unauthenticated abuse unblocked)
❌ NEVER forget middleware order matters (recovery must be outermost)
============================================================
SECTION 11: TESTING MIDDLEWARE
============================================================
Go

func TestRequireAuth_ValidToken(t *testing.T) {
    app := fiber.New()
    tokenSvc := newTestTokenService()

    app.Use(RequireAuth(tokenSvc))
    app.Get("/test", func(c *fiber.Ctx) error {
        userID := c.Locals("user_id").(int64)
        return c.JSON(fiber.Map{"user_id": userID})
    })

    token := generateTestToken(tokenSvc, 123)
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    resp, _ := app.Test(req)
    assert.Equal(t, 200, resp.StatusCode)
}

func TestRequireAuth_MissingToken(t *testing.T) {
    app := fiber.New()
    app.Use(RequireAuth(newTestTokenService()))
    app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

    req := httptest.NewRequest("GET", "/test", nil)
    resp, _ := app.Test(req)
    assert.Equal(t, 401, resp.StatusCode)
}

func TestRequireRole_Forbidden(t *testing.T) {
    app := fiber.New()
    tokenSvc := newTestTokenService()
    app.Use(RequireAuth(tokenSvc))
    app.Use(RequireRole("admin"))
    app.Get("/admin", func(c *fiber.Ctx) error { return c.SendStatus(200) })

    token := generateTestTokenWithRoles(tokenSvc, 123, []string{"player"})
    req := httptest.NewRequest("GET", "/admin", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    resp, _ := app.Test(req)
    assert.Equal(t, 403, resp.StatusCode)
}