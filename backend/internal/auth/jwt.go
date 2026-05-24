package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload for access tokens.
type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"usr"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs a short-lived JWT containing the user id + username.
func IssueAccessToken(secret, userID, username string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("jwt secret is required")
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseAccessToken verifies signature + expiration and returns the claims.
func ParseAccessToken(secret, tokenStr string) (*Claims, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	if !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// GenerateRefreshToken returns (raw, hash). The raw value is returned to the
// client once; the hash is stored server-side so a DB leak cannot replay.
func GenerateRefreshToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw := hex.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken produces the server-side stored representation.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
