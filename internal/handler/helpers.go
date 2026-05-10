package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/xuzhiping7/ai-kanban/internal/domain"
	"github.com/xuzhiping7/ai-kanban/internal/httputil"
)

// decodeJSON reads and decodes JSON from the request body.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		badRequest(w, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// parseUUID extracts and parses a UUID URL parameter.
func parseUUID(w http.ResponseWriter, r *http.Request, param string) (domain.UUID, bool) {
	s := chi.URLParam(r, param)
	if s == "" {
		badRequest(w, "missing "+param)
		return domain.UUID{}, false
	}
	id, err := domain.ParseUUID(s)
	if err != nil {
		badRequest(w, "invalid "+param+": "+err.Error())
		return domain.UUID{}, false
	}
	return id, true
}

// getQuery returns a query parameter value.
func getQuery(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// getQuerySlice returns all values for a query parameter key.
func getQuerySlice(r *http.Request, key string) []string {
	return r.URL.Query()[key]
}

// badRequest is a shorthand for 400.
func badRequest(w http.ResponseWriter, msg string) {
	httputil.BadRequest(w, msg)
}

// notFound is a shorthand for 404.
func notFound(w http.ResponseWriter, msg string) {
	httputil.NotFound(w, msg)
}

// internalError is a shorthand for 500.
func internalError(w http.ResponseWriter, msg string) {
	httputil.InternalError(w, msg)
}

// success is a shorthand for 200.
func success(w http.ResponseWriter, data any) {
	httputil.Success(w, data)
}

// created is a shorthand for 201.
func created(w http.ResponseWriter, data any) {
	httputil.Created(w, data)
}

// errorWithData sends an error response with additional structured error data.
func errorWithData(w http.ResponseWriter, code int, msg string, errData any) {
	httputil.ErrorWithData(w, code, msg, errData)
}
