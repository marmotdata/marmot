package nifi

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pipeline is named by its path from the root group and a task by that
// path plus the processor name. mrn.New folds the slashes and spaces into
// hyphens, which is why the qualifier lives in the name rather than in a
// separate field: the server rebuilds the MRN from the name alone.

func TestPipelineMRN_IsThePathFromTheRoot(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/nifi/nifi-flow-ingest", assetMRN("Pipeline", "NiFi Flow/Ingest"))
}

func TestRootPipelineMRN_IsTheRootGroupsName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/nifi/nifi-flow", assetMRN("Pipeline", "NiFi Flow"))
}

func TestTaskMRN_IsThePipelinePathAndProcessorName(t *testing.T) {
	assert.Equal(t, "mrn://task/nifi/nifi-flow-ingest-generateflowfile", assetMRN("Task", "NiFi Flow/Ingest/GenerateFlowFile"))
}

func TestTopicMRN_IsTheKafkaPluginsIdentity(t *testing.T) {
	// The topic an edge points at has to be the asset the Kafka plugin
	// creates, or the edge lands on a NiFi-flavoured duplicate.
	assert.Equal(t, "mrn://topic/kafka/orders-events", nativeMRN("Topic", "Kafka", "orders-events"))
}

func TestBucketMRN_IsTheS3PluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://bucket/s3/marmot-landing", nativeMRN("Bucket", "S3", "marmot-landing"))
}

func TestTableMRN_IsThePostgreSQLPluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/orders", nativeMRN("Table", "PostgreSQL", "orders"))
}

func TestTableMRN_IsTheSQLServerPluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://table/sql server/shop.dbo.orders", nativeMRN("Table", "SQL Server", "shop.dbo.orders"))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Pipeline", "NiFi Flow/Ingest")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTaskMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Task", "NiFi Flow/Ingest/Generate Orders (7ed464a7-01a0-1000-a568-021125e71551)")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDiscoveredAssets_MRNAgreesWithTheirOwnFields(t *testing.T) {
	// The server derives identity from (Type, Providers[0], Name); the MRN
	// the plugin sets must be exactly that or the asset is unreachable.
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %s", *a.Name)
	}
}

func TestDiscoveredAssets_MRNsAreUnique(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	seen := make(map[string]bool)
	for _, a := range result.Assets {
		assert.False(t, seen[*a.MRN], "duplicate MRN %s", *a.MRN)
		seen[*a.MRN] = true
	}
}

func TestPipelineMRN_IsNotAPrefixOfItsTasks(t *testing.T) {
	// The Contents tree is built from CONTAINS edges, not by matching MRN
	// prefixes, so nothing relies on the task MRN starting with the
	// pipeline's; stated here so nobody adds that assumption.
	pipeline := assetMRN("Pipeline", "NiFi Flow/Ingest")
	task := assetMRN("Task", "NiFi Flow/Ingest/Generate Orders")

	assert.Equal(t, "mrn://pipeline/nifi/nifi-flow-ingest", pipeline)
	assert.Equal(t, "mrn://task/nifi/nifi-flow-ingest-generate-orders", task)
}
