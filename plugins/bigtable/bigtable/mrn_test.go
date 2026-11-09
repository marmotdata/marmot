package bigtable

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bigtable table names are only unique within an instance, so a table is addressed instance.table and an instance by its own id.

func TestInstanceMRN_IsTheInstanceID(t *testing.T) {
	assert.Equal(t, "mrn://asset/instance/bigtable/prod-metrics", assetMRN(&Config{}, "Instance", "prod-metrics"))
}

func TestTableMRN_IsQualifiedByItsInstance(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/bigtable/prod-metrics.events", assetMRN(cfg, "Table", "prod-metrics", "events"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "prod-metrics", "events"), assetMRN(cfg, "Table", "staging", "events"))
}

func TestAssetMRN_CarriesTheInstanceOption(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "prod-metrics", "events")
	assert.Equal(t, "mrn://asset/table/bigtable@prod-eu/prod-metrics.events", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestInstanceAsset_NameAndMRN(t *testing.T) {
	s := &Source{config: &Config{ProjectID: "analytics"}}

	a := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 3, 0)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "mrn://asset/instance/bigtable/prod-metrics", *a.MRN)
	assert.Equal(t, "Instance", a.Type)
	assert.Equal(t, []string{"Bigtable"}, a.Providers)
	assert.Equal(t, "prod-metrics", *a.Name, "the name people read is the instance's own id")
}

func TestTableMRN_KeepsTheProjectOutOfTheName(t *testing.T) {
	// Two projects with the same instance id would collide, so the zone option is what tells them apart; plugins/bigquery settles it the same way with one required project per pipeline.
	s := &Source{config: &Config{ProjectID: "analytics"}}

	a := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 1, 0)

	assert.NotContains(t, *a.MRN, "analytics")
	assert.NotContains(t, assetMRN(s.config, "Table", "prod-metrics", "events"), "analytics")
}

func TestAssetMRN_LowercasesTheName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/bigtable/prod-metrics.userevents", assetMRN(&Config{}, "Table", "Prod-Metrics", "UserEvents"))
}
