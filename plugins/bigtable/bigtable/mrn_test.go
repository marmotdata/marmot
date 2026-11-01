package bigtable

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Bigtable table names are only unique within an instance, so a table is
// addressed instance.table and an instance by its own id.

func TestInstanceMRN_IsTheInstanceID(t *testing.T) {
	assert.Equal(t, "mrn://instance/bigtable/prod-metrics",
		assetMRN("Instance", "prod-metrics"))
}

func TestTableMRN_IsQualifiedByItsInstance(t *testing.T) {
	assert.Equal(t, "mrn://table/bigtable/prod-metrics.events",
		assetMRN("Table", tableName("prod-metrics", "events")))
}

func TestInstanceAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The one asset this plugin builds without a live connection, so the
	// one place the agreement can be checked against real output: the
	// server rebuilds identity from Type, Providers[0] and Name, so the
	// MRN has to be exactly mrn.New over those three.
	s := &Source{config: &Config{ProjectID: "analytics"}}

	a := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 3, 0)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestInstanceMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Instance", "prod-metrics")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so the dot in the name has to survive
	// byte-identical.
	original := assetMRN("Table", tableName("prod-metrics", "events"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a BigTable service onto provider
	// "Bigtable" with schema-qualified table names, the instance sitting at
	// the schema level. This plugin has to produce the same MRN or the day
	// it takes over, those assets are stranded and a second set appears.
	assert.Equal(t, "mrn://table/bigtable/prod-metrics.events",
		mrn.New("Table", "Bigtable", "prod-metrics.events"))
	assert.Equal(t, mrn.New("Table", "Bigtable", "prod-metrics.events"),
		assetMRN("Table", tableName("prod-metrics", "events")))
}

func TestInstanceAsset_MatchesTheContainerAnOpenMetadataImportProduces(t *testing.T) {
	// OpenMetadata groups a BigTable service's tables under a container of
	// type Instance named by the instance alone, which is why the project
	// stays out of every name here.
	s := &Source{config: &Config{ProjectID: "analytics"}}

	a := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 3, 0)

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://instance/bigtable/prod-metrics", *a.MRN)
	assert.Equal(t, "Instance", a.Type)
	assert.Equal(t, []string{"Bigtable"}, a.Providers)
	require.NotNil(t, a.Name)
	assert.Equal(t, "prod-metrics", *a.Name, "the name people read is the instance's own id")
}

func TestTableMRN_KeepsTheProjectOutOfTheName(t *testing.T) {
	// Two projects with the same instance id would collide, but the
	// alternative loses the agreement with OpenMetadata and leaves the
	// Instance asset no longer a prefix of its tables. plugins/bigquery
	// settles it the same way: one required project per pipeline.
	s := &Source{config: &Config{ProjectID: "analytics"}}

	a := s.instanceAsset(instanceDetail{ID: "prod-metrics"}, 1, 0)

	assert.NotContains(t, *a.MRN, "analytics")
	assert.NotContains(t, assetMRN("Table", tableName("prod-metrics", "events")), "analytics")
}

func TestTableMRN_IsPrefixedByItsInstanceMRN(t *testing.T) {
	// Unlike the bare-name database plugins, an instance's name really is
	// the front of its tables' names here. The Contents tree is still built
	// from the CONTAINS edges Discover emits, not by matching prefixes.
	instance := assetMRN("Instance", "prod-metrics")
	table := assetMRN("Table", tableName("prod-metrics", "events"))

	assert.Equal(t, "mrn://instance/bigtable/prod-metrics", instance)
	assert.Contains(t, table, "prod-metrics.")
}

func TestAssetMRN_LowercasesTheName(t *testing.T) {
	// mrn.New lowercases, so a mixed-case instance id still resolves. The
	// asset's Name keeps the original spelling for people to read.
	assert.Equal(t, "mrn://table/bigtable/prod-metrics.userevents",
		assetMRN("Table", tableName("Prod-Metrics", "UserEvents")))
}
