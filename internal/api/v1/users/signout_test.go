package users

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A token copied before sign-out is refused when replayed after it, on reads and writes alike, while the person's other sessions keep working.
func TestSignOutRevokesTheToken(t *testing.T) {
	db := pgtest.TempDB(t)
	ctx := context.Background()

	userSvc := user.NewService(user.NewPostgresRepository(db))
	authSvc := auth.NewService(auth.NewPostgresRepository(db), userSvc,
		auth.WithRevocationStore(auth.NewPostgresRevocationStore(db)))
	h := NewHandler(userSvc, authSvc, &config.Config{})

	var userID string
	require.NoError(t, db.QueryRow(ctx,
		`INSERT INTO users (username, name, active) VALUES ('kim', 'Kim', true) RETURNING id`).Scan(&userID))
	u, err := userSvc.Get(ctx, userID)
	require.NoError(t, err)
	token, err := authSvc.GenerateToken(ctx, u, nil)
	require.NoError(t, err)
	otherTab, err := authSvc.GenerateToken(ctx, u, nil)
	require.NoError(t, err)

	routes := map[string]http.HandlerFunc{
		"POST /api/v1/users/sign-out":   wire(t, h, http.MethodPost, "/api/v1/users/sign-out"),
		"GET /api/v1/users/me":          wire(t, h, http.MethodGet, "/api/v1/users/me"),
		"PUT /api/v1/users/preferences": wire(t, h, http.MethodPut, "/api/v1/users/preferences"),
	}
	do := func(method, path, bearer, body string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		routes[method+" "+path](rec, req)
		return rec.Code
	}
	writePrefs := `{"preferences":{"theme":"dark"}}`

	require.Equal(t, 200, do("GET", "/api/v1/users/me", token, ""))
	require.Equal(t, 200, do("PUT", "/api/v1/users/preferences", token, writePrefs))

	assert.Equal(t, 401, do("POST", "/api/v1/users/sign-out", "", ""), "nothing to sign out")
	assert.Equal(t, 204, do("POST", "/api/v1/users/sign-out", token, ""))
	assert.Equal(t, 401, do("GET", "/api/v1/users/me", token, ""), "a signed-out token is refused although it has not expired")
	assert.Equal(t, 401, do("PUT", "/api/v1/users/preferences", token, writePrefs), "on writes too")
	assert.Equal(t, 204, do("POST", "/api/v1/users/sign-out", token, ""), "signing out twice is not an error")
	assert.Equal(t, 204, do("POST", "/api/v1/users/sign-out", "bogus", ""), "a token that never verified has nothing to revoke")
	assert.Equal(t, 200, do("GET", "/api/v1/users/me", otherTab, ""), "only the token signed out is revoked, not the person's other sessions")

	var revoked int
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM revoked_tokens`).Scan(&revoked))
	assert.Equal(t, 1, revoked, "invalid tokens are not stored")

	// A deleted account's token can still be signed out, since the endpoint is not behind the account lookup.
	_, err = db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	require.NoError(t, err)
	assert.Equal(t, 204, do("POST", "/api/v1/users/sign-out", otherTab, ""))
	require.NoError(t, db.QueryRow(ctx, `SELECT count(*) FROM revoked_tokens`).Scan(&revoked))
	assert.Equal(t, 2, revoked)

	// A failed sign-out lookup fails closed: a server error, not a 401 and never a 200.
	failing := auth.NewService(auth.NewPostgresRepository(db), userSvc, auth.WithRevocationStore(brokenRevocations{}))
	broken := wire(t, NewHandler(userSvc, failing, &config.Config{}), http.MethodGet, "/api/v1/users/me")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+otherTab)
	rec := httptest.NewRecorder()
	broken(rec, req)
	assert.Equal(t, 500, rec.Code)
}

type brokenRevocations struct{}

func (brokenRevocations) Revoke(context.Context, []byte, time.Time) error {
	return errors.New("database down")
}

func (brokenRevocations) Revoked(context.Context, []byte) (bool, error) {
	return false, errors.New("database down")
}
