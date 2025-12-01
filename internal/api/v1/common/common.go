package common

import (
	"net/http"
	"time"
)

// Context keys for storing authenticated user info
type ContextKey string

const (
	UserContextKey ContextKey = "user"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// ValidationErrorResponse represents validation errors
type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Fields []ValidationError `json:"fields,omitempty"`
}

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Filter represents common query parameters for list operations
type Filter struct {
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
	Sort   []string `json:"sort,omitempty"`
}

// TimeRange represents a time-based filter range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Route represents a route for the HTTP server
type Route struct {
	Path       string
	Method     string
	Handler    http.HandlerFunc
	Middleware []func(http.HandlerFunc) http.HandlerFunc
}
