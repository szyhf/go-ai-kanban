package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := w.Header().Get("X-Request-ID")
		if id == "" {
			t.Error("expected X-Request-ID header to be set")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID in response header")
	}
}

func TestRequestIDPassthrough(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should keep the existing ID.
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "test-id-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "test-id-123" {
		t.Errorf("expected X-Request-ID to be preserved, got %q", got)
	}
}

func TestLogger(t *testing.T) {
	logger := slog.Default()
	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRecovery(t *testing.T) {
	logger := slog.Default()
	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestRecoveryNoPanic(t *testing.T) {
	logger := slog.Default()
	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name         string
		allowed      []string
		origin       string
		method       string
		expectOrigin string
		expectStatus int
	}{
		{
			name:         "matching origin",
			allowed:      []string{"http://localhost:3000"},
			origin:       "http://localhost:3000",
			method:       http.MethodGet,
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusOK,
		},
		{
			name:         "wildcard allows any origin",
			allowed:      []string{"*"},
			origin:       "http://example.com",
			method:       http.MethodGet,
			expectOrigin: "http://example.com",
			expectStatus: http.StatusOK,
		},
		{
			name:         "non-matching origin uses fallback",
			allowed:      []string{"http://localhost:3000"},
			origin:       "http://evil.com",
			method:       http.MethodGet,
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusOK,
		},
		{
			name:         "OPTIONS preflight",
			allowed:      []string{"http://localhost:3000"},
			origin:       "http://localhost:3000",
			method:       http.MethodOptions,
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusNoContent,
		},
		{
			name:         "no origin header",
			allowed:      []string{"http://localhost:3000"},
			origin:       "",
			method:       http.MethodGet,
			expectOrigin: "http://localhost:3000",
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CORS(tt.allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, rec.Code)
			}

			if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
				t.Error("expected Access-Control-Allow-Methods header")
			}
		})
	}
}

func TestLogServerErrors(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := LogServerErrors(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, rec.Code)
			}
		})
	}
}

// responseWriter tests.
func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("expected statusCode 404, got %d", rw.statusCode)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected recorded code 404, got %d", rec.Code)
	}
}
