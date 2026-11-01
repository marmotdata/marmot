package couchbase

import (
	"testing"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupIndexes_KeysByScopeAndCollection(t *testing.T) {
	grouped := groupIndexes([]gocb.QueryIndex{
		{Name: "orders_primary", IsPrimary: true, ScopeName: "sales", CollectionName: "orders", BucketName: "shop"},
		{Name: "customers_email", ScopeName: "sales", CollectionName: "customers", BucketName: "shop"},
		{Name: "orders_status", ScopeName: "sales", CollectionName: "orders", BucketName: "shop"},
	})

	require.Len(t, grouped, 2)
	assert.Len(t, grouped["sales.orders"], 2)
	assert.Len(t, grouped["sales.customers"], 1)
}

func TestGroupIndexes_BucketLevelIndexBelongsToTheDefaultCollection(t *testing.T) {
	// An index created with CREATE INDEX ... ON `shop` before collections
	// existed comes back with no scope or collection; gocb then leaves
	// both empty and sets BucketName from keyspace_id.
	grouped := groupIndexes([]gocb.QueryIndex{
		{Name: "#primary", IsPrimary: true, Keyspace: "shop", BucketName: "shop"},
	})

	require.Contains(t, grouped, "_default._default")
	assert.Equal(t, "#primary", grouped["_default._default"][0].Name)
}

func TestGroupIndexes_Empty(t *testing.T) {
	assert.Empty(t, groupIndexes(nil))
}

func TestHasPrimaryIndex_TrueWhenAnyIndexIsPrimary(t *testing.T) {
	assert.True(t, hasPrimaryIndex([]gocb.QueryIndex{
		{Name: "by_email"},
		{Name: "#primary", IsPrimary: true},
	}))
}

func TestHasPrimaryIndex_FalseForSecondaryIndexesOnly(t *testing.T) {
	assert.False(t, hasPrimaryIndex([]gocb.QueryIndex{{Name: "by_email"}}))
}

func TestHasPrimaryIndex_FalseWhenNoIndexes(t *testing.T) {
	assert.False(t, hasPrimaryIndex(nil))
}

func TestIndexNames_SortedRegardlessOfServerOrder(t *testing.T) {
	names := indexNames([]gocb.QueryIndex{
		{Name: "orders_status"},
		{Name: "#primary", IsPrimary: true},
		{Name: "orders_customer"},
	})

	assert.Equal(t, []string{"#primary", "orders_customer", "orders_status"}, names)
}

func TestIndexNames_EmptyForNoIndexes(t *testing.T) {
	assert.Empty(t, indexNames(nil))
}
