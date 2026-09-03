package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ServeMux matches paths exactly, so a case-shifted API path matches no route and
// falls through to the SPA, answering HTML with a 200 where a client expects JSON.
func TestAPIPaths(t *testing.T) {
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html>"))
	})
	handler := apiPaths(spa)

	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/API/v1/users", http.StatusNotFound},
		{"/Api/v1/plugins", http.StatusNotFound},
		{"/API/", http.StatusNotFound},
		// Correctly cased paths belong to the mux, which has its own /api/ handler.
		{"/api/v1/users", http.StatusOK},
		// Everything outside /api is the SPA's, including client-side routes.
		{"/admin", http.StatusOK},
		{"/", http.StatusOK},
		{"/apidocs", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.status {
			t.Errorf("%s: got %d, want %d", tc.path, rec.Code, tc.status)
		}
		if tc.status == http.StatusNotFound && !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s: want a JSON error body, got %q", tc.path, rec.Body.String())
		}
	}
}
