package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/xuzhiping7/ai-kanban/internal/config"
	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/server/middleware"
)

// NewHandler builds the root HTTP handler with all routes and middleware.
func NewHandler(cfg *config.Config, db *database.DB, logger *slog.Logger) (http.Handler, error) {
	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":null}`))
	})

	// TODO: Register API routes here.

	// Frontend SPA serving (must be last).
	if distDir := cfg.Frontend.DistDir; distDir != "" {
		spa := spaHandler{distDir: distDir}
		mux.Handle("/", spa)
	}

	// Apply middleware chain.
	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Logger(logger)(handler)
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.CORS(cfg.Server.AllowedOrigins)(handler)

	return handler, nil
}

// spaHandler serves a Single Page Application from a directory.
type spaHandler struct {
	distDir string
	fileServer http.Handler
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Lazy-init file server.
	if h.fileServer == nil {
		if _, err := os.Stat(h.distDir); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		h.fileServer = http.FileServer(http.Dir(h.distDir))
	}

	path := r.URL.Path

	// Try serving the exact file.
	filePath := h.distDir + path
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// Fallback to index.html for SPA routing.
	r.URL.Path = "/"
	h.fileServer.ServeHTTP(w, r)
}

// EmbedFrontend embeds pre-built frontend assets.
func EmbedFrontend(embeddedFS embed.FS) http.Handler {
	sub, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		panic("failed to get embedded frontend: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip API routes.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try exact file.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if f, err := embeddedFS.Open("dist/" + path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
