package unitycatalog

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every object below a catalog is named by its three-part name, so the
// same table name in two schemas stays two assets. The provider has a
// space in it, which mrn.New lowercases but keeps.

func TestCatalogMRN_IsTheCatalogName(t *testing.T) {
	assert.Equal(t, "mrn://catalog/unity catalog/unity", assetMRN("Catalog", "unity"))
}

func TestTableMRN_IsTheThreePartName(t *testing.T) {
	assert.Equal(t, "mrn://table/unity catalog/unity.default.numbers", assetMRN("Table", "unity.default.numbers"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/unity catalog/shop.sales.big_orders", assetMRN("View", "shop.sales.big_orders"))
}

func TestVolumeMRN_IsTheThreePartName(t *testing.T) {
	assert.Equal(t, "mrn://volume/unity catalog/unity.default.txt_files", assetMRN("Volume", "unity.default.txt_files"))
}

func TestFunctionMRN_IsTheThreePartName(t *testing.T) {
	assert.Equal(t, "mrn://function/unity catalog/unity.default.sum", assetMRN("Function", "unity.default.sum"))
}

func TestModelMRN_IsTheThreePartName(t *testing.T) {
	assert.Equal(t, "mrn://model/unity catalog/ml.models.churn", assetMRN("Model", "ml.models.churn"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Table", "unity.default.numbers")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestCatalogMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Catalog", "unity")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, "unity catalog", parsed.Service)
	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestAssets_MRNAgreesWithOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name); the MRN
	// the plugin sets has to be exactly that.
	d := &discovery{config: &Config{}}

	assets := []pluginsdk.Asset{
		d.catalogAsset(catalogInfo{Name: "unity"}, 1, 2),
		d.tableAsset(tableInfo{CatalogName: "unity", SchemaName: "default", Name: "numbers", TableType: "EXTERNAL"}),
		d.tableAsset(tableInfo{CatalogName: "shop", SchemaName: "sales", Name: "big_orders", TableType: "VIEW"}),
		d.volumeAsset(volumeInfo{CatalogName: "unity", SchemaName: "default", Name: "txt_files"}),
		d.functionAsset(functionInfo{CatalogName: "unity", SchemaName: "default", Name: "sum"}),
		d.modelAsset(registeredModelInfo{CatalogName: "ml", SchemaName: "models", Name: "churn"}, nil),
	}

	for _, a := range assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
		assert.Equal(t, "Unity Catalog", a.Providers[0])
	}
}

func TestTableMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a UnityCatalog service onto the
	// same provider and the same catalog.schema.table name, so a table
	// reached through either route lands on one asset.
	assert.Equal(t, "mrn://table/unity catalog/shop.sales.orders",
		mrn.New("Table", "Unity Catalog", "shop.sales.orders"))
	assert.Equal(t, assetMRN("Table", "shop.sales.orders"),
		mrn.New("Table", "Unity Catalog", "shop.sales.orders"))
}

func TestStorageEdgeMRNs_MatchTheStoragePlugins(t *testing.T) {
	// FEEDS edges start at assets the S3, GCS and Azure Blob plugins own,
	// so they have to be spelled exactly the way those plugins spell them.
	assert.Equal(t, "mrn://bucket/s3/marmot-lake", mrn.New("Bucket", "S3", "marmot-lake"))
	assert.Equal(t, "mrn://bucket/gcs/marmot-gcs-lake", mrn.New("Bucket", "GCS", "marmot-gcs-lake"))
	assert.Equal(t, "mrn://container/azureblob/lake", mrn.New("Container", "AzureBlob", "lake"))
}
