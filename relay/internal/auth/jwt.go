package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager handles JWT creation and validation.
type JWTManager struct {
	secret []byte
}

// NewJWTManager creates a new JWTManager with the given HMAC secret.
// If secret is nil or empty, a random 32-byte secret is generated.
func NewJWTManager(secret []byte) *JWTManager {
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			// Fall back to a time-based seed if crypto/rand fails.
			// This should never happen in practice.
			panic(fmt.Sprintf("failed to generate random secret: %v", err))
		}
	}
	return &JWTManager{secret: secret}
}

// Claims represents JWT claims for the relay server.
type Claims struct {
	ClientID string `json:"client_id,omitempty"`
	HostID   string `json:"host_id,omitempty"`
	Role     string `json:"role"` // "client" or "host"
	jwt.RegisteredClaims
}

// CreateToken creates a signed JWT for a client or host.
// role is "client" or "host", ttl is the token lifetime.
func (m *JWTManager) CreateToken(clientID, hostID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		ClientID: clientID,
		HostID:   hostID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{"access"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateToken validates a JWT and returns its claims.
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
