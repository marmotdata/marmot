package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
)

func TestExpandRunsEnv(t *testing.T) {
	t.Setenv("PG_PASSWORD", "s3cret")
	t.Setenv("KAFKA_USER", "marmot")

	runs := []plugin.SourceRun{{
		"postgresql": plugin.RawPluginConfig{
			"host":     "db",
			"password": "${PG_PASSWORD}",
			"literal":  "price is $5 and $PG_PASSWORD stays",
			"port":     5432,
			"authentication": map[string]interface{}{
				"username": "${KAFKA_USER}-svc",
			},
			"brokers": []interface{}{"${KAFKA_USER}:9092", 7},
		},
	}}

	require.NoError(t, expandRunsEnv(runs))

	cfg := runs[0]["postgresql"]
	assert.Equal(t, "s3cret", cfg["password"])
	assert.Equal(t, "price is $5 and $PG_PASSWORD stays", cfg["literal"])
	assert.Equal(t, 5432, cfg["port"])
	assert.Equal(t, "marmot-svc", cfg["authentication"].(map[string]interface{})["username"])
	assert.Equal(t, []interface{}{"marmot:9092", 7}, cfg["brokers"])
}

func TestExpandRunsEnvFailsOnUnsetVariable(t *testing.T) {
	runs := []plugin.SourceRun{{"postgresql": plugin.RawPluginConfig{"password": "${MARMOT_TEST_UNSET_VARIABLE}"}}}

	err := expandRunsEnv(runs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARMOT_TEST_UNSET_VARIABLE")
	assert.Contains(t, err.Error(), "postgresql: password")
}

func TestExpandRunsEnvAllowsEmptyValue(t *testing.T) {
	t.Setenv("EMPTY", "")
	runs := []plugin.SourceRun{{"postgresql": plugin.RawPluginConfig{"password": "${EMPTY}"}}}

	require.NoError(t, expandRunsEnv(runs))
	assert.Equal(t, "", runs[0]["postgresql"]["password"])
}

// A value containing YAML or another reference is inserted verbatim, never parsed or expanded again.
func TestExpandRunsEnvIsNotRecursive(t *testing.T) {
	t.Setenv("INJECT", "x\nadmin: true\n${PG_PASSWORD}")
	t.Setenv("PG_PASSWORD", "leak")
	runs := []plugin.SourceRun{{"postgresql": plugin.RawPluginConfig{"password": "${INJECT}"}}}

	require.NoError(t, expandRunsEnv(runs))
	cfg := runs[0]["postgresql"]
	assert.Equal(t, "x\nadmin: true\n${PG_PASSWORD}", cfg["password"])
	assert.NotContains(t, cfg, "admin")
}
