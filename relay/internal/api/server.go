package api

import (
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/xuzhiping7/ai-kanban/relay/internal/auth"
	"github.com/xuzhiping7/ai-kanban/relay/internal/signaling"
	"github.com/xuzhiping7/ai-kanban/relay/internal/store"
)

// Server is the relay HTTP server.
type Server struct {
	store       *store.Store
	jwt         *auth.JWTManager
	sessions    *auth.SessionStore
	hub         *signaling.Hub
	hubStop     chan struct{}
	hostChecker HostChecker
}

// HostChecker verifies that a host ID belongs to the authenticated entity.
type HostChecker interface {
	IsValidHost(hostID string) bool
}

// NewServer creates a new relay API server.
func NewServer(s *store.Store, jwtMgr *auth.JWTManager, sessions *auth.SessionStore) *Server {
	hub := signaling.NewHub()
	hubStop := make(chan struct{})
	go hub.Run(hubStop)

	return &Server{
		store:    s,
		jwt:      jwtMgr,
		sessions: sessions,
		hub:      hub,
		hubStop:  hubStop,
	}
}

// Close shuts down the server's background resources (hub goroutine).
func (s *Server) Close() {
	close(s.hubStop)
}

// Handler returns the root HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// Health check.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// WebSocket endpoint.
	r.Get("/api/ws", s.handleWebSocket)

	// API routes.
	r.Route("/api/relay-auth", func(r chi.Router) {
		// Server-side endpoints (host-authenticated).
		r.Group(func(r chi.Router) {
			r.Use(s.requireJWT("host"))
			r.Post("/server/enrollment-code", s.handleEnrollmentCode)
			r.Get("/server/clients", s.handleListClients)
			r.Delete("/server/clients/{clientId}", s.handleRemoveClient)
		})

		// SPAKE2 pairing endpoints (no auth — enrollment code is the auth).
		r.Post("/server/spake2/start", s.handleSpake2Start)
		r.Post("/server/spake2/finish", s.handleSpake2Finish)

		// Signing session refresh (client-authenticated).
		r.Group(func(r chi.Router) {
			r.Use(s.requireJWT("client"))
			r.Post("/server/signing-session/refresh", s.handleSigningSessionRefresh)
			r.Get("/client/hosts", s.handleListHosts)
			r.Delete("/client/hosts/{hostId}", s.handleRemoveHost)
		})
	})

	return r
}

// handleWebSocket upgrades an HTTP connection to WebSocket and hands off to
// the signaling handler. Authentication is via JWT token in the
// "token" query parameter.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		fail(w, "missing token parameter", http.StatusUnauthorized)
		return
	}

	claims, err := s.jwt.ValidateToken(tokenStr)
	if err != nil {
		fail(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	_ = claims // Claims are validated; peer identity comes from register message.

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		// websocket.Accept writes the error response.
		return
	}

	handler := signaling.NewHandler(s.hub, conn)
	go handler.Serve()
}
