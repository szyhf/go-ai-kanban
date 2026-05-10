package httputil

import (
	"encoding/json"
	"net/http"
)

// ApiResponse is the standard JSON response envelope.
type ApiResponse struct {
	Success   bool   `json:"success"`
	Data      any    `json:"data,omitempty"`
	ErrorData any    `json:"error_data,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Success sends a successful JSON response.
func Success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessMessage sends a successful JSON response with a message.
func SuccessMessage(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Message: message,
	})
}

// Created sends a 201 Created response with data.
func Created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, ApiResponse{
		Success: true,
		Data:    data,
	})
}

// Error sends an error response with the given status code and message.
func Error(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, ApiResponse{
		Success: false,
		Message: message,
	})
}

// ErrorWithData sends an error response with additional error data.
func ErrorWithData(w http.ResponseWriter, code int, message string, errData any) {
	writeJSON(w, code, ApiResponse{
		Success:   false,
		Message:   message,
		ErrorData: errData,
	})
}

// NotFound sends a 404 response.
func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "not found"
	}
	Error(w, http.StatusNotFound, message)
}

// BadRequest sends a 400 response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, message)
}

// InternalError sends a 500 response.
func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, message)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
