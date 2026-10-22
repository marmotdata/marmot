package schedules

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/pkg/config"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeScheduleRepo holds schedules in memory; methods the handler under test does not call fall through to the nil embedded interface and panic
type fakeScheduleRepo struct {
	runs.ScheduleRepository
	schedules map[string]*runs.Schedule
}

func (f *fakeScheduleRepo) GetSchedule(_ context.Context, id string) (*runs.Schedule, error) {
	s, ok := f.schedules[id]
	if !ok {
		return nil, runs.ErrScheduleNotFound
	}
	return cloneSchedule(s), nil
}

func (f *fakeScheduleRepo) UpdateSchedule(_ context.Context, s *runs.Schedule) error {
	f.schedules[s.ID] = cloneSchedule(s)
	return nil
}

// cloneSchedule copies a schedule the way a database round trip would, so callers mutating what they get back cannot reach what is stored
func cloneSchedule(s *runs.Schedule) *runs.Schedule {
	clone := *s
	clone.Config = make(map[string]interface{}, len(s.Config))
	for k, v := range s.Config {
		clone.Config[k] = v
	}
	return &clone
}

func registerTestPlugin(t *testing.T, id string) {
	t.Helper()
	if _, err := plugin.GetRegistry().Get(id); err == nil {
		return
	}
	require.NoError(t, plugin.GetRegistry().Register(pluginsdk.Meta{
		ID: id,
		ConfigSpec: []pluginsdk.ConfigField{
			{Name: "host"},
			{Name: "password", Sensitive: true},
		},
	}, nil))
}

func newTestHandler(t *testing.T, stored *runs.Schedule) (*Handler, *fakeScheduleRepo, *crypto.Encryptor) {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	enc, err := crypto.NewEncryptor(key)
	require.NoError(t, err)

	require.NoError(t, runs.EncryptScheduleConfig(stored, enc))
	repo := &fakeScheduleRepo{schedules: map[string]*runs.Schedule{stored.ID: stored}}
	h := NewHandler(runs.NewScheduleService(repo), nil, nil, nil, enc, &config.Config{}, true)
	return h, repo, enc
}

func storedSchedule(pluginID string) *runs.Schedule {
	return &runs.Schedule{
		ID:             "sched-1",
		Name:           "warehouse",
		PluginID:       pluginID,
		CronExpression: "0 3 * * *",
		Config:         map[string]interface{}{"host": "db.internal", "password": "hunter2"},
	}
}

func decryptedPassword(t *testing.T, repo *fakeScheduleRepo, enc *crypto.Encryptor) interface{} {
	t.Helper()
	s, err := repo.GetSchedule(context.Background(), "sched-1")
	require.NoError(t, err)
	require.NoError(t, runs.DecryptScheduleConfig(s, enc))
	return s.Config["password"]
}

func doGet(h *Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingestion/schedules/sched-1", nil)
	req.SetPathValue("id", "sched-1")
	rec := httptest.NewRecorder()
	h.getSchedule(rec, req)
	return rec
}

func doUpdate(h *Handler, pluginID string, cfg map[string]interface{}) *httptest.ResponseRecorder {
	body, _ := json.Marshal(UpdateScheduleRequest{
		Name:           "warehouse",
		PluginID:       pluginID,
		Config:         cfg,
		CronExpression: "0 3 * * *",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/ingestion/schedules/sched-1", strings.NewReader(string(body)))
	req.SetPathValue("id", "sched-1")
	rec := httptest.NewRecorder()
	h.updateSchedule(rec, req)
	return rec
}

func TestGetSchedule_MasksSecrets(t *testing.T) {
	registerTestPlugin(t, "schedtest-a")
	h, _, _ := newTestHandler(t, storedSchedule("schedtest-a"))

	rec := doGet(h)
	require.Equal(t, http.StatusOK, rec.Code)

	var got runs.Schedule
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, plugin.SensitiveMask, got.Config["password"])
	assert.Equal(t, "db.internal", got.Config["host"])
	assert.NotContains(t, rec.Body.String(), "hunter2")
}

// An empty config here would be saved over the real one by the edit form, so the request has to fail until plugins have loaded
func TestGetSchedule_PluginsStillLoading(t *testing.T) {
	if plugin.GetLoadState().Ready() {
		t.Skip("plugin load state already marked ready")
	}
	h, _, _ := newTestHandler(t, &runs.Schedule{ID: "sched-1", PluginID: "schedtest-unregistered"})

	rec := doGet(h)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestUpdateSchedule_KeepsMaskedSecret(t *testing.T) {
	registerTestPlugin(t, "schedtest-a")
	h, repo, enc := newTestHandler(t, storedSchedule("schedtest-a"))

	rec := doUpdate(h, "schedtest-a", map[string]interface{}{"host": "db2.internal", "password": plugin.SensitiveMask})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, "hunter2", decryptedPassword(t, repo, enc))
	assert.NotContains(t, rec.Body.String(), "hunter2")
}

func TestUpdateSchedule_ReplacesSecret(t *testing.T) {
	registerTestPlugin(t, "schedtest-a")
	h, repo, enc := newTestHandler(t, storedSchedule("schedtest-a"))

	rec := doUpdate(h, "schedtest-a", map[string]interface{}{"host": "db.internal", "password": "new-password"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, "new-password", decryptedPassword(t, repo, enc))
}

func TestUpdateSchedule_PluginSwitchCarriesNoSecrets(t *testing.T) {
	registerTestPlugin(t, "schedtest-a")
	registerTestPlugin(t, "schedtest-b")
	h, repo, enc := newTestHandler(t, storedSchedule("schedtest-a"))

	rec := doUpdate(h, "schedtest-b", map[string]interface{}{"host": "db.internal", "password": plugin.SensitiveMask})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Nil(t, decryptedPassword(t, repo, enc))
}

func servedSchedule(pluginID string) *runs.Schedule {
	s := storedSchedule(pluginID)
	principalType, principalID := "service_account", "11111111-1111-1111-1111-111111111111"
	s.ServedByType, s.ServedByID = &principalType, &principalID
	s.Config = map[string]interface{}{"host": "db.internal", "password": plugin.SensitiveMask}
	return s
}

// Only the serving process holds a served source's plugin and credentials, so Marmot must not edit it and must never run it.
func TestUpdateSchedule_RefusesServedSource(t *testing.T) {
	registerTestPlugin(t, "schedtest-a")
	stored := servedSchedule("schedtest-a")
	repo := &fakeScheduleRepo{schedules: map[string]*runs.Schedule{stored.ID: stored}}
	h := NewHandler(runs.NewScheduleService(repo), nil, nil, nil, nil, &config.Config{}, true)

	rec := doUpdate(h, "schedtest-a", map[string]interface{}{"host": "attacker.example", "password": "x"})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "db.internal", repo.schedules["sched-1"].Config["host"])
}
