package lineage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An OpenLineage namespace and job name are whatever the producer sent, so
// these pin that neither can put a space or a stray slash in the MRN Marmot
// stores the asset under.

func TestOpenLineageMRN_IsNamespaceDotName(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/shop.orders",
		openLineageMRN(AssetTypeTable, ProviderPostgreSQL, "shop", "orders"))
}

func TestOpenLineageMRN_DashesASpaceInTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://dag/airflow/prod.daily-revenue-rollup",
		openLineageMRN(AssetTypeDAG, ProviderAirflow, "prod", "daily revenue rollup"))
}

func TestOpenLineageMRN_DashesASpaceInTheNamespace(t *testing.T) {
	assert.Equal(t, "mrn://job/spark/my-cluster.nightly",
		openLineageMRN(AssetTypeJob, ProviderSpark, "my cluster", "nightly"))
}

func TestOpenLineageMRN_DashesTheSlashesInAConnectionURI(t *testing.T) {
	// Producers routinely send a connection URI as the namespace. Left
	// alone its slashes would add components the MRN has no room for.
	assert.Equal(t, "mrn://table/postgresql/postgres:--db.example.com:5432-shop.orders",
		openLineageMRN(AssetTypeTable, ProviderPostgreSQL, "postgres://db.example.com:5432/shop", "orders"))
}

func TestOpenLineageMRN_NeverEmitsWhitespace(t *testing.T) {
	built := openLineageMRN(AssetTypeDataset, ProviderOpenLineage, "my namespace", "some\tdataset")

	assert.NotContains(t, built, " ")
	assert.NotContains(t, built, "\t")
}
