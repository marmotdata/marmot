package common

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

// RequestIDHeader carries the request reference both ways: a proxy may set it on the way in and every API response echoes it.
const RequestIDHeader = "X-Request-Id"

type requestIDKey struct{}

// inboundRequestID bounds what a caller can put in the header, since it is written back into responses and logs.
var inboundRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// WithRequestID gives the request a reference, reusing a well-formed one from a proxy so a support ticket can be followed across both, and sets it on the response before the handler writes anything so RespondError can include it.
func WithRequestID(w http.ResponseWriter, r *http.Request) (*http.Request, string) {
	id := r.Header.Get(RequestIDHeader)
	if !inboundRequestID.MatchString(id) {
		id = newRequestID()
	}
	w.Header().Set(RequestIDHeader, id)
	return r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)), id
}

// RequestIDFromContext returns the reference WithRequestID gave the request, or "" outside one.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
