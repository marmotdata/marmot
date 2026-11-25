package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadsWaitOutASecondaryRateLimit(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			fail(w, http.StatusForbidden, "You have exceeded a secondary rate limit")
			return
		}
		writeJSON(w, 200, map[string]any{"login": "acme"})
	}))
	t.Cleanup(srv.Close)
	c, err := newClient(srv.URL, testToken, "test")
	require.NoError(t, err)

	var a account
	require.NoError(t, c.do(context.Background(), http.MethodGet, "/users/acme", nil, nil, &a))
	assert.Equal(t, "acme", a.Login)
	assert.EqualValues(t, 2, calls.Load())

	calls.Store(0)
	err = c.do(context.Background(), http.MethodPost, "/repos/acme/x/pulls", nil, map[string]any{}, nil)
	assert.Equal(t, http.StatusForbidden, statusOf(err), "a write is never retried")
	assert.EqualValues(t, 1, calls.Load())
}

func TestRateLimitWait(t *testing.T) {
	resp := func(status int, h map[string]string) *http.Response {
		r := &http.Response{StatusCode: status, Header: http.Header{}}
		for k, v := range h {
			r.Header.Set(k, v)
		}
		return r
	}
	assert.Zero(t, rateLimitWait(resp(403, nil)), "a plain 403 is a permission error")
	assert.Zero(t, rateLimitWait(resp(403, map[string]string{"Retry-After": "3600"})), "too long to wait")
	assert.Positive(t, rateLimitWait(resp(429, nil)))
	assert.Positive(t, rateLimitWait(resp(403, map[string]string{"Retry-After": "2"})))
}
