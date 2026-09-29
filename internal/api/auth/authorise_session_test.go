package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	coreauth "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
)

func TestHandleAuthorizeCompleteRejectsRevokedFactorSession(t *testing.T) {
	h := newAuthorizeHandler()
	h.authService = &mockAuthService{validateTokenFn: func(_ context.Context, _ string) (*coreauth.Claims, error) {
		return &coreauth.Claims{SessionEpoch: "0", RegisteredClaims: jwt.RegisteredClaims{Subject: "u-1"}}, nil
	}}
	now := time.Now()
	h.userService = &mockUserService{getFn: func(_ context.Context, _ string) (*user.User, error) {
		return &user.User{ID: "u-1", Username: "alice", Active: true, SessionsInvalidatedAt: &now}, nil
	}}
	r := httptest.NewRequest(http.MethodPost, "/oauth/authorize/complete", nil)
	r.Header.Set("Authorization", "Bearer stale-but-signed")
	w := httptest.NewRecorder()
	h.handleAuthorizeComplete(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session granted OAuth code: %d", w.Code)
	}
}
