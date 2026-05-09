package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Frontend FrontendConfig
	Git      GitConfig
	Relay    RelayConfig
}

type ServerConfig struct {
	Host string
	Port int
	// AllowedOrigins for CORS, comma-separated.
	AllowedOrigins []string
	// Mode: "local" or "remote".
	Mode string
}

type DatabaseConfig struct {
	// Driver: "sqlite" or "postgres".
	Driver string
	// DSN is the connection string.
	DSN string
}

type FrontendConfig struct {
	// DistDir is the path to the built frontend assets.
	DistDir string
}

type GitConfig struct {
	// EditorCMD is the command to open files in the user's editor.
	EditorCMD string
}

type RelayConfig struct {
	// Enabled controls whether relay features are active.
	Enabled bool
	// APIBase is the relay server URL.
	APIBase string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:           getEnv("HOST", "127.0.0.1"),
			Port:           getEnvInt("BACKEND_PORT", 0),
			AllowedOrigins: getEnvSlice("VK_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
			Mode:           getEnv("VK_SERVER_MODE", "local"),
		},
		Database: DatabaseConfig{
			Driver: getEnv("VK_DB_DRIVER", "sqlite"),
			DSN:    getEnv("VK_DB_DSN", ""),
		},
		Frontend: FrontendConfig{
			DistDir: getEnv("VK_FRONTEND_DIST", "web/dist"),
		},
		Git: GitConfig{
			EditorCMD: getEnv("VK_EDITOR", ""),
		},
		Relay: RelayConfig{
			Enabled: getEnvBool("VK_RELAY_ENABLED", false),
			APIBase: getEnv("VK_SHARED_RELAY_API_BASE", ""),
		},
	}

	// Auto-assign port if 0.
	if cfg.Server.Port == 0 {
		cfg.Server.Port = getEnvInt("PORT", 3210)
	}

	// Default SQLite DSN.
	if cfg.Database.DSN == "" && cfg.Database.Driver == "sqlite" {
		assetDir := getEnv("VK_ASSET_DIR", "")
		if assetDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("get home dir: %w", err)
			}
			assetDir = homeDir + "/.vibe-kanban"
		}
		cfg.Database.DSN = assetDir + "/db.v2.sqlite"
	}

	return cfg, nil
}

// Address returns the server listen address.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
