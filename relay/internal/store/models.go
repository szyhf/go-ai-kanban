package store

import "time"

// ServerKey stores the relay server's Ed25519 key pair.
type ServerKey struct {
	ID         int64
	PublicKey  []byte
	PrivateKey []byte
	CreatedAt  time.Time
}

// EnrollmentCode is a one-time pairing code.
type EnrollmentCode struct {
	Code      string
	CreatedAt time.Time
	UsedAt    *time.Time
}

// Client represents a paired browser client.
type Client struct {
	ID          string
	PublicKey   []byte
	DisplayName string
	DeviceType  string
	PairedAt    time.Time
	LastSeen    *time.Time
}

// Host represents a paired desktop host.
type Host struct {
	ID          string
	ClientID    string
	PublicKey   []byte
	DisplayName string
	PairedAt    time.Time
}

// SigningSession tracks an active Ed25519 signing session.
type SigningSession struct {
	ID         string
	ClientID   string
	PublicKey  []byte
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
}
