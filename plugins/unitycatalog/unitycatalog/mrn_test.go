package unitycatalog

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assetMRN builds the default-zone MRN of a dotted Unity Catalog name, split into its levels.
func assetMRN(assetType, name string) string {
	d := &discovery{config: &Config{}}
	return d.assetMRN(assetType, strings.Split(name, ".")...)
}

func TestCatalogMRN_IsTheCatalogName(t *testing.T) {
	assert.Equal(t, "mrn://asset/catalog/unity-catalog/unity", assetMRN("Catalog", "unity"))
}

func TestObjectMRNs_AreTheThreePartName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/unity-catalog/unity.default.numbers", assetMRN("Table", "unity.default.numbers"))
	assert.Equal(t, "mrn://asset/view/unity-catalog/shop.sales.big_orders", assetMRN("View", "shop.sales.big_orders"))
	assert.Equal(t, "mrn://asset/volume/unity-catalog/unity.default.txt_files", assetMRN("Volume", "unity.default.txt_files"))
	assert.Equal(t, "mrn://asset/function/unity-catalog/unity.default.sum", assetMRN("Function", "unity.default.sum"))
	assert.Equal(t, "mrn://asset/model/unity-catalog/ml.models.churn", assetMRN("Model", "ml.models.churn"))
}

func TestTableMRN_SameNameInTwoSchemasDiffers(t *testing.T) {
	assert.NotEqual(t, assetMRN("Table", "shop.sales.orders"), assetMRN("Table", "shop.archive.orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod-eu"
	d := &discovery{config: cfg}

	a := d.tableAsset(tableInfo{CatalogName: "shop", SchemaName: "sales", Name: "orders"})

	assert.Equal(t, "mrn://asset/table/unity-catalog@prod-eu/shop.sales.orders", *a.MRN)
}

func TestTableMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := assetMRN("Table", "unity.default.numbers")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, "unity-catalog", parsed.Provider)
	assert.Equal(t, []string{"unity", "default", "numbers"}, parsed.Path())
	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestAssets_NameIsTheObjectsOwnName(t *testing.T) {
	d := &discovery{config: &Config{}}

	assets := map[string]pluginsdk.Asset{
		"unity":   d.catalogAsset(catalogInfo{Name: "unity"}, 1, 2),
		"numbers": d.tableAsset(tableInfo{CatalogName: "unity", SchemaName: "default", Name: "numbers", TableType: "EXTERNAL"}),
		"txt":     d.volumeAsset(volumeInfo{CatalogName: "unity", SchemaName: "default", Name: "txt"}),
		"sum":     d.functionAsset(functionInfo{CatalogName: "unity", SchemaName: "default", Name: "sum"}),
		"churn":   d.modelAsset(registeredModelInfo{CatalogName: "ml", SchemaName: "models", Name: "churn"}, nil),
	}

	for name, a := range assets {
		require.NotNil(t, a.MRN)
		assert.Equal(t, name, *a.Name)
		assert.Equal(t, []string{"Unity Catalog"}, a.Providers)
		assert.Empty(t, a.PreviousMRNs, "the dotted name is the MRN earlier releases produced")
	}
}

func TestStorageEdgeMRNs_MatchTheStoragePlugins(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/marmot-lake", storageContainerMRN("s3://marmot-lake/tables/x"))
	assert.Equal(t, "mrn://asset/bucket/gcs/marmot-gcs-lake", storageContainerMRN("gs://marmot-gcs-lake/x"))
	assert.Equal(t, "mrn://asset/container/azureblob/lake", storageContainerMRN("abfss://lake@acct.dfs.core.windows.net/x"))
}

func TestZoneBindings(t *testing.T) {
	cfg := &Config{Host: "http://UC.internal:8080"}
	cfg.Zone = "prod"

	assert.Equal(t,
		[]pluginsdk.ZoneBinding{{Provider: "Unity Catalog", Host: "uc.internal:8080", Zone: "prod"}},
		(&Source{config: cfg}).zoneBindings())
}
