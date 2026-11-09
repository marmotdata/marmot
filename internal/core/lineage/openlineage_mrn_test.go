package lineage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An OpenLineage namespace and job name are whatever the producer sent, so
// these pin that neither can put a space or a stray slash in the MRN Marmot
// stores the asset under.

func TestOpenLineageMRN_IsNamespaceDotName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/shop.orders",
		openLineageMRN(AssetTypeTable, ProviderPostgreSQL, "shop", "orders"))
}

func TestOpenLineageMRN_DashesASpaceInTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dag/airflow/prod.daily-revenue-rollup",
		openLineageMRN(AssetTypeDAG, ProviderAirflow, "prod", "daily revenue rollup"))
}

func TestOpenLineageMRN_DashesASpaceInTheNamespace(t *testing.T) {
	assert.Equal(t, "mrn://asset/job/spark/my-cluster.nightly",
		openLineageMRN(AssetTypeJob, ProviderSpark, "my cluster", "nightly"))
}

func TestOpenLineageMRN_DashesTheSlashesInAConnectionURI(t *testing.T) {
	// Producers routinely send a connection URI as the namespace. Left
	// alone its slashes would add components the MRN has no room for.
	assert.Equal(t, "mrn://asset/table/postgresql/postgres:--db.example.com:5432-shop.orders",
		openLineageMRN(AssetTypeTable, ProviderPostgreSQL, "postgres://db.example.com:5432/shop", "orders"))
}

func TestOpenLineageMRN_NeverEmitsWhitespace(t *testing.T) {
	built := openLineageMRN(AssetTypeDataset, ProviderOpenLineage, "my namespace", "some\tdataset")

	assert.NotContains(t, built, " ")
	assert.NotContains(t, built, "\t")
}

func TestOpenLineageDatasetMRN_TurnsTheAddressIntoAHostHint(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql@host=db.example.com:5432/shop.public.orders",
		openLineageDatasetMRN("Table", "PostgreSQL", "postgres://DB.example.com:5432", "shop.public.orders"))
}

func TestOpenLineageDatasetMRN_FillsInTheDefaultPort(t *testing.T) {
	assert.Equal(t, "mrn://asset/topic/kafka@host=broker-1:9092/orders.v1",
		openLineageDatasetMRN("Topic", "Kafka", "kafka://broker-1", "orders.v1"))
}

func TestOpenLineageDatasetMRN_BucketKeysAreTwoLevels(t *testing.T) {
	assert.Equal(t, `mrn://asset/file/s3/lake."raw-orders.parquet"`,
		openLineageDatasetMRN("File", "S3", "s3://lake", "raw/orders.parquet"))
}

func TestOpenLineageDatasetMRN_KeepsANamespaceThatIsNotAnAddress(t *testing.T) {
	assert.Equal(t, openLineageMRN("Table", "PostgreSQL", "warehouse", "shop.orders"),
		openLineageDatasetMRN("Table", "PostgreSQL", "warehouse", "shop.orders"))
}
