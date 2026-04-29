package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/opus-casino/admin-bff/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PostbackEvent is the payload sent to affiliate postback URLs.
type PostbackEvent struct {
	AffiliateID string
	Event       string // registration|ftd|deposit|redeposit
	PlayerID    string
	Amount      *string
	Currency    *string
	ClickID     *string
}

// FirePostbacks finds all matching postback configs for an affiliate event
// and dispatches them concurrently with retry backoff.
// Call this from payment/registration event handlers.
func FirePostbacks(db *gorm.DB, log *zap.Logger, evt PostbackEvent) {
	var aff models.Affiliate
	if err := db.Where("id = ?", evt.AffiliateID).First(&aff).Error; err != nil {
		return
	}

	for _, cfg := range aff.PostbackConfigs {
		if cfg.Event != evt.Event {
			continue
		}
		// Substitute template variables
		url := substituteVars(cfg.URL, evt)
		go retryPostback(db, log, aff.ID, evt, cfg, url, 1)
	}
}

func substituteVars(urlTemplate string, evt PostbackEvent) string {
	u := urlTemplate
	u = strings.ReplaceAll(u, "{player_id}", evt.PlayerID)
	u = strings.ReplaceAll(u, "{affiliate_id}", evt.AffiliateID)
	if evt.ClickID != nil {
		u = strings.ReplaceAll(u, "{click_id}", *evt.ClickID)
	}
	if evt.Amount != nil {
		u = strings.ReplaceAll(u, "{amount}", *evt.Amount)
	}
	return u
}

// retryPostback sends the postback and retries with exponential backoff (1min, 5min, 30min).
func retryPostback(db *gorm.DB, log *zap.Logger, affiliateID string, evt PostbackEvent,
	cfg models.PostbackConfig, url string, attempt int) {

	maxAttempts := cfg.RetryCount
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	success, statusCode, responseBody := sendPostback(cfg.Method, url, evt)

	entry := models.PostbackLog{
		AffiliateID: affiliateID,
		Event:       evt.Event,
		PlayerID:    evt.PlayerID,
		URL:         url,
		HTTPStatus:  statusCode,
		Response:    responseBody,
		AttemptNo:   attempt,
		SentAt:      time.Now(),
		Success:     success,
	}
	db.Create(&entry)

	if success {
		log.Info("postback sent successfully",
			zap.String("affiliate_id", affiliateID),
			zap.String("event", evt.Event),
			zap.String("url", url),
		)
		return
	}

	log.Warn("postback failed",
		zap.String("affiliate_id", affiliateID),
		zap.Int("attempt", attempt),
		zap.Int("status", statusCode),
	)

	if attempt >= maxAttempts {
		log.Error("postback exhausted retries",
			zap.String("affiliate_id", affiliateID),
			zap.String("event", evt.Event),
		)
		return
	}

	// Exponential backoff: 1min, 5min, 30min
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}
	delay := delays[int(math.Min(float64(attempt-1), float64(len(delays)-1)))]

	time.AfterFunc(delay, func() {
		retryPostback(db, log, affiliateID, evt, cfg, url, attempt+1)
	})
}

func sendPostback(method, url string, evt PostbackEvent) (success bool, statusCode int, responseBody string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var body io.Reader
	if strings.ToUpper(method) == "POST" {
		payload, _ := json.Marshal(map[string]any{
			"player_id":    evt.PlayerID,
			"affiliate_id": evt.AffiliateID,
			"event":        evt.Event,
			"amount":       evt.Amount,
			"currency":     evt.Currency,
		})
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), url, body)
	if err != nil {
		return false, 0, fmt.Sprintf("request create error: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0, fmt.Sprintf("http error: %v", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode, string(respBytes)
}
