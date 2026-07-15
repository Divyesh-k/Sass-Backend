// Package response provides a single, consistent JSON envelope for every
// handler in the service so API consumers never have to guess the shape
// of a success or error payload.
package response

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Data  any     `json:"data,omitempty"`
	Error *apiErr `json:"error,omitempty"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes a success payload wrapped in the standard envelope.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

// Error writes a standardized error payload. `code` is a stable,
// machine-readable identifier (e.g. "invalid_credentials") that clients
// can switch on without parsing the human-readable message.
func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: &apiErr{Code: code, Message: message}})
}
