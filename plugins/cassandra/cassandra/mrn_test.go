package cassandra

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cassandra has no database layer above the keyspace and a table name is
// only unique within one, so a table's MRN is built from keyspace.table.
// The keyspace itself is addressed by its bare name.

func TestTableMRN_IsKeyspaceQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/cassandra/shop.orders", assetMRN("Table", objectName("shop", "orders")))
}

func TestKeyspaceMRN_IsTheBareKeyspaceName(t *testing.T) {
	assert.Equal(t, "mrn://keyspace/cassandra/shop", assetMRN("Keyspace", "shop"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/cassandra/shop.orders_by_customer", assetMRN("View", objectName("shop", "orders_by_customer")))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI. The dot in the name is the part
	// worth proving.
	original := assetMRN("Table", objectName("shop", "orders"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, "shop.orders", parsed.Name)
	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from Type, Providers[0] and Name, so the
	// MRN an asset carries has to be exactly mrn.New over those.
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestViewAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := testSource().viewAsset("shop", viewInfo{Name: "orders_by_status", BaseTable: "orders"}, nil)

	require.NotNil(t, a.MRN)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestKeyspaceAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := testSource().keyspaceAsset(keyspaceInfo{Name: "shop"}, clusterInfo{}, 0, 0, nil)

	require.NotNil(t, a.MRN)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a Cassandra service's tables onto
	// provider "Cassandra" with keyspace.table names and a Keyspace
	// container. This plugin has to land on the same MRNs or the day it
	// takes over, those assets are stranded and a second copy appears.
	assert.Equal(t, "mrn://table/cassandra/shop.orders", assetMRN("Table", objectName("shop", "orders")))
	assert.Equal(t, "mrn://keyspace/cassandra/shop", assetMRN("Keyspace", "shop"))
}

func TestKeyspaceMRN_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// The Contents tree is built from the CONTAINS edges Discover emits,
	// not by matching MRN prefixes, so the two MRN shapes are independent.
	keyspace := assetMRN("Keyspace", "shop")
	table := assetMRN("Table", objectName("shop", "orders"))

	assert.Equal(t, "mrn://keyspace/cassandra/shop", keyspace)
	assert.Equal(t, "mrn://table/cassandra/shop.orders", table)
}
