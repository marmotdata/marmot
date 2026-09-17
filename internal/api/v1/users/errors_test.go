package users

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bare and wrapped, because wrapped is the case that used to fall through to
// 500: core/user attaches context to almost every sentinel it returns.
func TestRespondUserErrorMapsSentinels(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"not found", user.ErrUserNotFound, http.StatusNotFound},
		{"already exists", user.ErrAlreadyExists, http.StatusConflict},
		{"reserved username", user.ErrReservedUsername, http.StatusBadRequest},
		{"password required", user.ErrPasswordRequired, http.StatusBadRequest},
		{"invalid input", user.ErrInvalidInput, http.StatusBadRequest},
		{"role not found", user.ErrRoleNotFound, http.StatusBadRequest},
		{"cannot delete self", user.ErrCannotDeleteSelf, http.StatusConflict},
		{"cannot delete admin", user.ErrCannotDeleteAdmin, http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for label, err := range map[string]error{
				"bare":    tc.err,
				"wrapped": fmt.Errorf("creating user: %w", tc.err),
			} {
				rec := httptest.NewRecorder()
				require.True(t, respondUserError(rec, err), "%s: went unhandled", label)
				assert.Equal(t, tc.want, rec.Code, label)
			}
		})
	}
}

// An error the caller could not have avoided has to stay a 500, or a database
// outage would be reported as a bad request.
func TestRespondUserErrorLeavesUnknownErrorsAlone(t *testing.T) {
	rec := httptest.NewRecorder()
	assert.False(t, respondUserError(rec, fmt.Errorf("connection refused")))
	assert.Equal(t, http.StatusOK, rec.Code, "nothing should have been written")
}

// The reported bug: a create missing required fields answered 500 "Failed to
// create user", naming the operation rather than the problem.
func TestRespondUserErrorNamesTheMissingFields(t *testing.T) {
	svc := user.NewService(nil)
	// Validation runs before the repository is touched, so a nil repo is safe
	// and keeps this a test of the boundary rather than of storage.
	_, err := svc.Create(t.Context(), user.CreateUserInput{Username: "alice", Name: "Alice"})
	require.Error(t, err)

	rec := httptest.NewRecorder()
	require.True(t, respondUserError(rec, err))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body struct {
		Error  string `json:"error"`
		Fields []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	named := map[string]string{}
	for _, f := range body.Fields {
		named[f.Field] = f.Message
	}
	// Named as the request body spells them, not as Go spells them.
	assert.Contains(t, named, "password")
	assert.Contains(t, named, "role_names")
	assert.NotContains(t, named, "RoleNames")
}

// Field detail must name fields and rules, never submitted values: a create
// carries a password, which must not reach logs or proxies.
func TestRespondUserErrorDoesNotEchoSubmittedValues(t *testing.T) {
	const secret = "hunter2"
	svc := user.NewService(nil)
	_, err := svc.Create(t.Context(), user.CreateUserInput{
		Username: "ab", // below min=3
		Password: secret,
	})
	require.Error(t, err)

	rec := httptest.NewRecorder()
	require.True(t, respondUserError(rec, err))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	for _, submitted := range []string{secret, "ab"} {
		assert.NotContains(t, rec.Body.String(), submitted)
		assert.NotContains(t, err.Error(), submitted)
	}
}
