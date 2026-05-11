package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database and provides CRUD operations for the relay server.
type Store struct {
	db *sql.DB
}

// NewStore opens a SQLite database at dbPath, creates the parent directory if
// needed, and runs all pending migrations within a single transaction.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys for better concurrency and cascade support.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(context.Background(), p); err != nil {
			db.Close()
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	// Run migrations in a single transaction.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("begin migration transaction: %w", err)
	}
	for i, m := range Migrations {
		if _, err := tx.ExecContext(context.Background(), m); err != nil {
			tx.Rollback()
			db.Close()
			return nil, fmt.Errorf("run migration %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		db.Close()
		return nil, fmt.Errorf("commit migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ---------------------------------------------------------------------------
// Server keys
// ---------------------------------------------------------------------------

// GetServerKey returns the most recently created server key pair.
func (s *Store) GetServerKey() (*ServerKey, error) {
	row := s.db.QueryRowContext(context.Background(),
		"SELECT id, public_key, private_key, created_at FROM server_keys ORDER BY id DESC LIMIT 1",
	)
	var sk ServerKey
	if err := row.Scan(&sk.ID, &sk.PublicKey, &sk.PrivateKey, &sk.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get server key: %w", err)
	}
	return &sk, nil
}

// SaveServerKey inserts a new server key pair and returns it with the assigned ID.
func (s *Store) SaveServerKey(pub, priv []byte) (*ServerKey, error) {
	now := time.Now()
	res, err := s.db.ExecContext(context.Background(),
		"INSERT INTO server_keys (public_key, private_key, created_at) VALUES (?, ?, ?)",
		pub, priv, now,
	)
	if err != nil {
		return nil, fmt.Errorf("save server key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}
	return &ServerKey{
		ID:         id,
		PublicKey:  pub,
		PrivateKey: priv,
		CreatedAt:  now,
	}, nil
}

// ---------------------------------------------------------------------------
// Enrollment codes
// ---------------------------------------------------------------------------

// CreateEnrollmentCode inserts a new one-time pairing code.
func (s *Store) CreateEnrollmentCode(code string) error {
	_, err := s.db.ExecContext(context.Background(),
		"INSERT INTO enrollment_codes (code, created_at) VALUES (?, ?)",
		code, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("create enrollment code: %w", err)
	}
	return nil
}

// UseEnrollmentCode marks a code as used if it has not expired (5 minute TTL)
// and has not already been used. Returns true if the code was valid and marked
// as used.
func (s *Store) UseEnrollmentCode(code string) (bool, error) {
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)

	res, err := s.db.ExecContext(context.Background(),
		"UPDATE enrollment_codes SET used_at = ? WHERE code = ? AND used_at IS NULL AND created_at >= ?",
		now, code, cutoff,
	)
	if err != nil {
		return false, fmt.Errorf("use enrollment code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check rows affected: %w", err)
	}
	return n > 0, nil
}

// CleanExpiredCodes deletes enrollment codes older than 1 hour.
func (s *Store) CleanExpiredCodes() error {
	cutoff := time.Now().Add(-1 * time.Hour)
	_, err := s.db.ExecContext(context.Background(),
		"DELETE FROM enrollment_codes WHERE created_at < ?",
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("clean expired codes: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Clients
// ---------------------------------------------------------------------------

// SaveClient upserts a client record.
func (s *Store) SaveClient(c *Client) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO clients (id, public_key, display_name, device_type, paired_at, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 public_key = excluded.public_key,
			 display_name = excluded.display_name,
			 device_type = excluded.device_type,
			 last_seen = excluded.last_seen`,
		c.ID, c.PublicKey, c.DisplayName, c.DeviceType, c.PairedAt, c.LastSeen,
	)
	if err != nil {
		return fmt.Errorf("save client: %w", err)
	}
	return nil
}

// GetClient returns a client by ID. Returns nil if not found.
func (s *Store) GetClient(id string) (*Client, error) {
	row := s.db.QueryRowContext(context.Background(),
		"SELECT id, public_key, display_name, device_type, paired_at, last_seen FROM clients WHERE id = ?",
		id,
	)
	var c Client
	if err := row.Scan(&c.ID, &c.PublicKey, &c.DisplayName, &c.DeviceType, &c.PairedAt, &c.LastSeen); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get client: %w", err)
	}
	return &c, nil
}

// ListClients returns all registered clients.
func (s *Store) ListClients() ([]Client, error) {
	rows, err := s.db.QueryContext(context.Background(),
		"SELECT id, public_key, display_name, device_type, paired_at, last_seen FROM clients",
	)
	if err != nil {
		return nil, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.PublicKey, &c.DisplayName, &c.DeviceType, &c.PairedAt, &c.LastSeen); err != nil {
			return nil, fmt.Errorf("scan client: %w", err)
		}
		clients = append(clients, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clients: %w", err)
	}
	return clients, nil
}

// DeleteClient deletes a client by ID. Hosts and signing sessions are cascade-deleted.
func (s *Store) DeleteClient(id string) error {
	_, err := s.db.ExecContext(context.Background(),
		"DELETE FROM clients WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("delete client: %w", err)
	}
	return nil
}

// UpdateClientLastSeen sets the last_seen timestamp for a client to now.
func (s *Store) UpdateClientLastSeen(id string) error {
	_, err := s.db.ExecContext(context.Background(),
		"UPDATE clients SET last_seen = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update client last seen: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hosts
// ---------------------------------------------------------------------------

// SaveHost upserts a host record.
func (s *Store) SaveHost(h *Host) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO hosts (id, client_id, public_key, display_name, paired_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 client_id = excluded.client_id,
			 public_key = excluded.public_key,
			 display_name = excluded.display_name`,
		h.ID, h.ClientID, h.PublicKey, h.DisplayName, h.PairedAt,
	)
	if err != nil {
		return fmt.Errorf("save host: %w", err)
	}
	return nil
}

// ListHostsByClient returns all hosts belonging to a client.
func (s *Store) ListHostsByClient(clientID string) ([]Host, error) {
	rows, err := s.db.QueryContext(context.Background(),
		"SELECT id, client_id, public_key, display_name, paired_at FROM hosts WHERE client_id = ?",
		clientID,
	)
	if err != nil {
		return nil, fmt.Errorf("list hosts by client: %w", err)
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.ClientID, &h.PublicKey, &h.DisplayName, &h.PairedAt); err != nil {
			return nil, fmt.Errorf("scan host: %w", err)
		}
		hosts = append(hosts, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hosts: %w", err)
	}
	return hosts, nil
}

// DeleteHost deletes a host by ID.
func (s *Store) DeleteHost(id string) error {
	_, err := s.db.ExecContext(context.Background(),
		"DELETE FROM hosts WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Signing sessions
// ---------------------------------------------------------------------------

// SaveSigningSession inserts a new signing session.
func (s *Store) SaveSigningSession(ss *SigningSession) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO signing_sessions (id, client_id, public_key, created_at, last_used_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		ss.ID, ss.ClientID, ss.PublicKey, ss.CreatedAt, ss.LastUsedAt, ss.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save signing session: %w", err)
	}
	return nil
}

// GetSigningSession returns a signing session by ID. Returns nil if not found.
func (s *Store) GetSigningSession(id string) (*SigningSession, error) {
	row := s.db.QueryRowContext(context.Background(),
		"SELECT id, client_id, public_key, created_at, last_used_at, expires_at FROM signing_sessions WHERE id = ?",
		id,
	)
	var ss SigningSession
	if err := row.Scan(&ss.ID, &ss.ClientID, &ss.PublicKey, &ss.CreatedAt, &ss.LastUsedAt, &ss.ExpiresAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get signing session: %w", err)
	}
	return &ss, nil
}

// DeleteSigningSession deletes a signing session by ID.
func (s *Store) DeleteSigningSession(id string) error {
	_, err := s.db.ExecContext(context.Background(),
		"DELETE FROM signing_sessions WHERE id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("delete signing session: %w", err)
	}
	return nil
}

// CleanExpiredSessions deletes all signing sessions past their expires_at time.
func (s *Store) CleanExpiredSessions() error {
	_, err := s.db.ExecContext(context.Background(),
		"DELETE FROM signing_sessions WHERE expires_at < ?",
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("clean expired sessions: %w", err)
	}
	return nil
}

// UpdateSessionLastUsed sets the last_used_at timestamp for a session to now.
func (s *Store) UpdateSessionLastUsed(id string) error {
	_, err := s.db.ExecContext(context.Background(),
		"UPDATE signing_sessions SET last_used_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update session last used: %w", err)
	}
	return nil
}
