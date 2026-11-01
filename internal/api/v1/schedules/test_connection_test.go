package schedules

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/plugin"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeSource connects only when given the right password, the way a real source would refuse a wrong one.
type probeSource struct{}

func (probeSource) Validate(c plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	if c["host"] == nil || c["host"] == "" {
		return nil, pluginsdk.ValidationErrors{Errors: []pluginsdk.ValidationError{{Field: "host", Message: "host is required"}}}
	}
	return c, nil
}

func (probeSource) Discover(_ context.Context, c plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if c["password"] != "hunter2" {
		return nil, errors.New("password authentication failed")
	}
	name := "orders"
	return &plugin.DiscoveryResult{Assets: []asset.Asset{{Name: &name, Type: "Table"}}}, nil
}

func registerProbePlugin(t *testing.T) {
	t.Helper()
	if _, err := plugin.GetRegistry().Get("schedtest-probe"); err == nil {
		return
	}
	require.NoError(t, plugin.GetRegistry().Register(pluginsdk.Meta{
		ID:         "schedtest-probe",
		ConfigSpec: []pluginsdk.ConfigField{{Name: "host"}, {Name: "password", Sensitive: true}},
	}, probeSource{}))
}

func doTestConnection(h *Handler, req TestConnectionRequest) (*httptest.ResponseRecorder, TestConnectionResponse) {
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/ingestion/test-connection", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	h.testConnection(rec, r)
	var resp TestConnectionResponse
	_ = json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&resp)
	return rec, resp
}

func TestTestConnection(t *testing.T) {
	plugin.GetLoadState().MarkReady()
	registerProbePlugin(t)
	stored := storedSchedule("schedtest-probe")
	h, _, _ := newTestHandler(t, stored)

	t.Run("settings that fail validation never start the plugin", func(t *testing.T) {
		rec, resp := doTestConnection(h, TestConnectionRequest{PluginID: "schedtest-probe", Config: map[string]interface{}{}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.False(t, resp.Valid)
		assert.Nil(t, resp.Result)
		require.Len(t, resp.Errors, 1)
		assert.Equal(t, "host", resp.Errors[0].Field)
	})

	t.Run("a refused connection reports the plugin's error", func(t *testing.T) {
		rec, resp := doTestConnection(h, TestConnectionRequest{PluginID: "schedtest-probe", Config: map[string]interface{}{"host": "db", "password": "wrong"}})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NotNil(t, resp.Result)
		assert.False(t, resp.Result.Succeeded)
		assert.Equal(t, "password authentication failed", resp.Result.Error)
	})

	t.Run("an edited pipeline is tested with its stored secret", func(t *testing.T) {
		rec, resp := doTestConnection(h, TestConnectionRequest{
			PluginID:   "schedtest-probe",
			ScheduleID: stored.ID,
			Config:     map[string]interface{}{"host": "db", "password": plugin.SensitiveMask},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NotNil(t, resp.Result)
		assert.True(t, resp.Result.Succeeded)
		assert.Equal(t, 1, resp.Result.Tables)
		assert.NotContains(t, rec.Body.String(), "hunter2")
	})

	t.Run("unknown plugin", func(t *testing.T) {
		rec, _ := doTestConnection(h, TestConnectionRequest{PluginID: "schedtest-nope"})
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
