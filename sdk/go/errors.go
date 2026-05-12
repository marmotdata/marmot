package marmot

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-openapi/runtime"
)

// APIError is the base type embedded in every typed SDK error.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("marmot: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("marmot: %s", e.Message)
}

// AuthError is returned for 401 and 403 responses.
type AuthError struct{ *APIError }

// NotFoundError is returned for 404 responses.
type NotFoundError struct{ *APIError }

// ValidationError is returned for 400 responses.
type ValidationError struct{ *APIError }

// RateLimitError is returned for 429 responses.
type RateLimitError struct{ *APIError }

// ServerError is returned for 5xx responses.
type ServerError struct{ *APIError }

// IsNotFound reports whether err is a *NotFoundError.
func IsNotFound(err error) bool {
	var e *NotFoundError
	return errors.As(err, &e)
}

// IsRateLimit reports whether err is a *RateLimitError.
func IsRateLimit(err error) bool {
	var e *RateLimitError
	return errors.As(err, &e)
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	code, hasCode := statusCode(err)
	msg := extractPayloadMessage(err)
	if !hasCode {
		var apiErr *runtime.APIError
		if errors.As(err, &apiErr) {
			code, hasCode = apiErr.Code, true
		}
	}
	if !hasCode {
		return err
	}
	if msg == "" {
		msg = err.Error()
	}
	return typed(code, msg)
}

func statusCode(err error) (int, bool) {
	type coder interface{ Code() int }
	var c coder
	if errors.As(err, &c) {
		return c.Code(), true
	}
	return 0, false
}

func typed(status int, msg string) error {
	base := &APIError{StatusCode: status, Message: msg}
	switch {
	case status == 400:
		return &ValidationError{APIError: base}
	case status == 401, status == 403:
		return &AuthError{APIError: base}
	case status == 404:
		return &NotFoundError{APIError: base}
	case status == 429:
		return &RateLimitError{APIError: base}
	case status >= 500:
		return &ServerError{APIError: base}
	default:
		return base
	}
}

// extractPayloadMessage walks the go-swagger error's Payload field via
// reflection to surface the server's error message instead of the
// framework's "[GET /x][404] body=..." wrapper.
func extractPayloadMessage(err error) string {
	v := reflect.ValueOf(err)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	payload := v.FieldByName("Payload")
	if !payload.IsValid() {
		return ""
	}
	for payload.Kind() == reflect.Ptr {
		if payload.IsNil() {
			return ""
		}
		payload = payload.Elem()
	}
	if payload.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range []string{"Message", "Error"} {
		if f := payload.FieldByName(name); f.IsValid() && f.Kind() == reflect.String {
			if s := f.String(); s != "" {
				return s
			}
		}
	}
	return ""
}
