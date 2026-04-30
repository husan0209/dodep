# SKILL #16 — go-fiber-handlers.skill.md

```markdown
# go-fiber-handlers.skill.md
# GAMBLING PLATFORM — GO FIBER HANDLER PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

All Go HTTP services use Fiber v2 (Express-inspired, fasthttp-based).
Handlers are the THINNEST layer.
Same principle as Rust handlers: extract → validate → call service → respond.

# ============================================================
# SECTION 2: HANDLER RULES
# ============================================================

```text
1. Handler takes *fiber.Ctx, returns error
2. Handler body: 5-15 lines maximum
3. Handler parses request (BodyParser, Params, Query)
4. Handler validates input FORMAT (go-playground/validator)
5. Handler calls service method (NEVER repository)
6. Handler maps service errors to HTTP via mapError()
7. Handler uses respondSuccess() / respondError() helpers
8. All handlers are methods on a Handler struct
9. Handler NEVER contains business logic
10. Handler NEVER imports database packages
============================================================
SECTION 3: HANDLER STRUCT AND CONSTRUCTOR
============================================================
Go

package handler

import (
    "github.com/gofiber/fiber/v2"
    "github.com/go-playground/validator/v10"
    
    "github.com/platform/services/payment-service/internal/service"
)

type Handler struct {
    paymentSvc *service.PaymentService
    validate   *validator.Validate
}

func New(paymentSvc *service.PaymentService) *Handler {
    v := validator.New()
    // Register custom validators
    v.RegisterValidation("currency", validateCurrency)
    v.RegisterValidation("payment_method", validatePaymentMethod)
    
    return &Handler{
        paymentSvc: paymentSvc,
        validate:   v,
    }
}
============================================================
SECTION 4: REQUEST/RESPONSE DTOs
============================================================
Go

// ── Request DTOs ──
// Use struct tags for parsing + validation

type DepositRequest struct {
    Amount   string `json:"amount" validate:"required,numeric,gt=0"`
    Currency string `json:"currency" validate:"required,len=3,currency"`
    Method   string `json:"method" validate:"required,payment_method"`
    ReturnURL string `json:"return_url" validate:"required,url"`
}

type WithdrawalRequest struct {
    Amount      string `json:"amount" validate:"required,numeric,gt=0"`
    Currency    string `json:"currency" validate:"required,len=3,currency"`
    Method      string `json:"method" validate:"required,payment_method"`
    Destination string `json:"destination" validate:"required"`
}

// ── Query params ──
type TransactionHistoryQuery struct {
    PageSize int    `query:"page_size" validate:"omitempty,min=1,max=100"`
    Cursor   string `query:"cursor" validate:"omitempty"`
    Type     string `query:"type" validate:"omitempty,oneof=deposit withdrawal"`
    Status   string `query:"status" validate:"omitempty,oneof=pending completed failed"`
}

// ── Response DTOs ──
type DepositResponse struct {
    PaymentID   string `json:"payment_id"`
    RedirectURL string `json:"redirect_url"`
    Status      string `json:"status"`
    ExpiresAt   string `json:"expires_at"`
}

type TransactionResponse struct {
    ID        string `json:"id"`
    Type      string `json:"type"`
    Amount    string `json:"amount"`
    Currency  string `json:"currency"`
    Status    string `json:"status"`
    Method    string `json:"method"`
    CreatedAt string `json:"created_at"`
}
============================================================
SECTION 5: HANDLER IMPLEMENTATIONS
============================================================
Go

// ── POST /api/v1/payments/deposit ──
func (h *Handler) Deposit(c *fiber.Ctx) error {
    var req DepositRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_BODY", "Invalid request body")
    }
    if err := h.validate.Struct(req); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    userID := c.Locals("user_id").(int64)

    result, err := h.paymentSvc.InitiateDeposit(c.Context(), service.DepositInput{
        UserID:    userID,
        Amount:    req.Amount,
        Currency:  req.Currency,
        Method:    req.Method,
        ReturnURL: req.ReturnURL,
        IPAddress: c.IP(),
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusCreated, DepositResponse{
        PaymentID:   result.PaymentID,
        RedirectURL: result.RedirectURL,
        Status:      result.Status,
        ExpiresAt:   result.ExpiresAt.Format(time.RFC3339),
    })
}

// ── POST /api/v1/payments/withdraw ──
func (h *Handler) Withdraw(c *fiber.Ctx) error {
    var req WithdrawalRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_BODY", "Invalid request body")
    }
    if err := h.validate.Struct(req); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    userID := c.Locals("user_id").(int64)

    result, err := h.paymentSvc.InitiateWithdrawal(c.Context(), service.WithdrawalInput{
        UserID:      userID,
        Amount:      req.Amount,
        Currency:    req.Currency,
        Method:      req.Method,
        Destination: req.Destination,
        IPAddress:   c.IP(),
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusCreated, result)
}

// ── GET /api/v1/payments/history ──
func (h *Handler) GetHistory(c *fiber.Ctx) error {
    var params TransactionHistoryQuery
    if err := c.QueryParser(&params); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_PARAMS", "Invalid query parameters")
    }
    if err := h.validate.Struct(params); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    if params.PageSize == 0 {
        params.PageSize = 20
    }

    userID := c.Locals("user_id").(int64)

    result, err := h.paymentSvc.GetHistory(c.Context(), userID, service.HistoryFilter{
        PageSize: params.PageSize,
        Cursor:   params.Cursor,
        Type:     params.Type,
        Status:   params.Status,
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondPaginated(c, fiber.StatusOK, result.Items, result.Cursor, result.HasMore)
}

// ── GET /api/v1/payments/methods ──
func (h *Handler) GetMethods(c *fiber.Ctx) error {
    userID := c.Locals("user_id").(int64)

    methods, err := h.paymentSvc.GetAvailableMethods(c.Context(), userID)
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusOK, methods)
}

// ── POST /api/v1/payments/webhook/:provider ──
func (h *Handler) Webhook(c *fiber.Ctx) error {
    provider := c.Params("provider")
    body := c.Body()
    signature := c.Get("X-Webhook-Signature")

    err := h.paymentSvc.ProcessWebhook(c.Context(), provider, body, signature)
    if err != nil {
        return h.mapError(c, err)
    }

    return c.SendStatus(fiber.StatusOK)
}
============================================================
SECTION 6: ROUTER SETUP
============================================================
Go

func SetupRoutes(app *fiber.App, h *Handler, authMiddleware fiber.Handler) {
    // Health (no auth)
    app.Get("/healthz", h.Liveness)
    app.Get("/readyz", h.Readiness)
    app.Get("/metrics", h.Metrics)

    v1 := app.Group("/api/v1")

    // Payment routes (authenticated)
    payments := v1.Group("/payments", authMiddleware)
    payments.Post("/deposit", h.Deposit)
    payments.Post("/withdraw", h.Withdraw)
    payments.Get("/history", h.GetHistory)
    payments.Get("/methods", h.GetMethods)

    // Webhook routes (no user auth, PSP signature verification in handler)
    webhooks := v1.Group("/payments/webhooks")
    webhooks.Post("/:provider", h.Webhook)
}
============================================================
SECTION 7: CUSTOM VALIDATORS
============================================================
Go

func validateCurrency(fl validator.FieldLevel) bool {
    valid := map[string]bool{
        "USD": true, "EUR": true, "GBP": true, "BTC": true,
        "ETH": true, "USDT": true, "BRL": true, "INR": true,
    }
    return valid[fl.Field().String()]
}

func validatePaymentMethod(fl validator.FieldLevel) bool {
    valid := map[string]bool{
        "card": true, "bank_transfer": true, "pix": true,
        "crypto_btc": true, "crypto_eth": true, "crypto_usdt": true,
        "skrill": true, "neteller": true,
    }
    return valid[fl.Field().String()]
}
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
Go

// ❌ BAD: Business logic in handler
func (h *Handler) Deposit(c *fiber.Ctx) error {
    // ... parse request ...
    user, _ := h.userRepo.GetByID(c.Context(), userID) // ❌ calls repo directly
    if user.KYCLevel < 2 {                              // ❌ business rule in handler
        return respondError(c, 403, "KYC_REQUIRED", "...")
    }
    balance, _ := h.walletRepo.GetBalance(...)          // ❌ calls repo
    // ... 50 more lines of logic ...
}

// ✅ GOOD: Thin handler delegates to service
func (h *Handler) Deposit(c *fiber.Ctx) error {
    var req DepositRequest
    if err := c.BodyParser(&req); err != nil { return respondError(...) }
    if err := h.validate.Struct(req); err != nil { return respondValidationError(...) }
    result, err := h.paymentSvc.InitiateDeposit(c.Context(), /* input */)
    if err != nil { return h.mapError(c, err) }
    return respondSuccess(c, 201, result)
}

// ❌ BAD: Not using standard response format
func (h *Handler) GetMethods(c *fiber.Ctx) error {
    return c.JSON(methods) // ❌ no envelope, no meta
}

// ✅ GOOD: Using respondSuccess helper
func (h *Handler) GetMethods(c *fiber.Ctx) error {
    return respondSuccess(c, 200, methods) // wraps in { "data": ..., "meta": ... }
}

// ❌ BAD: Ignoring context
func (h *Handler) Deposit(c *fiber.Ctx) error {
    result, err := h.paymentSvc.Deposit(context.Background(), ...) // ❌ loses request context
}

// ✅ GOOD: Propagating context
func (h *Handler) Deposit(c *fiber.Ctx) error {
    result, err := h.paymentSvc.Deposit(c.Context(), ...) // ✅ propagates trace, deadline
}