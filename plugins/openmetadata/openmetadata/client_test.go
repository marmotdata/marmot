package openmetadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListAllDropsFieldsTheServerRejects covers an OpenMetadata server
// that 400s on `domains` and `dataProducts`: the client must drop each
// and retry rather than fail the whole list call.
func TestListAllDropsFieldsTheServerRejects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := r.URL.Query().Get("fields")
		w.Header().Set("Content-Type", "application/json")

		// Real OpenMetadata rejects one unknown field at a time.
		for _, unknown := range []string{"domains", "dataProducts"} {
			if strings.Contains(fields, unknown) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"code":400,"message":"Invalid field name %s"}`, unknown)
				return
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   []map[string]interface{}{{"id": "1", "name": "one"}},
			"paging": map[string]interface{}{"total": 1},
		})
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	entities, err := listAll[map[string]interface{}](t.Context(), c, "/v1/databases",
		"owners,tags,domains,dataProducts", 100, false)
	require.NoError(t, err)
	require.Len(t, entities, 1)
	assert.Equal(t, "one", entities[0]["name"])

	c.fieldMu.Lock()
	cached := c.fieldCache["/v1/databases|owners,tags,domains,dataProducts"]
	c.fieldMu.Unlock()
	assert.Equal(t, "owners,tags", cached)
}

// TestResolveFieldsCachesPerEndpoint asserts the client probes each
// endpoint once and reuses the result. Without caching, a run over
// 500k assets would pay the negotiation cost on every page.
func TestResolveFieldsCachesPerEndpoint(t *testing.T) {
	var probes int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := r.URL.Query().Get("fields")
		if r.URL.Query().Get("limit") == "1" {
			atomic.AddInt32(&probes, 1)
		}
		if strings.Contains(fields, "domains") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":400,"message":"Invalid field name domains"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":   []map[string]interface{}{},
			"paging": map[string]interface{}{},
		})
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	for i := 0; i < 3; i++ {
		_, err := listAll[map[string]interface{}](t.Context(), c, "/v1/tables",
			"columns,domains", 100, false)
		require.NoError(t, err)
	}

	// The first call probes twice (initial + retry without domains).
	// The next two calls must hit the cache and probe zero times.
	assert.Equal(t, int32(2), atomic.LoadInt32(&probes))
}

// TestResolveFieldsSurfacesNonFieldErrors makes sure 400s that are not
// about a bad field name bubble up to the caller.
func TestResolveFieldsSurfacesNonFieldErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"Malformed request"}`))
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	_, err := listAll[map[string]interface{}](t.Context(), c, "/v1/databases",
		"owners,tags", 100, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Malformed request")
}

// TestListOptionalTreatsWrapped404AsMissing covers older OpenMetadata
// releases that wrap a missing endpoint in a 500 with `HTTP 404 Not
// Found` in the body — must be treated the same as a real 404.
func TestListOptionalTreatsWrapped404AsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"responseMessage":"An exception with message [HTTP 404 Not Found] was thrown while processing request.","errorType":"UNHANDLED_SERVER_EXCEPTION"}`))
	}))
	t.Cleanup(server.Close)

	c := newClient(server.URL, "t", 5*time.Second, false)
	_, ok, err := listOptional[map[string]interface{}](t.Context(), c, "/v1/drives/directories",
		"owners,tags", 100, false)
	require.NoError(t, err)
	assert.False(t, ok, "endpoint should be reported as absent so callers skip it")
}

func TestDropField(t *testing.T) {
	cases := []struct {
		in, name, want string
	}{
		{"a,b,c", "b", "a,c"},
		{"a,b,c", "a", "b,c"},
		{"a,b,c", "c", "a,b"},
		{"a, b ,c", "b", "a,c"},
		{"a", "a", ""},
		{"a,b", "missing", "a,b"},
	}
	for _, tc := range cases {
		t.Run(tc.in+"-"+tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dropField(tc.in, tc.name))
		})
	}
}
