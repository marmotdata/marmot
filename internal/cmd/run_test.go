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

// API bodies are account data, so no browser or shared cache may keep one after sign-out. That has to hold for the 404 fallback, a bare 204 and the case-shifted 404 too, while the SPA keeps its own caching.
func TestSecurityHeadersAPICaching(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("POST /api/v1/users/sign-out", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<!doctype html>"))
	})
	handler := securityHeaders(apiPaths(mux), nil)

	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/v1/users/me", http.StatusOK},
		{http.MethodPost, "/api/v1/users/sign-out", http.StatusNoContent},
		{http.MethodGet, "/api/v1/nope", http.StatusNotFound},
		{http.MethodGet, "/API/v1/users/me", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.status {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s %s: Cache-Control = %q, want no-store", tc.method, tc.path, got)
		}
		if got := rec.Header().Get("Vary"); got != "Authorization" {
			t.Errorf("%s %s: Vary = %q, want Authorization", tc.method, tc.path, got)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Errorf("the SPA got Cache-Control %q; it sets its own caching", got)
	}
}
