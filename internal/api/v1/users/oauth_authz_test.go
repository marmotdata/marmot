package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUserService implements only what the auth and permission middleware call.
// The embedded interface leaves the rest nil so an unexpected call panics loudly.
type fakeUserService struct {
	user.Service
	canManage bool
	linked    bool
	unlinked  bool
}

func (f *fakeUserService) ValidateAPIKey(_ context.Context, key string) (*user.User, error) {
	if key != "k" {
		return nil, user.ErrInvalidAPIKey
	}
	return &user.User{ID: "attacker", Username: "attacker", Active: true}, nil
}

func (f *fakeUserService) HasPermission(_ context.Context, _, _, _ string) (bool, error) {
	return f.canManage, nil
}

func (f *fakeUserService) LinkOAuthAccount(_ context.Context, _, _, _ string, _ map[string]interface{}) error {
	f.linked = true
	return nil
}

func (f *fakeUserService) UnlinkOAuthAccount(_ context.Context, _, _ string) error {
	f.unlinked = true
	return nil
}

// wire applies a route's middleware to its handler the same way the server does.
func wire(t *testing.T, h *Handler, method, path string) http.HandlerFunc {
	t.Helper()
	for _, r := range h.Routes() {
		if r.Path != path || r.Method != method {
			continue
		}
		handler := r.Handler
		for i := len(r.Middleware) - 1; i >= 0; i-- {
			handler = r.Middleware[i](handler)
		}
		return handler
	}
	t.Fatalf("no route for %s %s", method, path)
	return nil
}

func apiKeyRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-API-Key", "k")
	return r
}

const linkBody = `{"user_id":"admin","provider":"google","provider_user_id":"attacker-sub","user_info":{"email":"e@x.io"}}`

func TestLinkOAuthAccount_RequiresManagePermission(t *testing.T) {
	svc := &fakeUserService{canManage: false}
	h := NewHandler(svc, nil, &config.Config{})

	rec := httptest.NewRecorder()
	wire(t, h, http.MethodPost, "/api/v1/users/oauth/link")(rec, apiKeyRequest(http.MethodPost, "/api/v1/users/oauth/link", linkBody))

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.linked, "a non-admin must not be able to link an identity onto another account")
}

func TestUnlinkOAuthAccount_RequiresManagePermission(t *testing.T) {
	svc := &fakeUserService{canManage: false}
	h := NewHandler(svc, nil, &config.Config{})

	rec := httptest.NewRecorder()
	wire(t, h, http.MethodDelete, "/api/v1/users/oauth/unlink/{id}/{provider}")(
		rec, apiKeyRequest(http.MethodDelete, "/api/v1/users/oauth/unlink/admin/google", ""))

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, svc.unlinked)
}

func TestLinkOAuthAccount_AllowsAdmin(t *testing.T) {
	svc := &fakeUserService{canManage: true}
	h := NewHandler(svc, nil, &config.Config{})

	rec := httptest.NewRecorder()
	wire(t, h, http.MethodPost, "/api/v1/users/oauth/link")(rec, apiKeyRequest(http.MethodPost, "/api/v1/users/oauth/link", linkBody))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, svc.linked)
}
