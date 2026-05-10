package api

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response envelope.
// Format: {success, data, error_data, message}.
type Response struct {
	Success   bool            `json:"success"`
	Data      json.RawMessage `json:"data,omitempty"`
	ErrorData json.RawMessage `json:"error_data,omitempty"`
	Message   string          `json:"message,omitempty"`
}

// OK writes a successful JSON response.
func OK(w http.ResponseWriter, data any) {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    rawData,
	})
}

// OKWithStatus writes a successful JSON response with a custom status code.
func OKWithStatus(w http.ResponseWriter, status int, data any) {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	writeJSON(w, status, Response{
		Success: true,
		Data:    rawData,
	})
}

// Created writes a 201 Created response.
func Created(w http.ResponseWriter, data any) {
	OKWithStatus(w, http.StatusCreated, data)
}

// Error writes an error JSON response.
func Error(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Message: message,
	})
}

// ErrorWithData writes an error JSON response with structured error data.
func ErrorWithData(w http.ResponseWriter, status int, message string, errData any) {
	var rawErr json.RawMessage
	if errData != nil {
		rawErr, _ = json.Marshal(errData)
	}
	writeJSON(w, status, Response{
		Success:   false,
		Message:   message,
		ErrorData: rawErr,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
