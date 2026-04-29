package pragmatic_test

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/casino/internal/provider"
	"github.com/opus-casino/casino/internal/provider/pragmatic"
)

func testAdapter(t *testing.T) *pragmatic.Adapter {
	t.Helper()
	cfg := pragmatic.Config{
		AgentID:         "test_agent",
		SecretKey:       "test_secret_key",
		APIURL:          "https://test.pragmatic.invalid",
		Enabled:         true,
		ReplayWindowSec: 180,
	}
	return pragmatic.New(cfg, zap.NewNop())
}

func TestAdapter_Name(t *testing.T) {
	a := testAdapter(t)
	assert.Equal(t, "pragmatic", a.Name())
}

func TestAdapter_VerifyCallbackSignature_Valid(t *testing.T) {
	a := testAdapter(t)

	// Build a callback with correct MD5 hash
	params := map[string]interface{}{
		"amount":        "10",
		"currency":      "USD",
		"gameId":        "vs20doghouse",
		"roundId":       "round1",
		"token":         "session1",
		"transactionId": "tx1",
		"type":          "bet",
	}
	params["hash"] = computePragmaticHash(params, "test_secret_key")

	body, _ := json.Marshal(params)
	assert.True(t, a.VerifyCallbackSignature(body, nil))
}

func TestAdapter_VerifyCallbackSignature_Invalid(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"amount":        "10",
		"currency":      "USD",
		"transactionId": "tx1",
		"type":          "bet",
		"hash":          "wrong_hash_value",
	}

	body, _ := json.Marshal(params)
	assert.False(t, a.VerifyCallbackSignature(body, nil))
}

func TestAdapter_VerifyCallbackSignature_MissingHash(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"amount":        "10",
		"transactionId": "tx1",
	}

	body, _ := json.Marshal(params)
	assert.False(t, a.VerifyCallbackSignature(body, nil))
}

func TestAdapter_ParseCallback_Bet(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"type":          "bet",
		"token":         "session_abc",
		"gameId":        "vs20doghouse",
		"roundId":       "round123",
		"transactionId": "txBet001",
		"currency":      "USD",
		"amount":        "5.00",
		"timestamp":     float64(timeNowMillis()),
		"hash":          "placeholder",
	}

	body, _ := json.Marshal(params)
	event, err := a.ParseCallback(body)

	require.NoError(t, err)
	assert.Equal(t, provider.CallbackBet, event.Type)
	assert.Equal(t, "txBet001", event.TransactionID)
	assert.Equal(t, "round123", event.RoundID)
	assert.Equal(t, "session_abc", event.PlayerID)
}

func TestAdapter_ParseCallback_Win(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"type":          "result",
		"token":         "session_abc",
		"gameId":        "vs20doghouse",
		"roundId":       "round123",
		"transactionId": "txWin001",
		"currency":      "USD",
		"amount":        "15.50",
		"timestamp":     float64(timeNowMillis()),
		"hash":          "placeholder",
	}

	body, _ := json.Marshal(params)
	event, err := a.ParseCallback(body)

	require.NoError(t, err)
	assert.Equal(t, provider.CallbackWin, event.Type)
	assert.Equal(t, "15.5", event.Amount.String())
}

func TestAdapter_ParseCallback_Rollback(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"type":                    "refund",
		"token":                   "session_abc",
		"gameId":                  "vs20doghouse",
		"roundId":                 "round123",
		"transactionId":           "txRollback001",
		"referenceTransactionId":  "txBet001",
		"currency":                "USD",
		"amount":                  "5.00",
		"timestamp":               float64(timeNowMillis()),
		"hash":                    "placeholder",
	}

	body, _ := json.Marshal(params)
	event, err := a.ParseCallback(body)

	require.NoError(t, err)
	assert.Equal(t, provider.CallbackRollback, event.Type)
	assert.Equal(t, "txBet001", event.RefTransID)
}

func TestAdapter_ParseCallback_ReplayDetected(t *testing.T) {
	a := testAdapter(t)

	params := map[string]interface{}{
		"type":          "bet",
		"token":         "session_abc",
		"gameId":        "vs20doghouse",
		"roundId":       "round123",
		"transactionId": "txOld",
		"currency":      "USD",
		"amount":        "5.00",
		"timestamp":     float64(1000), // Very old timestamp (1970)
		"hash":          "placeholder",
	}

	body, _ := json.Marshal(params)
	_, err := a.ParseCallback(body)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "replay")
}

func TestAdapter_BuildLaunchURL_IsEmpty_WithoutRealAPI(t *testing.T) {
	// BuildLaunchURL calls an external API — verify it fails gracefully with invalid URL
	a := testAdapter(t)
	_, err := a.BuildLaunchURL(context.Background(), provider.LaunchRequest{
		GameID:   "vs20doghouse",
		Token:    "testtoken",
		Currency: "USD",
		Language: "en",
		LobbyURL: "https://casino.example.com",
	})
	// Expected: network error since URL is invalid in tests
	assert.Error(t, err)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func computePragmaticHash(params map[string]interface{}, secret string) string {
	keys := make([]string, 0)
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%v", params[k]))
	}
	sb.WriteString(secret)

	//nolint:gosec
	return fmt.Sprintf("%x", md5.Sum([]byte(sb.String())))
}

func timeNowMillis() int64 {
	return time.Now().UnixMilli()
}
