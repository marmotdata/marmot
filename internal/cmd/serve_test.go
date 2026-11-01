package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
)

func TestTakeQueryableRunsStripsFlagFromEveryRun(t *testing.T) {
	runs := []plugin.SourceRun{
		{"postgresql": plugin.RawPluginConfig{"host": "db", "queryable": true}},
		{"kafka": plugin.RawPluginConfig{"brokers": "k:9092", "queryable": false}},
		{"s3": plugin.RawPluginConfig{"bucket": "b"}},
	}

	queryable, err := takeQueryableRuns(runs)
	require.NoError(t, err)

	assert.Equal(t, map[string]plugin.RawPluginConfig{"postgresql": {"host": "db"}}, queryable)
	for _, run := range runs {
		for _, cfg := range run {
			assert.NotContains(t, cfg, queryableKey, "Marmot's own setting never reaches the plugin")
		}
	}
}

func TestTakeQueryableRunsRejectsNonBoolean(t *testing.T) {
	_, err := takeQueryableRuns([]plugin.SourceRun{{"postgresql": plugin.RawPluginConfig{"queryable": "yes"}}})
	assert.Error(t, err)
}

func TestServedSourcesNeedsExactlyOneQueryableRun(t *testing.T) {
	_, err := servedSources("pipeline", map[string]plugin.RawPluginConfig{})
	assert.Error(t, err)

	_, err = servedSources("pipeline", map[string]plugin.RawPluginConfig{"a": {}, "b": {}})
	assert.Error(t, err)

	_, err = servedSources("pipeline", map[string]plugin.RawPluginConfig{"not-a-plugin": {}})
	assert.Error(t, err)
}

// The enrolment carries the process's credential and returns what it will trust, so it is only ever made over HTTPS, not even to localhost, where another user on the machine could be listening first.
func TestEnrolmentURLRequiresHTTPS(t *testing.T) {
	_, err := enrolmentURL("https://marmot.example.com")
	assert.NoError(t, err)

	for _, bad := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080", "http://marmot.example.com", "ftp://marmot", "marmot.example.com", ""} {
		_, err := enrolmentURL(bad)
		assert.Error(t, err, bad)
	}
}
