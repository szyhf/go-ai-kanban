package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateOrigin(t *testing.T) {
	logger := slog.Default()
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	tests := []struct {
		name         string
		origin       string
		host         string
		relay        bool
		extraOrigins []string
		expectCode   int
	}{
		{
			name:       "no origin header allowed",
			origin:     "",
			host:       "localhost:3210",
			expectCode: http.StatusOK,
		},
		{
			name:       "same origin allowed",
			origin:     "http://localhost:3210",
			host:       "localhost:3210",
			expectCode: http.StatusOK,
		},
		{
			name:       "loopback equivalence 127.0.0.1",
			origin:     "http://127.0.0.1:3210",
			host:       "localhost:3210",
			expectCode: http.StatusOK,
		},
		{
			name:       "loopback equivalence ::1",
			origin:     "http://[::1]:3210",
			host:       "localhost:3210",
			expectCode: http.StatusOK,
		},
		{
			name:       "different origin rejected",
			origin:     "http://evil.com:3210",
			host:       "localhost:3210",
			expectCode: http.StatusForbidden,
		},
		{
			name:         "extra allowed origin",
			origin:       "http://example.com",
			host:         "localhost:3210",
			extraOrigins: []string{"http://example.com"},
			expectCode:   http.StatusOK,
		},
		{
			name:       "relay bypasses validation",
			origin:     "http://evil.com",
			host:       "localhost:3210",
			relay:      true,
			expectCode: http.StatusOK,
		},
		{
			name:       "https scheme same host",
			origin:     "https://localhost:3210",
			host:       "localhost:3210",
			expectCode: http.StatusOK, // only host+port are compared, not scheme
		},
		{
			name:       "port mismatch rejected",
			origin:     "http://localhost:3000",
			host:       "localhost:3210",
			expectCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ValidateOrigin(logger, OriginValidationConfig{
				AllowedOrigins: tt.extraOrigins,
			})(okHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.relay {
				req.Header.Set("X-Relay", "1")
			}
			req.Host = tt.host

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectCode {
				t.Errorf("expected status %d, got %d", tt.expectCode, rec.Code)
			}
		})
	}
}

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"same host port", "http://localhost:3000", "localhost:3000", true},
		{"loopback normalization", "http://127.0.0.1:3000", "localhost:3000", true},
		{"ipv6 loopback", "http://[::1]:3000", "localhost:3000", true},
		{"different host", "http://example.com:3000", "localhost:3000", false},
		{"different port", "http://localhost:3000", "localhost:8080", false},
		{"origin without port", "http://localhost", "localhost:8080", true},
		{"https vs http same host", "https://localhost:3000", "localhost:3000", true}, // scheme not checked
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSameOrigin(tt.origin, tt.host)
			if got != tt.want {
				t.Errorf("isSameOrigin(%q, %q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort string
	}{
		{"localhost:3000", "localhost", "3000"},
		{"http://localhost:3000", "localhost", "3000"},
		{"https://example.com:443/path", "example.com", "443"},
		{"example.com", "example.com", ""},
		{"http://example.com", "example.com", ""},
		{"127.0.0.1:8080", "127.0.0.1", "8080"},
		{"[::1]:3000", "::1", "3000"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port := splitHostPort(tt.input)
			if host != tt.wantHost || port != tt.wantPort {
				t.Errorf("splitHostPort(%q) = (%q, %q), want (%q, %q)",
					tt.input, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestNormalizeLoopback(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"localhost", "localhost"},
		{"127.0.0.1", "localhost"},
		{"::1", "localhost"},
		{"[::1]", "localhost"},
		{"example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeLoopback(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLoopback(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
