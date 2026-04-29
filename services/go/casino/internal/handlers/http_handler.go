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

// GetGames GET /api/v1/casino/games
func (h *CasinoHTTPHandler) GetGames(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	providerID := c.Query("provider")
	category := c.Query("category")
	search := c.Query("search")

	opts := service.GetGamesOptions{
		Limit:  int32(limit),
		Offset: int32(offset),
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

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	result, err := h.svc.GetGameHistory(c.Context(), service.GetGameHistoryOptions{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"sessions": result.Sessions,
		"total":    result.TotalCount,
	})
}
