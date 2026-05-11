package api

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const clientIDKey contextKey = "client_id"

// requireJWT returns middleware that validates a JWT and checks the role.
func (s *Server) requireJWT(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				fail(w, "missing authorization token", http.StatusUnauthorized)
				return
			}

			claims, err := s.jwt.ValidateToken(tokenStr)
			if err != nil {
				fail(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if claims.Role != role {
				fail(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			// Store client ID in context for downstream handlers.
			var id string
			if role == "client" {
				id = claims.ClientID
			} else {
				id = claims.HostID
			}
			ctx := context.WithValue(r.Context(), clientIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// clientIDFromContext extracts the client ID from the request context.
func clientIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clientIDKey).(string)
	return id, ok
}

// extractBearerToken extracts the token from an Authorization: Bearer header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
