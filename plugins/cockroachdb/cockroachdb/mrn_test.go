package cockroachdb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from database.schema.table. One cluster holds many databases, so a bare name would fold shop.public.orders and archive.public.orders into one asset.

func TestTableMRN_IsFullyQualified(t *testing.T) {
	cfg := newSource().config

	assert.Equal(t, "mrn://asset/table/cockroachdb/shop.public.orders", assetMRN(cfg, "Table", "shop", "public", "orders"))
	assert.Equal(t, "mrn://asset/view/cockroachdb/shop.public.customer_totals", assetMRN(cfg, "View", "shop", "public", "customer_totals"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "public", "orders"), assetMRN(cfg, "Table", "shop", "sales", "orders"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "public", "orders"), assetMRN(cfg, "Table", "archive", "public", "orders"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/cockroachdb/shop", assetMRN(newSource().config, "Database", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "public", "orders")
	assert.Equal(t, "mrn://asset/table/cockroachdb@prod-eu/shop.public.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestRelationAsset_ListsTheZonelessMRNAsPreviousOnlyWhenAnZoneIsSet(t *testing.T) {
	r := relation{Schema: "public", Name: "orders", Kind: relkindTable}

	assert.Empty(t, newSource().relationAsset("shop", r, relationDetails{}).PreviousMRNs)

	s := newSource()
	s.config.Zone = "prod-eu"
	a := s.relationAsset("shop", r, relationDetails{})
	assert.Equal(t, []string{"mrn://asset/table/cockroachdb/shop.public.orders"}, a.PreviousMRNs)
}

func TestTableAsset_NameIsTheOwnNameAndMRNKeepsTheQualification(t *testing.T) {
	a := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	require.NotNil(t, a.MRN)
	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, "mrn://asset/table/cockroachdb/shop.public.orders", *a.MRN)
	assert.Equal(t, []string{"CockroachDB"}, a.Providers)
}

func TestTableMRN_UppercaseNamesAreLowercased(t *testing.T) {
	a := newSource().relationAsset("shop", relation{Schema: "Sales", Name: "Regions", Kind: relkindTable}, relationDetails{})

	assert.Equal(t, "Regions", *a.Name)
	assert.Equal(t, "mrn://asset/table/cockroachdb/shop.sales.regions", *a.MRN)
}
