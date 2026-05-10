package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/xuzhiping7/ai-kanban/internal/config"
	"github.com/xuzhiping7/ai-kanban/internal/database"
	"github.com/xuzhiping7/ai-kanban/internal/server/middleware"
)

// NewRouter builds the root HTTP handler with all routes and middleware.
// The returned chi.Router can be used to mount additional route groups.
func NewRouter(cfg *config.Config, db *database.DB, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware (outermost first).
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	r.Use(chimw.Compress(5))

	// Frontend SPA serving (root level).
	r.Get("/", serveFrontendRoot(cfg))
	r.Get("/*", serveFrontend(cfg))

	// API routes.
	r.Route("/api", func(r chi.Router) {
		// API-level middleware.
		r.Use(middleware.ValidateOrigin(logger, middleware.OriginValidationConfig{
			AllowedOrigins: cfg.Server.AllowedOrigins,
		}))
		r.Use(middleware.LogServerErrors(logger))

		// Health check.
		r.Get("/health", healthCheck)

		// System info.
		r.Get("/info", infoHandler(cfg))
	})

	return r
}

// healthCheck returns a simple health response.
func healthCheck(w http.ResponseWriter, _ *http.Request) {
	Success(w, map[string]string{"status": "ok"})
}

// infoHandler returns system information.
func infoHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		Success(w, map[string]any{
			"mode":     cfg.Server.Mode,
			"dbDriver": cfg.Database.Driver,
		})
	}
}

// serveFrontendRoot serves index.html for the root path.
func serveFrontendRoot(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveSPAFile(cfg, w, r, "index.html")
	}
}

// serveFrontend serves static files with SPA fallback.
func serveFrontend(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try exact file match.
		filePath := cfg.Frontend.DistDir + "/" + path
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, filePath)
			return
		}

		// Fallback to index.html for SPA routing.
		serveSPAFile(cfg, w, r, "index.html")
	}
}

func serveSPAFile(cfg *config.Config, w http.ResponseWriter, r *http.Request, name string) {
	filePath := cfg.Frontend.DistDir + "/" + name
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, "frontend not built", http.StatusServiceUnavailable)
		return
	}
	http.ServeFile(w, r, filePath)
}

// EmbedFrontend serves pre-built frontend assets from an embedded FS.
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
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
