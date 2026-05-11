package store

// Migrations is an ordered list of SQL migration statements.
var Migrations = []string{
	`CREATE TABLE IF NOT EXISTS server_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_key BLOB NOT NULL,
		private_key BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS enrollment_codes (
		code TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		used_at DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS clients (
		id TEXT PRIMARY KEY,
		public_key BLOB NOT NULL,
		display_name TEXT DEFAULT '',
		device_type TEXT DEFAULT '',
		paired_at DATETIME NOT NULL,
		last_seen DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS hosts (
		id TEXT PRIMARY KEY,
		client_id TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
		public_key BLOB NOT NULL,
		display_name TEXT DEFAULT '',
		paired_at DATETIME NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS signing_sessions (
		id TEXT PRIMARY KEY,
		client_id TEXT NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
		public_key BLOB NOT NULL,
		created_at DATETIME NOT NULL,
		last_used_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_signing_sessions_client ON signing_sessions(client_id)`,
	`CREATE INDEX IF NOT EXISTS idx_signing_sessions_expires ON signing_sessions(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_enrollment_codes_created ON enrollment_codes(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_hosts_client ON hosts(client_id)`,
}
