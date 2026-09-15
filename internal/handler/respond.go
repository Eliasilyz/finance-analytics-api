package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

// errorEnvelope is the single error shape every handler returns.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	codeValidation   = "VALIDATION_ERROR"
	codeNotFound     = "NOT_FOUND"
	codeInsufficient = "INSUFFICIENT_DATA"
	codeRateLimited  = "RATE_LIMITED"
	codeUnauthorized = "UNAUTHORIZED"
	codeInternal     = "INTERNAL"
)

// writeJSON renders body with the given status. data is encoded as
// {"data": body} unless it is nil.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError renders the shared error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

// writeData renders a successful {"data": ...} response.
func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"data": data})
}

// logError records a 500 so unexpected failures are at least visible.
func logError(log *slog.Logger, err error) {
	if log != nil {
		log.Error("internal error", "error", err)
	}
}

// writeServiceError maps QueryService errors onto the shared error envelope.
func writeServiceError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidSymbol):
		writeError(w, http.StatusBadRequest, codeValidation, err.Error())
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, codeNotFound, err.Error())
	case errors.Is(err, service.ErrInsufficientData):
		writeError(w, http.StatusUnprocessableEntity, codeInsufficient, err.Error())
	default:
		logError(log, err)
		writeError(w, http.StatusInternalServerError, codeInternal, "internal error")
	}
}
