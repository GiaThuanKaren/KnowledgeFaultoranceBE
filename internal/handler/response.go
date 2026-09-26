package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/feaziest/kfdesktopbe/internal/domain"
)

// ErrorEnvelope wraps standardized error responses
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes an error code and user-facing message
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON sets the Content-Type header and encodes data to JSON with status code
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes a standardized error envelope JSON response
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorEnvelope{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// HandleError translates domain errors into corresponding HTTP error responses
func HandleError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, domain.ErrValidation):
		WriteError(w, http.StatusBadRequest, "ERR_VALIDATION", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Unauthorized access")
	case errors.Is(err, domain.ErrForbidden):
		WriteError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Forbidden resource")
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, http.StatusNotFound, "ERR_NOT_FOUND", "Resource not found")
	case errors.Is(err, domain.ErrRateLimited):
		WriteError(w, http.StatusTooManyRequests, "ERR_RATE_LIMITED", "Rate limit exceeded")
	default:
		WriteError(w, http.StatusInternalServerError, "ERR_INTERNAL_SERVER", "Internal server error")
	}
}
