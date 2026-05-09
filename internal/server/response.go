package server

import (
	"net/http"

	"github.com/xuzhiping7/ai-kanban/internal/httputil"
)

// Re-export response helpers from httputil for backward compatibility.
// New code should import httputil directly.

// ApiResponse is the standard JSON response envelope.
type ApiResponse = httputil.ApiResponse

// Success sends a successful JSON response.
func Success(w http.ResponseWriter, data interface{}) {
	httputil.Success(w, data)
}

// SuccessMessage sends a successful JSON response with a message.
func SuccessMessage(w http.ResponseWriter, message string) {
	httputil.SuccessMessage(w, message)
}

// Created sends a 201 Created response with data.
func Created(w http.ResponseWriter, data interface{}) {
	httputil.Created(w, data)
}

// Error sends an error response with the given status code and message.
func Error(w http.ResponseWriter, code int, message string) {
	httputil.Error(w, code, message)
}

// ErrorWithData sends an error response with additional error data.
func ErrorWithData(w http.ResponseWriter, code int, message string, errData interface{}) {
	httputil.ErrorWithData(w, code, message, errData)
}

// NotFound sends a 404 response.
func NotFound(w http.ResponseWriter, message string) {
	httputil.NotFound(w, message)
}

// BadRequest sends a 400 response.
func BadRequest(w http.ResponseWriter, message string) {
	httputil.BadRequest(w, message)
}

// InternalError sends a 500 response.
func InternalError(w http.ResponseWriter, message string) {
	httputil.InternalError(w, message)
}
