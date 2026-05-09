package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// OriginValidationConfig holds the configuration for origin validation.
type OriginValidationConfig struct {
	// AllowedOrigins is a list of additional allowed origins (from VK_ALLOWED_ORIGINS).
	AllowedOrigins []string
}

// ValidateOrigin validates the Origin header against the Host header.
// This matches the Rust implementation's custom origin validation (not traditional CORS).
//
// Rules:
//   - Requests without an Origin header are allowed (server-side, curl, etc.)
//   - Same-origin requests are allowed (origin matches host)
//   - Loopback addresses (localhost, 127.0.0.1, ::1) are normalized and treated as equivalent
//   - Additional origins from VK_ALLOWED_ORIGINS are allowed
//   - Relay-proxied requests (X-Relay: 1) bypass origin validation
func ValidateOrigin(logger *slog.Logger, cfg OriginValidationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// No origin header — allow (server-side requests, curl, etc.)
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Relay-proxied requests bypass origin validation.
			if r.Header.Get("X-Relay") == "1" {
				next.ServeHTTP(w, r)
				return
			}

			// Check additional allowed origins.
			for _, allowed := range cfg.AllowedOrigins {
				if origin == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Compare origin against host.
			host := r.Host
			if host == "" {
				host = r.URL.Host
			}

			if isSameOrigin(origin, host) {
				next.ServeHTTP(w, r)
				return
			}

			logger.Warn("origin validation failed",
				"origin", origin,
				"host", host,
				"path", r.URL.Path,
				"method", r.Method,
			)

			http.Error(w, `{"success":false,"message":"Origin not allowed"}`, http.StatusForbidden)
		})
	}
}

// isSameOrigin checks if the origin matches the host, with loopback normalization.
func isSameOrigin(origin, host string) bool {
	originHost, originPort := splitHostPort(origin)
	hostAddr, hostPort := splitHostPort(host)

	// Normalize loopback addresses.
	originHost = normalizeLoopback(originHost)
	hostAddr = normalizeLoopback(hostAddr)

	if originHost != hostAddr {
		return false
	}

	// If both have ports, they must match.
	if originPort != "" && hostPort != "" {
		return originPort == hostPort
	}

	return true
}

// normalizeLoopback normalizes loopback addresses to "localhost".
func normalizeLoopback(host string) string {
	if host == "127.0.0.1" || host == "::1" || host == "[::1]" {
		return "localhost"
	}
	return host
}

// splitHostPort extracts host and port from a URL-like string.
// Handles: "localhost:3000", "http://localhost:3000", "https://example.com", etc.
func splitHostPort(s string) (string, string) {
	// Strip scheme.
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")

	// Strip path.
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}

	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// No port.
		return s, ""
	}
	return host, port
}
