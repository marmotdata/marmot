package runs

import (
	"context"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
)

type createOnlyRepo struct {
	Repository
	created *plugin.Run
}

func (r *createOnlyRepo) Create(_ context.Context, run *plugin.Run) error {
	r.created = run
	return nil
}

func registerSecretPlugin(t *testing.T, id string) {
	t.Helper()
	if _, err := plugin.GetRegistry().Get(id); err != nil {
		require.NoError(t, plugin.GetRegistry().Register(pluginsdk.Meta{
			ID:         id,
			ConfigSpec: []pluginsdk.ConfigField{{Name: "host"}, {Name: "password", Sensitive: true}},
		}, nil))
	}
}

// The scheduler hands StartRunAs the decrypted config the plugin ran with, so the run must not keep its secrets.
func TestStartRunAs_StoresTheConfigWithItsSecretsMasked(t *testing.T) {
	const id = "runsecrets-known"
	registerSecretPlugin(t, id)
	repo := &createOnlyRepo{}

	_, err := (&service{repo: repo}).StartRunAs(context.Background(), "p", id, "someone", "", plugin.RawPluginConfig{"host": "db.internal", "password": "hunter2"})

	require.NoError(t, err)
	assert.Equal(t, "db.internal", repo.created.Config["host"])
	assert.Equal(t, plugin.SensitiveMask, repo.created.Config["password"])
}

func TestStartRunAs_DropsTheConfigOfAnUnknownPlugin(t *testing.T) {
	repo := &createOnlyRepo{}

	_, err := (&service{repo: repo}).StartRunAs(context.Background(), "p", "runsecrets-unregistered", "someone", "", plugin.RawPluginConfig{"password": "hunter2"})

	require.NoError(t, err)
	assert.Nil(t, repo.created.Config)
}

func TestRedactJobRunConfig_KeepsOnlyTheSettingsEveryPluginShares(t *testing.T) {
	run := &JobRun{Config: map[string]interface{}{
		"host": "db.internal", "user": "svc", "password": plugin.SensitiveMask,
		"tags": []string{"pg"}, "filter": map[string]interface{}{"include": []string{"^a"}},
	}}

	RedactJobRunConfig(run)

	assert.Equal(t, map[string]interface{}{"tags": []string{"pg"}, "filter": map[string]interface{}{"include": []string{"^a"}}}, run.Config)
}
