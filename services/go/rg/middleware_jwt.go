package main

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// base64RawURLDecode decodes unpadded base64url (JWT segments).
func base64RawURLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// decodeClaims parses the JWT payload and enforces expiry.
func decodeClaims(payloadSegment string) (jwtClaims, error) {
	var claims jwtClaims
	raw, err := base64.RawURLEncoding.DecodeString(payloadSegment)
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return claims, err
	}
	if claims.Exp != 0 && time.Now().Unix() > claims.Exp {
		return claims, errBadToken
	}
	return claims, nil
}
