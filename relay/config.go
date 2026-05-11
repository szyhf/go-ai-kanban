package relay

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds relay server configuration.
type Config struct {
	// Port is the HTTP server listen port.
	Port int
	// DBPath is the path to the SQLite database file.
	DBPath string
	// JWTSecret is the HMAC key for JWT signing. Auto-generated if empty.
	JWTSecret []byte
	// Host is the bind address (default "0.0.0.0").
	Host string
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dbPath := filepath.Join(homeDir, ".vibe-kanban", "relay.db")

	return &Config{
		Port:   8788,
		DBPath: dbPath,
		Host:   "0.0.0.0",
	}
}

// ConfigFromEnv creates config from environment variables with defaults.
func ConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if v := os.Getenv("RELAY_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("RELAY_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("RELAY_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("RELAY_JWT_SECRET"); v != "" {
		cfg.JWTSecret = []byte(v)
	}

	return cfg
}
