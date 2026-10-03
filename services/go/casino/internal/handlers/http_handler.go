package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/service"
)

// CasinoHTTPHandler exposes casino REST endpoints.
type CasinoHTTPHandler struct {
	svc *service.CasinoService
	log *zap.Logger
}

// NewCasinoHTTPHandler creates a new HTTP handler.
func NewCasinoHTTPHandler(svc *service.CasinoService, log *zap.Logger) *CasinoHTTPHandler {
	return &CasinoHTTPHandler{svc: svc, log: log}
}

// Pagination bounds enforced on every list endpoint. The service layer speaks
// int32, so an unvalidated strconv.Atoi result would silently wrap: a client
// asking for ?limit=99999999999 would otherwise turn into a negative limit.
const (
	defaultPageLimit int32 = 50
	maxPageLimit     int32 = 200
	maxPageOffset    int32 = 1_000_000
)

// parseBoundedInt32 reads a query parameter as an int32 clamped to [min, max].
// Anything unparseable or out of range falls back to def.
func parseBoundedInt32(raw string, def, min, max int32) int32 {
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return def
	}
	switch {
	case v < int64(min):
		return min
	case v > int64(max):
		return max
	default:
		return int32(v)
	}
}

func pageLimit(c *fiber.Ctx, def int32) int32 {
	return parseBoundedInt32(c.Query("limit"), def, 1, maxPageLimit)
}

func pageOffset(c *fiber.Ctx) int32 {
	return parseBoundedInt32(c.Query("offset"), 0, 0, maxPageOffset)
}

// GetGames GET /api/v1/casino/games
func (h *CasinoHTTPHandler) GetGames(c *fiber.Ctx) error {
	limit := pageLimit(c, defaultPageLimit)
	offset := pageOffset(c)
	providerID := c.Query("provider")
	category := c.Query("category")
	search := c.Query("search")

	opts := service.GetGamesOptions{
		Limit:  limit,
		Offset: offset,
	}
	if providerID != "" {
		opts.ProviderID = &providerID
	}
	if category != "" {
		opts.Category = &category
	}
	if search != "" {
		opts.Search = &search
	}

	result, err := h.svc.GetGames(c.Context(), opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"games": result.Games,
		"total": result.TotalCount,
	})
}

// GetGame GET /api/v1/casino/games/:id
func (h *CasinoHTTPHandler) GetGame(c *fiber.Ctx) error {
	gameID := c.Params("id")
	game, err := h.svc.GetGame(c.Context(), gameID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "game not found"})
	}
	return c.JSON(game)
}

// LaunchGame POST /api/v1/casino/games/launch
func (h *CasinoHTTPHandler) LaunchGame(c *fiber.Ctx) error {
	type req struct {
		GameID     string `json:"game_id"`
		DeviceType string `json:"device_type"`
		LobbyURL   string `json:"lobby_url"`
	}

	var body req
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// user_id comes from JWT middleware (set as local)
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	lobbyURL := body.LobbyURL
	if lobbyURL == "" {
		lobbyURL = c.Get("Referer", "/casino")
	}

	result, err := h.svc.LaunchGame(c.Context(), &service.LaunchGameRequest{
		UserID:     userID,
		GameID:     body.GameID,
		DeviceType: body.DeviceType,
		LobbyURL:   lobbyURL,
	})
	if err != nil {
		h.log.Warn("LaunchGame failed", zap.Error(err), zap.String("game_id", body.GameID))
		return c.Status(422).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"launch_url": result.LaunchURL,
		"session_id": result.Session.ID,
		"token":      result.Token,
	})
}

// GetProviders GET /api/v1/casino/providers
func (h *CasinoHTTPHandler) GetProviders(c *fiber.Ctx) error {
	active := true
	providers, err := h.svc.GetProviders(c.Context(), &active)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"providers": providers})
}

// GetSession GET /api/v1/casino/sessions/:id
func (h *CasinoHTTPHandler) GetSession(c *fiber.Ctx) error {
	session, err := h.svc.GetGameSession(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "session not found"})
	}
	return c.JSON(session)
}

// EndSession POST /api/v1/casino/sessions/:id/end
func (h *CasinoHTTPHandler) EndSession(c *fiber.Ctx) error {
	result, err := h.svc.EndGameSession(c.Context(), &service.EndGameSessionRequest{
		SessionID: c.Params("id"),
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// GetHistory GET /api/v1/casino/history
func (h *CasinoHTTPHandler) GetHistory(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	limit := pageLimit(c, 20)
	offset := pageOffset(c)

	result, err := h.svc.GetGameHistory(c.Context(), service.GetGameHistoryOptions{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"sessions": result.Sessions,
		"total":    result.TotalCount,
	})
}
