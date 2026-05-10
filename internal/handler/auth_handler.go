package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerAuthRoutes registers authentication-related routes.
// In local mode, all auth endpoints are stubs.
func (h *Handler) registerAuthRoutes(r chi.Router) {
	r.Get("/methods", h.handleAuthMethods)
	r.Post("/handoff/init", h.handleAuthHandoffInit)
	r.Get("/status", h.handleAuthStatus)
	r.Post("/local/login", h.handleAuthLocalLogin)
	r.Post("/logout", h.handleAuthLogout)
	r.Get("/token", h.handleAuthToken)
	r.Get("/user", h.handleAuthUser)
}

// handleAuthMethods handles GET /api/auth/methods.
func (h *Handler) handleAuthMethods(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"local_auth_enabled": false,
		"oauth_providers":    []string{},
	})
}

// handleAuthHandoffInit handles POST /api/auth/handoff/init.
func (h *Handler) handleAuthHandoffInit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"success":false,"error_data":{"message":"OAuth not available in local mode"},"message":"not implemented"}`, http.StatusNotImplemented)
}

// handleAuthStatus handles GET /api/auth/status.
func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"authenticated": false,
	})
}

// handleAuthLocalLogin handles POST /api/auth/local/login.
func (h *Handler) handleAuthLocalLogin(w http.ResponseWriter, r *http.Request) {
	// In local mode, auto-login with a default user.
	success(w, map[string]interface{}{
		"user_id":  "local-user",
		"email":    "local@localhost",
		"username": "local",
	})
}

// handleAuthLogout handles POST /api/auth/logout.
func (h *Handler) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// handleAuthToken handles GET /api/auth/token.
func (h *Handler) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	success(w, map[string]interface{}{
		"token": nil,
	})
}

// handleAuthUser handles GET /api/auth/user.
func (h *Handler) handleAuthUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"success":false,"error_data":{"message":"not authenticated"},"message":"unauthorized"}`, http.StatusUnauthorized)
}
