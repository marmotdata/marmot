package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The reference has to reach the error body as well as the header, because the UI error page only sees the body, and a proxy's well-formed id is kept so one reference covers both hops.
func TestWithRequestID(t *testing.T) {
	t.Run("generated and echoed in errors", func(t *testing.T) {
		w := httptest.NewRecorder()
		r, id := WithRequestID(w, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))
		if id == "" || RequestIDFromContext(r.Context()) != id {
			t.Fatalf("id %q not carried on the context", id)
		}
		RespondError(w, http.StatusInternalServerError, "boom")
		if got := w.Header().Get(RequestIDHeader); got != id {
			t.Fatalf("header %q, want %q", got, id)
		}
		var body ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.RequestID != id {
			t.Fatalf("body request_id %q, want %q", body.RequestID, id)
		}
	})

	t.Run("well-formed inbound id kept", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, "proxy-abc-12345")
		_, id := WithRequestID(httptest.NewRecorder(), req)
		if id != "proxy-abc-12345" {
			t.Fatalf("id %q, want the proxy's", id)
		}
	})

	t.Run("hostile inbound id replaced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(RequestIDHeader, "<script>alert(1)</script>")
		_, id := WithRequestID(httptest.NewRecorder(), req)
		if id == "<script>alert(1)</script>" || len(id) != 16 {
			t.Fatalf("id %q should have been regenerated", id)
		}
	})
}
