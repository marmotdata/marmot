package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCreateAPIKeyRequest(t *testing.T, body string, authenticated bool) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/apikeys", strings.NewReader(body))
	if authenticated {
		ctx := context.WithValue(req.Context(), common.UserContextKey, &user.User{ID: "user-1"})
		req = req.WithContext(ctx)
	}
	return req
}

func TestCreateAPIKey_RejectsNegativeExpiry(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()

	h.createAPIKey(rec, newCreateAPIKeyRequest(t, `{"name":"ci","expires_in_days":-1}`, true))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "expires_in_days")
}

func TestCreateAPIKey_RejectsInvalidBody(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()

	h.createAPIKey(rec, newCreateAPIKeyRequest(t, `{not json`, true))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateAPIKey_RequiresAuthentication(t *testing.T) {
	h := &Handler{}
	rec := httptest.NewRecorder()

	h.createAPIKey(rec, newCreateAPIKeyRequest(t, `{"name":"ci"}`, false))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
