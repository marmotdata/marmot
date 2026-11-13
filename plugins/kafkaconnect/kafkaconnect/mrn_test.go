package kafkaconnect

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDiscovery(zone string, servers string, connectorMap map[string]map[string]string) *discovery {
	cfg := &Config{BootstrapServers: servers, ConnectorMap: connectorMap}
	cfg.Zone = zone
	return &discovery{config: cfg}
}

func TestPipelineMRN_IsTheConnectorName(t *testing.T) {
	d := testDiscovery("", "", nil)
	assert.Equal(t, "mrn://asset/pipeline/kafka-connect/orders-file-source", d.config.AssetMRN("Pipeline", provider, "orders-file-source"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	d := testDiscovery("prod", "", nil)
	assert.Equal(t, "mrn://asset/pipeline/kafka-connect@prod/orders-file-source", d.config.AssetMRN("Pipeline", provider, "orders-file-source"))
}

func TestTaskMRN_IsTheConnectorNameAndTaskId(t *testing.T) {
	src := testDiscovery("", "", nil)
	asset := src.taskAsset("orders-file-source", taskStatus{ID: 0})

	assert.Equal(t, "mrn://asset/task/kafka-connect/orders-file-source.task-0", *asset.MRN)
	assert.Equal(t, "task-0", *asset.Name)
}

func TestTaskMRN_SameTaskIdInDifferentConnectorsDiffers(t *testing.T) {
	d := testDiscovery("", "", nil)
	a := d.taskAsset("a", taskStatus{ID: 0})
	b := d.taskAsset("b", taskStatus{ID: 0})
	assert.NotEqual(t, *a.MRN, *b.MRN)
}

func TestTaskMRN_ADottedConnectorNameChangesTheMRNSoTheOldOneIsKept(t *testing.T) {
	d := testDiscovery("", "", nil)
	asset := d.taskAsset("orders.v1", taskStatus{ID: 2})

	assert.Equal(t, `mrn://asset/task/kafka-connect/"orders.v1".task-2`, *asset.MRN)
	assert.Equal(t, []string{"mrn://asset/task/kafka-connect/orders.v1.task-2"}, asset.PreviousMRNs)
}

func TestTopicMRN_MatchesWhatTheKafkaPluginProduces(t *testing.T) {
	// The Kafka plugin names a topic by its name alone. This has to be byte-identical or a topic both plugins see becomes two assets.
	d := testDiscovery("", "", nil)
	assert.Equal(t, "mrn://asset/topic/kafka/orders-events", d.topicMRN("c", connectorConfig{}, "orders-events"))
	assert.Equal(t, mrn.Asset("Topic", "Kafka", "shop.public.orders").String(), d.topicMRN("c", connectorConfig{}, "shop.public.orders"))
}

func TestTopicMRN_TheConnectClustersBootstrapServersBecomeAHostHint(t *testing.T) {
	d := testDiscovery("", "Broker-1:9093,broker-2:9093", nil)
	assert.Equal(t, "mrn://asset/topic/kafka@host=broker-1:9093/orders", d.topicMRN("c", connectorConfig{}, "orders"))
}

func TestTopicMRN_TheConnectorsOwnBootstrapServersWin(t *testing.T) {
	d := testDiscovery("", "broker-1", nil)
	cfg := connectorConfig{"producer.override.bootstrap.servers": "other:9094"}
	assert.Equal(t, "mrn://asset/topic/kafka@host=other:9094/orders", d.topicMRN("c", cfg, "orders"))
}

func TestTopicMRN_ABootstrapServerWithoutAPortGetsTheDefault(t *testing.T) {
	d := testDiscovery("", "broker-1", nil)
	assert.Equal(t, "mrn://asset/topic/kafka@host=broker-1:9092/orders", d.topicMRN("c", connectorConfig{}, "orders"))
}

func TestConnectorMap_AnExplicitZoneBeatsTheHostHint(t *testing.T) {
	d := testDiscovery("", "broker-1:9092", map[string]map[string]string{"orders-sink": {"kafka": "prod-eu", "PostgreSQL": "warehouse"}})

	assert.Equal(t, "mrn://asset/topic/kafka@prod-eu/orders", d.topicMRN("orders-sink", connectorConfig{}, "orders"))
	assert.Equal(t, "mrn://asset/table/postgresql@warehouse/shop.public.orders",
		d.datasetMRN("orders-sink", dataset{Type: "Table", Provider: "PostgreSQL", Path: []string{"shop", "public", "orders"}, Host: "db:5432"}))
}

func TestConnectorMap_OnlyAppliesToTheNamedConnector(t *testing.T) {
	d := testDiscovery("", "", map[string]map[string]string{"orders-sink": {"PostgreSQL": "warehouse"}})

	assert.Equal(t, "mrn://asset/table/postgresql@host=db:5432/shop.public.orders",
		d.datasetMRN("other-sink", dataset{Type: "Table", Provider: "PostgreSQL", Path: []string{"shop", "public", "orders"}, Host: "db:5432"}))
}

func TestDatasetMRNs_MatchTheOwningPlugins(t *testing.T) {
	// Pinned to the naming table of each plugin the lineage points at.
	d := testDiscovery("", "", nil)
	cases := []struct {
		ds   dataset
		want string
	}{
		{table("PostgreSQL", "shop", "public", "orders"), "mrn://asset/table/postgresql/shop.public.orders"},
		{table("MySQL", "shop", "orders"), "mrn://asset/table/mysql/shop.orders"},
		{table("MariaDB", "shop", "orders"), "mrn://asset/table/mariadb/shop.orders"},
		{table("SQL Server", "shop", "dbo", "orders"), "mrn://asset/table/sql-server/shop.dbo.orders"},
		{table("Oracle", "ORCL", "SHOP", "ORDERS"), "mrn://asset/table/oracle/orcl.shop.orders"},
		{table("Snowflake", "SHOP", "RAW", "ORDERS"), "mrn://asset/table/snowflake/shop.raw.orders"},
		{table("Redshift", "dev", "public", "orders"), "mrn://asset/table/redshift/dev.public.orders"},
		{table("BigQuery", "acme", "raw", "orders"), "mrn://asset/table/bigquery/acme.raw.orders"},
		{table("Elasticsearch", "orders"), "mrn://asset/table/elasticsearch/orders"},
		{collection("", "shop", "orders"), "mrn://asset/collection/mongodb/shop.orders"},
		{dataset{Type: "Bucket", Provider: "S3", Path: []string{"data-lake"}}, "mrn://asset/bucket/s3/data-lake"},
		{dataset{Type: "Bucket", Provider: "GCS", Path: []string{"data-lake"}}, "mrn://asset/bucket/gcs/data-lake"},
		{dataset{Type: "Container", Provider: "AzureBlob", Path: []string{"raw"}}, "mrn://asset/container/azureblob/raw"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, d.datasetMRN("c", tc.ds))
	}
}

func TestEveryAsset_MRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, twoFileConnectors().with(brokenSinkFixture), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *a.MRN)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, strings.ReplaceAll(strings.ToLower(a.Providers[0]), " ", "-"), parsed.Provider)
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
