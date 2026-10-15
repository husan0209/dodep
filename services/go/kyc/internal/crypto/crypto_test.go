package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncrypt_Decrypt_RoundTrip(t *testing.T) {
	t.Parallel()
	svc, err := Ephemeral()
	require.NoError(t, err)

	ct, err := svc.Encrypt("AB1234567")
	require.NoError(t, err)
	assert.NotContains(t, ct, "AB1234567")
	assert.True(t, strings.HasPrefix(ct, "v1:"))

	open, err := svc.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "AB1234567", open)
}

func TestEncrypt_NoncesDiffer(t *testing.T) {
	t.Parallel()
	svc, err := Ephemeral()
	require.NoError(t, err)

	a, _ := svc.Encrypt("same")
	b, _ := svc.Encrypt("same")
	assert.NotEqual(t, a, b, "random nonce must make ciphertexts differ")
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	t.Parallel()
	a, _ := Ephemeral()
	b, _ := Ephemeral()

	ct, err := a.Encrypt("secret")
	require.NoError(t, err)
	_, err = b.Decrypt(ct)
	assert.Error(t, err)
}

func TestParseKey_Encodings(t *testing.T) {
	t.Parallel()
	raw := "0123456789abcdef0123456789abcdef" // 32 bytes
	key, err := ParseKey(raw)
	require.NoError(t, err)
	require.Len(t, key, 32)

	svc, err := New(key)
	require.NoError(t, err)
	ct, err := svc.Encrypt("x")
	require.NoError(t, err)
	open, err := svc.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "x", open)

	_, err = ParseKey("too-short")
	assert.Error(t, err)
	_, err = ParseKey("")
	assert.ErrorIs(t, err, ErrNoKey)
}

func TestHashForSearch_StableAndNormalised(t *testing.T) {
	t.Parallel()
	assert.Equal(t, HashForSearch("ab123"), HashForSearch(" AB123 "))
	assert.NotEqual(t, HashForSearch("ab123"), HashForSearch("ab124"))
}

func TestMaskNumber(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "AB***4567", MaskNumber("AB1234567"))
	assert.Equal(t, "***", MaskNumber("123"))
}
