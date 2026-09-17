package kafkaconnect

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A connector is named by its own name and a task by the connector name
// and its numeric id, the only identity Connect gives a task. The
// provider has a space in it, which mrn.New keeps.

func TestPipelineMRN_IsTheConnectorName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/kafka-connect/orders-file-source", assetMRN("Pipeline", "orders-file-source"))
}

func TestTaskMRN_IsTheConnectorNameAndTaskId(t *testing.T) {
	assert.Equal(t, "mrn://task/kafka-connect/orders-file-source.task-0", assetMRN("Task", taskName("orders-file-source", 0)))
}

func TestTopicMRN_MatchesWhatTheKafkaPluginProduces(t *testing.T) {
	// plugins/kafka builds mrn.New("Topic", "Kafka", topic). This has to be
	// byte-identical or a topic both plugins see becomes two assets.
	assert.Equal(t, "mrn://topic/kafka/orders-events", topicMRN("orders-events"))
	assert.Equal(t, mrn.New("Topic", "Kafka", "shop.public.orders"), topicMRN("shop.public.orders"))
}

func TestDatasetMRNs_MatchTheOwningPlugins(t *testing.T) {
	// Pinned to the identity rule of each plugin the lineage points at.
	assert.Equal(t, "mrn://table/postgresql/orders", datasetMRN(dataset{"Table", "PostgreSQL", "orders"}))
	assert.Equal(t, "mrn://table/mysql/orders", datasetMRN(dataset{"Table", "MySQL", "orders"}))
	assert.Equal(t, "mrn://table/mariadb/orders", datasetMRN(dataset{"Table", "MariaDB", "orders"}))
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", datasetMRN(dataset{"Table", "SQL Server", "shop.dbo.orders"}))
	assert.Equal(t, "mrn://table/oracle/shop.orders", datasetMRN(dataset{"Table", "Oracle", "SHOP.ORDERS"}))
	assert.Equal(t, "mrn://table/snowflake/shop.raw.orders", datasetMRN(dataset{"Table", "Snowflake", "SHOP.RAW.ORDERS"}))
	assert.Equal(t, "mrn://table/redshift/dev.public.orders", datasetMRN(dataset{"Table", "Redshift", "dev.public.orders"}))
	assert.Equal(t, "mrn://table/bigquery/orders", datasetMRN(dataset{"Table", "BigQuery", "orders"}))
	assert.Equal(t, "mrn://table/elasticsearch/orders", datasetMRN(dataset{"Table", "Elasticsearch", "orders"}))
	assert.Equal(t, "mrn://collection/mongodb/orders", datasetMRN(dataset{"Collection", "MongoDB", "orders"}))
	assert.Equal(t, "mrn://bucket/s3/data-lake", datasetMRN(dataset{"Bucket", "S3", "data-lake"}))
	assert.Equal(t, "mrn://bucket/gcs/data-lake", datasetMRN(dataset{"Bucket", "GCS", "data-lake"}))
	assert.Equal(t, "mrn://container/azureblob/raw", datasetMRN(dataset{"Container", "AzureBlob", "raw"}))
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	for _, original := range []string{
		assetMRN("Pipeline", "orders-file-source"),
		assetMRN("Task", taskName("shop-cdc", 3)),
		topicMRN("shop.public.orders"),
	} {
		parsed, err := mrn.Parse(original)
		require.NoError(t, err)
		assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
	}
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN set here has to be exactly that or the asset is stored under a
	// different identity than the lineage points at.
	result := discover(t, twoFileConnectors().with(brokenSinkFixture), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, *a.Name)
	}
}

func TestEveryEdge_PointsAtAnAssetFromThisRunOrAnotherPluginsIdentity(t *testing.T) {
	result := discover(t, twoFileConnectors(), nil)

	created := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		created[*a.MRN] = struct{}{}
	}
	for _, edge := range result.Lineage {
		assert.Contains(t, created, edge.Source)
		assert.Contains(t, created, edge.Target)
	}
}
