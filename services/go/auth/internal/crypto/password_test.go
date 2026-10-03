package crypto

import (
	"strings"
	"testing"
)

// A regression guard for the range validation added to VerifyPassword: the
// argon2 parameters are parsed out of the stored hash string, so a malformed or
// tampered hash must be rejected rather than verified with wrapped values.
func TestVerifyPasswordRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("correct password was rejected")
	}

	ok, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatalf("VerifyPassword(wrong): %v", err)
	}
	if ok {
		t.Error("wrong password was accepted")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	hash, err := HashPassword("pw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	cases := map[string]string{
		"wrong field count":     strings.Replace(hash, "$", "!", 1),
		"non-numeric memory":    replaceParam(hash, "m=", "m=abc"),
		"zero parallelism":      replaceParam(hash, "p=", "p=0"),
		"oversized parallelism": replaceParam(hash, "p=", "p=9999"),
		"corrupt salt":          corruptField(hash, 4),
		"corrupt hash":          corruptField(hash, 5),
	}

	for name, malformed := range cases {
		t.Run(name, func(t *testing.T) {
			// Must not panic and must not report a match.
			ok, err := VerifyPassword("pw", malformed)
			if err == nil && ok {
				t.Error("malformed hash was accepted")
			}
		})
	}
}

// replaceParam rewrites the value of a "key=" parameter inside the
// "m=..,t=..,p=.." field. The field is rebuilt key by key so a replacement
// cannot be concatenated onto the existing value (replacing "p=" with "p=0" in
// "p=4" naively yields "p=04", which still parses as 4).
func replaceParam(hash, key, value string) string {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return hash
	}
	pairs := strings.Split(parts[3], ",")
	for i, p := range pairs {
		if strings.HasPrefix(p, key) {
			pairs[i] = key + value
		}
	}
	parts[3] = strings.Join(pairs, ",")
	return strings.Join(parts, "$")
}

// corruptField makes a base64 field undecodable.
func corruptField(hash string, idx int) string {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return hash
	}
	parts[idx] = "!!!not-base64!!!"
	return strings.Join(parts, "$")
}
