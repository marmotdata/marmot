package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireJWTSession(t *testing.T) {
	for _, tc := range []struct {
		name, method, actual string
		present              bool
		want                 int
	}{
		{name: "api key", method: "local", want: http.StatusForbidden},
		{name: "sso", method: "local", actual: "sso", present: true, want: http.StatusForbidden},
		{name: "local", method: "local", actual: "local", present: true, want: http.StatusNoContent},
		{name: "admin sso", actual: "sso", present: true, want: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			if tc.present {
				r = r.WithContext(context.WithValue(r.Context(), jwtSessionKey{}, tc.actual))
			}
			w := httptest.NewRecorder()
			RequireJWTSession(tc.method)(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d", w.Code, tc.want)
			}
		})
	}
}
