package common

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/plugin"
)

// RespondJSON sends a JSON response with standard headers
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondError sends a standard error response
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, ErrorResponse{Error: message})
}

// RequirePluginsReady writes a 503 with Retry-After and returns false if
// plugin loading has not yet completed. Handlers that resolve or invoke
// a plugin should call this before doing any work so callers get a clear
// signal instead of an "unknown plugin" error during startup.
func RequirePluginsReady(w http.ResponseWriter) bool {
	if plugin.GetLoadState().Ready() {
		return true
	}
	w.Header().Set("Retry-After", "5")
	RespondError(w, http.StatusServiceUnavailable, "Plugins are still loading, try again shortly")
	return false
}

// RespondValidationError sends a validation error response with field-level errors
func RespondValidationError(w http.ResponseWriter, message string, fields []ValidationError) {
	RespondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
		Error:  message,
		Fields: fields,
	})
}

// ParseLimit parses and validates limit parameter
func ParseLimit(limitStr string, defaultLimit, maxLimit int) int {
	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}

// ParseOffset parses and validates offset parameter
func ParseOffset(offsetStr string) int {
	if offsetStr == "" {
		return 0
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0
	}

	return offset
}
