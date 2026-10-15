package screening

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opus-casino/kyc/internal/domain"
)

func TestStubProvider_Outcomes(t *testing.T) {
	t.Parallel()
	p := StubProvider{}
	assert.Equal(t, "complyadvantage-stub", p.Name())

	clear, err := p.Screen(context.Background(), SearchRequest{FullName: "John Smith"})
	require.NoError(t, err)
	assert.Equal(t, domain.ScreeningClear, clear.Status)

	pep, err := p.Screen(context.Background(), SearchRequest{FullName: "Robert PEP Johnson"})
	require.NoError(t, err)
	assert.Equal(t, domain.ScreeningPotentialMatch, pep.Status)
	assert.Equal(t, 1, pep.Matches)

	sanctions, err := p.Screen(context.Background(), SearchRequest{FullName: "Sanctioned Person"})
	require.NoError(t, err)
	assert.Equal(t, domain.ScreeningTrueMatch, sanctions.Status)
}

func TestSelectProvider(t *testing.T) {
	t.Parallel()
	assert.IsType(t, StubProvider{}, SelectProvider("", ""))
	prod := SelectProvider("https://api.example.com", "key")
	assert.IsType(t, &ComplyAdvantageClient{}, prod)
	assert.True(t, prod.(*ComplyAdvantageClient).Configured())

	_, err := prod.Screen(context.Background(), SearchRequest{FullName: "x"})
	require.Error(t, err, "unconfigured production provider fails closed")
}

func TestVerifyWebhookSignature(t *testing.T) {
	t.Parallel()
	body := []byte(`{"a":1}`)
	sig := SignForTest(body, "secret")
	assert.True(t, VerifyWebhookSignature(body, sig, "secret"))
	assert.True(t, VerifyWebhookSignature(body, "sha256="+sig, "secret"))
	assert.False(t, VerifyWebhookSignature(body, sig, "wrong"))
	assert.False(t, VerifyWebhookSignature(body, "bad", "secret"))
	assert.False(t, VerifyWebhookSignature(body, sig, ""))

	other := SignForTest([]byte(`{"a":2}`), "secret")
	assert.False(t, VerifyWebhookSignature(body, other, "secret"), "tampered body must fail")
}
