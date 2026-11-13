package cassandra

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cassandra has no database layer above the keyspace and a table name is only unique within one, so a table's MRN is built from keyspace.table. The keyspace itself is addressed by its bare name.

func TestTableMRN_IsKeyspaceQualified(t *testing.T) {
	cfg := testSource().config

	assert.Equal(t, "mrn://asset/table/cassandra/shop.orders", assetMRN(cfg, "Table", "shop", "orders"))
	assert.Equal(t, "mrn://asset/view/cassandra/shop.orders_by_customer", assetMRN(cfg, "View", "shop", "orders_by_customer"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "orders"), assetMRN(cfg, "Table", "archive", "orders"))
}

func TestKeyspaceMRN_IsTheBareKeyspaceName(t *testing.T) {
	assert.Equal(t, "mrn://asset/keyspace/cassandra/shop", assetMRN(testSource().config, "Keyspace", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/cassandra@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestTableAsset_NameIsTheTablesOwnName(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	require.NotNil(t, a.MRN)
	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, "mrn://asset/table/cassandra/shop.orders", *a.MRN)
	assert.Empty(t, a.PreviousMRNs)
}

func TestTableAsset_ListsTheZonelessMRNAsPreviousWhenAnZoneIsSet(t *testing.T) {
	s := testSource()
	s.config.Zone = "prod-eu"

	a := s.tableAsset("shop", ordersTable(), nil, nil)

	assert.Equal(t, "mrn://asset/table/cassandra@prod-eu/shop.orders", *a.MRN)
	assert.Equal(t, []string{"mrn://asset/table/cassandra/shop.orders"}, a.PreviousMRNs)
}

func TestViewAsset_NameAndMRN(t *testing.T) {
	a := testSource().viewAsset("shop", viewInfo{Name: "orders_by_status", BaseTable: "orders"}, nil)

	assert.Equal(t, "orders_by_status", *a.Name)
	assert.Equal(t, "mrn://asset/view/cassandra/shop.orders_by_status", *a.MRN)
}

func TestKeyspaceAsset_NameAndMRN(t *testing.T) {
	a := testSource().keyspaceAsset(keyspaceInfo{Name: "shop"}, clusterInfo{}, 0, 0, nil)

	assert.Equal(t, "shop", *a.Name)
	assert.Equal(t, "mrn://asset/keyspace/cassandra/shop", *a.MRN)
}
