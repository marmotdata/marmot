package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/mfa"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

func TestTOTPHTTPAuthenticationBoundary(t *testing.T) {
	pool := pgtest.TempDB(t)
	ctx := t.Context()
	users := user.NewService(user.NewPostgresRepository(pool))
	tokens := auth.NewService(auth.NewPostgresRepository(pool), users)
	account, err := users.Create(ctx, user.CreateUserInput{Username: "totp-alice", Name: "TOTP test", Password: "initial-password", RoleNames: []string{"user"}})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE users SET must_change_password=false WHERE id=$1`, account.ID)
	require.NoError(t, err)
	account, err = users.Get(ctx, account.ID)
	require.NoError(t, err)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	encryptor, err := crypto.NewEncryptor(key)
	require.NoError(t, err)
	factors := mfa.NewService(pool, encryptor, "Marmot test")
	common.SetTOTPService(factors)
	t.Cleanup(func() { common.SetTOTPService(nil) })
	cfg := &config.Config{}
	cfg.Auth.TOTP.Enabled = true
	handler := NewHandler(users, tokens, cfg, factors)
	mux := http.NewServeMux()
	serverMux := http.NewServeMux()
	seen := map[string]bool{}
	for _, route := range handler.Routes() {
		if !seen[route.Path] {
			serverMux.HandleFunc(route.Path, route.Handler)
			seen[route.Path] = true
		}

		fn := route.Handler
		for i := len(route.Middleware) - 1; i >= 0; i-- {
			fn = route.Middleware[i](fn)
		}
		mux.HandleFunc(route.Method+" "+route.Path, fn)
	}
	call := func(method, path, token string, body any, want int) map[string]json.RawMessage {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.RemoteAddr = "192.0.2.171:41000"
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		require.Equal(t, want, rec.Code, rec.Body.String())
		out := map[string]json.RawMessage{}
		if rec.Body.Len() > 0 {
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		}
		return out
	}
	text := func(body map[string]json.RawMessage, key string) string {
		var value string
		_ = json.Unmarshal(body[key], &value)
		return value
	}
	login := func(password string) map[string]json.RawMessage {
		return call("POST", "/api/v1/users/login", "", map[string]string{"username": account.Username, "password": password}, 200)
	}
	before := login("initial-password")
	oldToken := text(before, "access_token")
	require.NotEmpty(t, oldToken)
	setup := call("POST", "/api/v1/users/totp/setup", oldToken, map[string]string{"password": "initial-password"}, 200)
	secret := text(setup, "secret")
	require.NotEmpty(t, secret)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	confirmed := call("POST", "/api/v1/users/totp/confirm", oldToken, map[string]string{"code": code}, 200)
	var recovery []string
	require.NoError(t, json.Unmarshal(confirmed["recovery_codes"], &recovery))
	require.Len(t, recovery, 10)
	token := text(confirmed, "access_token")
	require.NotEmpty(t, token)
	call("GET", "/api/v1/users/me", oldToken, nil, 401)
	call("GET", "/api/v1/users/me", token, nil, 200)
	challenge := login("initial-password")
	require.Empty(t, text(challenge, "access_token"))
	require.Equal(t, "true", string(challenge["requires_totp"]))
	pending := text(challenge, "mfa_token")
	_, err = tokens.ValidateToken(ctx, pending)
	require.Error(t, err)
	call("GET", "/api/v1/users/me", pending, nil, 401)
	signedIn := call("POST", "/api/v1/users/login/totp", "", map[string]string{"mfa_token": pending, "code": recovery[0]}, 200)
	require.NotEmpty(t, text(signedIn, "access_token"))
	call("POST", "/api/v1/users/login/totp", "", map[string]string{"mfa_token": pending, "code": recovery[1]}, 401)
	pending = text(login("initial-password"), "mfa_token")
	call("POST", "/api/v1/users/login/totp", "", map[string]string{"mfa_token": pending, "code": recovery[0]}, 401)
	// SSO and API keys keep their existing session paths, independently of local TOTP.
	account, err = users.Get(ctx, account.ID)
	require.NoError(t, err)
	sso, err := tokens.GenerateToken(ctx, account, nil)
	require.NoError(t, err)
	call("GET", "/api/v1/users/me", sso, nil, 200)
	apiKey, err := users.CreateAPIKey(ctx, account.ID, "totp-test", nil)
	require.NoError(t, err)
	call("POST", "/api/v1/users/totp/setup", apiKey.Key, map[string]string{"password": "initial-password"}, 403)
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("X-API-Key", apiKey.Key)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	// Mandatory password change must still leave TOTP pending, never grant a session.
	_, err = pool.Exec(ctx, `UPDATE users SET must_change_password=true WHERE id=$1`, account.ID)
	require.NoError(t, err)
	change := login("initial-password")
	require.Empty(t, text(change, "access_token"))
	require.Equal(t, "true", string(change["requires_password_change"]))
	call("POST", "/api/v1/users/update-password", apiKey.Key, map[string]string{"new_password": "bypass-password"}, 403)
	apiKeyPayload, err := json.Marshal(map[string]string{"new_password": "bypass-password"})
	require.NoError(t, err)
	req = httptest.NewRequest("POST", "/api/v1/users/update-password", bytes.NewReader(apiKeyPayload))
	req.Header.Set("X-API-Key", apiKey.Key)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, 403, rec.Code)
	call("GET", "/api/v1/users/me", text(change, "mfa_token"), nil, 401)
	changed := call("POST", "/api/v1/users/login/password", "", map[string]string{"mfa_token": text(change, "mfa_token"), "new_password": "changed-password"}, 200)
	require.Empty(t, text(changed, "access_token"))
	require.Equal(t, "true", string(changed["requires_totp"]))
	signedIn = call("POST", "/api/v1/users/login/totp", "", map[string]string{"mfa_token": text(changed, "mfa_token"), "code": recovery[1]}, 200)
	token = text(signedIn, "access_token")
	call("GET", "/api/v1/users/me", token, nil, 200)
	call("DELETE", "/api/v1/users/totp/reset/"+account.ID, token, nil, 403)
	_, err = pool.Exec(ctx, `UPDATE users SET must_change_password=false WHERE username='admin'`)
	require.NoError(t, err)
	admin, err := users.GetUserByUsername(ctx, "admin")
	require.NoError(t, err)
	adminToken, err := tokens.GenerateToken(ctx, admin, nil)
	require.NoError(t, err)
	call("DELETE", "/api/v1/users/totp/reset/"+admin.ID, adminToken, nil, 403)
	adminKey, err := users.CreateAPIKey(ctx, admin.ID, "admin-totp-test", nil)
	require.NoError(t, err)
	call("DELETE", "/api/v1/users/totp/reset/"+account.ID, adminKey.Key, nil, 403)
	call("DELETE", "/api/v1/users/totp/reset/"+account.ID, adminToken, nil, 204)
	call("GET", "/api/v1/users/me", token, nil, 401)
	require.NotEmpty(t, text(login("changed-password"), "access_token"))
	// Turning on mandatory enrollment blocks existing local sessions and
	// returns a restricted challenge, while explicit SSO sessions remain valid.
	cfg.Auth.TOTP.Required = true
	account, err = users.Get(ctx, account.ID)
	require.NoError(t, err)
	legacy, err := tokens.GenerateToken(ctx, account, nil)
	require.NoError(t, err)
	call("GET", "/api/v1/users/me", legacy, nil, 401)
	sso, err = tokens.GenerateToken(ctx, account, map[string]interface{}{"auth_method": "sso"})
	require.NoError(t, err)
	call("GET", "/api/v1/users/me", sso, nil, 200)
	enroll := login("changed-password")
	require.Equal(t, "true", string(enroll["requires_totp_enrollment"]))
	require.Empty(t, text(enroll, "access_token"))
	pending = text(enroll, "mfa_token")
	call("GET", "/api/v1/users/me", pending, nil, 401)
	call("POST", "/api/v1/users/login/totp/setup", "", map[string]string{"mfa_token": "wrong"}, 401)
	setup = call("POST", "/api/v1/users/login/totp/setup", "", map[string]string{"mfa_token": pending}, 200)
	code, err = totp.GenerateCode(text(setup, "secret"), time.Now())
	require.NoError(t, err)
	call("POST", "/api/v1/users/login/totp/confirm", "", map[string]string{"mfa_token": pending, "code": "bad"}, 401)
	confirmed = call("POST", "/api/v1/users/login/totp/confirm", "", map[string]string{"mfa_token": pending, "code": code}, 200)
	require.NotEmpty(t, text(confirmed, "access_token"))
	call("POST", "/api/v1/users/login/totp/confirm", "", map[string]string{"mfa_token": pending, "code": code}, 401)
	call("DELETE", "/api/v1/users/totp", text(confirmed, "access_token"), map[string]string{"password": "changed-password", "code": code}, 403)
	// With the feature disabled no second-factor routes exist and legacy login is preserved.
	cfg.Auth.TOTP.Enabled = false
	cfg.Auth.TOTP.Required = false
	off := NewHandler(users, tokens, cfg)
	for _, route := range off.Routes() {
		require.NotContains(t, route.Path, "totp")
	}
	raw := bytes.NewBufferString(`{"username":"totp-alice","password":"changed-password"}`)
	rec = httptest.NewRecorder()
	off.login(rec, httptest.NewRequest("POST", "/api/v1/users/login", raw))
	require.Equal(t, 200, rec.Code)
	var response TokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.NotEmpty(t, response.AccessToken)
	call("POST", "/api/v1/users/update-password", response.AccessToken, map[string]string{"new_password": "bypass-password"}, 403)
	call("POST", "/api/v1/users/change-password", apiKey.Key, map[string]string{"current_password": "changed-password", "new_password": "bypass-password"}, 403)
	call("POST", "/api/v1/users/change-password", response.AccessToken, map[string]string{"current_password": "wrong", "new_password": "next-password"}, 401)
	changedPassword := call("POST", "/api/v1/users/change-password", response.AccessToken, map[string]string{"current_password": "changed-password", "new_password": "next-password"}, 200)
	newToken := text(changedPassword, "access_token")
	require.NotEmpty(t, newToken)
	call("GET", "/api/v1/users/me", response.AccessToken, nil, 401)
	call("GET", "/api/v1/users/me", newToken, nil, 200)
	call("POST", "/api/v1/users/password/require-change/"+account.ID, newToken, nil, 403)
	call("POST", "/api/v1/users/password/require-change/"+account.ID, adminToken, nil, 204)
	call("GET", "/api/v1/users/me", newToken, nil, 401)
	resetLogin := login("next-password")
	require.Equal(t, "true", string(resetLogin["requires_password_change"]))
	call("GET", "/api/v1/users/me", text(resetLogin, "access_token"), nil, 403)
}
