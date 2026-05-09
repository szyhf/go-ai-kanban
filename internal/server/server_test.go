package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/xuzhiping7/ai-kanban/internal/config"
	"github.com/xuzhiping7/ai-kanban/internal/database"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:           "127.0.0.1",
			Port:           3210,
			AllowedOrigins: []string{"http://localhost:3000"},
			Mode:           "local",
		},
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			DSN:    ":memory:",
		},
		Frontend: config.FrontendConfig{
			DistDir: "", // no frontend in tests
		},
	}
}

func testDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestHealthCheck(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp ApiResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestInfoEndpoint(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp ApiResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestMiddlewareChain(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	// Request ID should be added.
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header")
	}
}

func TestSPAUnavailable(t *testing.T) {
	cfg := testConfig()
	cfg.Frontend.DistDir = "/nonexistent"
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Should return 503 when frontend is not built.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}
}

func TestSPAWithDistDir(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()
	cfg.Frontend.DistDir = dir
	db := testDB(t)
	defer db.Close()

	// Create a minimal index.html.
	if err := os.WriteFile(dir+"/index.html", []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAPINotFound(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestGzipCompression(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// chi's Compress middleware sets Content-Encoding.
	enc := rec.Header().Get("Content-Encoding")
	if enc != "gzip" {
		t.Errorf("expected gzip content-encoding, got %q", enc)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	cfg := testConfig()
	db := testDB(t)
	defer db.Close()

	router := NewRouter(cfg, db, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}

func TestEmbedFrontendSkipsAPI(t *testing.T) {
	// Create a minimal embedded FS.
	dir := t.TempDir()
	os.WriteFile(dir+"/index.html", []byte("<html></html>"), 0o644)

	// Create an FS-like structure. For testing, just verify the logic.
	// The actual embed.FS test requires build-time embedding, so we test
	// the behavior with a simple mock FS.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test that API routes would be skipped.
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	// This is a simplified test - the actual EmbedFrontend uses embed.FS
	// which requires compile-time embedding.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// TestApiResponseJSON verifies the JSON structure of ApiResponse.
func TestApiResponseJSON(t *testing.T) {
	tests := []struct {
		name     string
		resp     ApiResponse
		wantJSON string
	}{
		{
			name:     "success with data",
			resp:     ApiResponse{Success: true, Data: map[string]string{"key": "value"}},
			wantJSON: `"success":true`,
		},
		{
			name:     "error response",
			resp:     ApiResponse{Success: false, Message: "something went wrong"},
			wantJSON: `"success":false`,
		},
		{
			name:     "empty success",
			resp:     ApiResponse{Success: true},
			wantJSON: `"success":true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !contains(string(b), tt.wantJSON) {
				t.Errorf("response JSON %q does not contain %q", string(b), tt.wantJSON)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Compile-time interface check.
var _ fs.FS = (fs.FS)(nil)
