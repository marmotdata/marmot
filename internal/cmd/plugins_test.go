package cmd

import (
	"testing"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/stretchr/testify/assert"
)

func TestPluginStatusLine(t *testing.T) {
	// An open advisory is the most urgent thing to show, even if a plain
	// update is also available.
	advisoryAndUpdate := &marmot.PluginVersionStatus{
		UpdateAvailable: true,
		Advisories:      []*marmot.PluginAdvisory{{ID: "MPA-1"}, {ID: "MPA-2"}},
	}
	assert.Equal(t, "security: MPA-1, MPA-2", pluginStatusLine(advisoryAndUpdate))

	update := &marmot.PluginVersionStatus{UpdateAvailable: true}
	assert.Equal(t, "update available", pluginStatusLine(update))

	errored := &marmot.PluginVersionStatus{Error: "install failed"}
	assert.Equal(t, "error: install failed", pluginStatusLine(errored))

	assert.Equal(t, "ok", pluginStatusLine(&marmot.PluginVersionStatus{}))
}

func TestPluginsHeaderLine(t *testing.T) {
	disabled := &marmot.PluginUpdatesStatus{AutoUpdate: "off", Enabled: false}
	assert.Equal(t, "Auto-update: off  |  Plugin index: disabled\n", pluginsHeaderLine(disabled))

	// An empty policy means the server left it at its default.
	neverChecked := &marmot.PluginUpdatesStatus{Enabled: true, Index: &marmot.PluginIndexStatus{}}
	assert.Equal(t, "Auto-update: security  |  Plugin index last checked: never\n", pluginsHeaderLine(neverChecked))

	withError := &marmot.PluginUpdatesStatus{
		AutoUpdate: "patch",
		Enabled:    true,
		Index:      &marmot.PluginIndexStatus{CheckedAt: "2026-11-01T00:00:00Z", Error: "signature mismatch"},
	}
	assert.Equal(t, "Auto-update: patch  |  Plugin index last checked: 2026-11-01T00:00:00Z  (error: signature mismatch)\n", pluginsHeaderLine(withError))
}

func TestFindPluginStatus(t *testing.T) {
	status := &marmot.PluginUpdatesStatus{Plugins: []*marmot.PluginVersionStatus{{ID: "postgresql"}, {ID: "trino"}}}

	assert.Same(t, status.Plugins[1], findPluginStatus(status, "trino"))
	assert.Nil(t, findPluginStatus(status, "mysql"))
}

func TestDashIfEmpty(t *testing.T) {
	assert.Equal(t, "-", dashIfEmpty(""))
	assert.Equal(t, "0.3.1", dashIfEmpty("0.3.1"))
}
