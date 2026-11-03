package runs

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
)

func TestMaskJobRunConfig_MasksSecretsOfAKnownPlugin(t *testing.T) {
	const id = "jobrunmask-known"
	if _, err := plugin.GetRegistry().Get(id); err != nil {
		require.NoError(t, plugin.GetRegistry().Register(pluginsdk.Meta{
			ID:         id,
			ConfigSpec: []pluginsdk.ConfigField{{Name: "host"}, {Name: "password", Sensitive: true}},
		}, nil))
	}
	run := &JobRun{SourceName: id, Config: map[string]interface{}{"host": "db.internal", "password": "hunter2"}}

	(&SchedulePostgresRepository{}).maskJobRunConfig(run)

	assert.Equal(t, "db.internal", run.Config["host"])
	assert.Equal(t, plugin.SensitiveMask, run.Config["password"])
}

// Without the plugin's spec, as while plugins are still loading, nothing says which fields are secret, so none of the config is returned.
func TestMaskJobRunConfig_DropsTheConfigOfAnUnknownPlugin(t *testing.T) {
	run := &JobRun{SourceName: "jobrunmask-unregistered", Config: map[string]interface{}{"password": "hunter2"}}

	(&SchedulePostgresRepository{}).maskJobRunConfig(run)

	assert.Nil(t, run.Config)
}
