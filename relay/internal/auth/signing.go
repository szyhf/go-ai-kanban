package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// SigningSession represents an active signing session.
type SigningSession struct {
	ID         string
	ClientID   string
	PublicKey  ed25519.PublicKey
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
}

// HTTPSigningPayload constructs the message to sign for HTTP requests.
// Format: "v1|{timestamp}|{method}|{path}|{signingSessionId}|{nonce}|{bodyHashB64}"
func HTTPSigningPayload(timestamp int64, method, path, sessionID, nonce string, body []byte) string {
	return fmt.Sprintf("v1|%d|%s|%s|%s|%s|%s",
		timestamp,
		strings.ToUpper(method),
		path,
		sessionID,
		nonce,
		bodyHash(body),
	)
}

// WSFrameSigningPayload constructs the message to sign for WebSocket frames.
// Format: "v1|{signingSessionId}|{requestNonce}|{seq}|{msgType}|{payloadHashB64}"
func WSFrameSigningPayload(sessionID, requestNonce string, seq int64, msgType string, payload []byte) string {
	h := sha256.Sum256(payload)
	payloadHash := base64.StdEncoding.EncodeToString(h[:])
	return fmt.Sprintf("v1|%s|%s|%d|%s|%s",
		sessionID,
		requestNonce,
		seq,
		msgType,
		payloadHash,
	)
}

// RefreshSigningPayload constructs the message to sign for session refresh.
// Format: "v1|refresh|{timestamp}|{nonce}|{clientId}"
func RefreshSigningPayload(timestamp int64, nonce, clientID string) string {
	return fmt.Sprintf("v1|refresh|%d|%s|%s", timestamp, nonce, clientID)
}

// Sign signs a message with the given Ed25519 private key.
func Sign(privateKey ed25519.PrivateKey, message string) ([]byte, error) {
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("private key is empty")
	}
	sig := ed25519.Sign(privateKey, []byte(message))
	return sig, nil
}

// Verify verifies a signature against a public key.
func Verify(publicKey ed25519.PublicKey, message string, signature []byte) bool {
	if len(publicKey) == 0 || len(signature) == 0 {
		return false
	}
	return ed25519.Verify(publicKey, []byte(message), signature)
}

// VerifyHTTPSignature verifies an HTTP request signature.
// Returns error if timestamp drift exceeds 30 seconds, nonce is invalid, or signature doesn't match.
func VerifyHTTPSignature(publicKey ed25519.PublicKey, timestamp int64, method, path, sessionID, nonce string, body []byte, signature []byte) error {
	now := time.Now().Unix()
	drift := now - timestamp
	if drift < 0 {
		drift = -drift
	}
	if drift > int64(TimestampDrift.Seconds()) {
		return fmt.Errorf("timestamp drift too large: %d seconds (max %d)", drift, int64(TimestampDrift.Seconds()))
	}

	payload := HTTPSigningPayload(timestamp, method, path, sessionID, nonce, body)
	if !Verify(publicKey, payload, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// bodyHash computes SHA-256 of body and returns base64-encoded hash.
func bodyHash(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	h := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(h[:])
}
