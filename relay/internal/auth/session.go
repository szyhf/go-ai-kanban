package auth

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"
)

const (
	SessionTTL     = 1 * time.Hour
	SessionIdleTTL = 15 * time.Minute
	NonceTTL       = 2 * time.Minute
	TimestampDrift = 30 * time.Second
)

// SessionStore manages signing sessions in memory.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SigningSession
	nonces   map[string]time.Time // nonce -> expires at
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*SigningSession),
		nonces:   make(map[string]time.Time),
	}
}

// CreateSession creates a new signing session.
func (s *SessionStore) CreateSession(id, clientID string, publicKey []byte) *SigningSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	session := &SigningSession{
		ID:         id,
		ClientID:   clientID,
		PublicKey:  ed25519.PublicKey(publicKey),
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.Add(SessionTTL),
	}
	s.sessions[id] = session
	return session
}

// GetSession returns a session by ID, checking if it's expired.
func (s *SessionStore) GetSession(id string) (*SigningSession, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check if session has expired.
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// DeleteSession removes a session.
func (s *SessionStore) DeleteSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// CheckAndConsumeNonce records a nonce and checks if it was already used.
// Returns error if nonce was already used or expired.
func (s *SessionStore) CheckAndConsumeNonce(nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	if nonce == "" {
		return fmt.Errorf("nonce is empty")
	}

	// Check if nonce already exists.
	if expiresAt, exists := s.nonces[nonce]; exists {
		// If the existing nonce has not expired, it's a replay.
		if now.Before(expiresAt) {
			return fmt.Errorf("nonce already used: %s", nonce)
		}
		// If it has expired, we can allow reuse of the same nonce string,
		// but this is unusual. Clean it up first.
		delete(s.nonces, nonce)
	}

	// Record the nonce with its expiry time.
	s.nonces[nonce] = now.Add(NonceTTL)
	return nil
}

// RefreshSession extends a session's expiry.
func (s *SessionStore) RefreshSession(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return false
	}

	now := time.Now()
	// Cannot refresh an already expired session.
	if now.After(session.ExpiresAt) {
		return false
	}

	session.ExpiresAt = now.Add(SessionTTL)
	session.LastUsedAt = now
	return true
}

// Clean removes expired sessions and nonces. Call periodically.
func (s *SessionStore) Clean() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Remove expired sessions.
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}

	// Remove expired nonces.
	for nonce, expiresAt := range s.nonces {
		if now.After(expiresAt) {
			delete(s.nonces, nonce)
		}
	}
}

// StartCleaner starts a background goroutine that cleans expired sessions every minute.
// Returns a stop channel. Close the channel to stop the cleaner.
func (s *SessionStore) StartCleaner() chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Clean()
			case <-stop:
				return
			}
		}
	}()
	return stop
}
