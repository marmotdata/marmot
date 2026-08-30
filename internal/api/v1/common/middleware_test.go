package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

func TestPasswordChangeGate(t *testing.T) {
	tests := []struct {
		name string
		user *user.User
		path string
		want int
	}{
		{name: "pending change is refused everywhere else", user: &user.User{MustChangePassword: true}, path: "/api/v1/assets", want: http.StatusForbidden},
		{name: "pending change may change the password", user: &user.User{MustChangePassword: true}, path: "/api/v1/users/update-password", want: http.StatusOK},
		{name: "settled user passes", user: &user.User{}, path: "/api/v1/assets", want: http.StatusOK},
		{name: "non-user principals pass", user: nil, path: "/api/v1/assets", want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := passwordChangeGate(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			r := httptest.NewRequest(http.MethodPost, tt.path, nil)
			if tt.user != nil {
				r = r.WithContext(context.WithValue(r.Context(), UserContextKey, tt.user))
			}
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != tt.want {
				t.Fatalf("%s %s = %d, want %d", http.MethodPost, tt.path, w.Code, tt.want)
			}
		})
	}
}

// The gate has to sit inside WithAuth rather than on individual routes: every
// credential of the user must hit it, however the request authenticated.
func TestWithAuthPasswordChangePending(t *testing.T) {
	userSvc := &mockUserService{
		validateAPIKeyFn: func(_ context.Context, _ string) (*user.User, error) {
			return &user.User{ID: "u1", Active: true, MustChangePassword: true}, nil
		},
	}
	handler := WithAuth(userSvc, &mockAuthService{}, &config.Config{})(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for path, want := range map[string]int{
		"/api/v1/assets":                http.StatusForbidden,
		"/api/v1/users/update-password": http.StatusOK,
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-API-Key", "key")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != want {
			t.Fatalf("POST %s = %d, want %d", path, rec.Code, want)
		}
	}
}
