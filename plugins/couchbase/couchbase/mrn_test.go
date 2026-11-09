package couchbase

import (
	"testing"

	"github.com/couchbase/gocb/v2"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A collection's MRN is built from bucket.scope.collection, the same shape the OpenMetadata projection produces for a Couchbase service, so both routes land on one asset. A bucket's MRN is its bare name.

func TestBucketMRN_IsTheBareBucketName(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/couchbase/travel-sample", assetMRN(&Config{}, "Bucket", "travel-sample"))
}

func TestCollectionMRN_IsBucketScopeCollection(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/collection/couchbase/travel-sample.inventory.airline", assetMRN(cfg, "Collection", "travel-sample", "inventory", "airline"))
	// The default collection is a real keyspace, so it keeps the same three-part shape as every other collection.
	assert.Equal(t, "mrn://asset/collection/couchbase/shop._default._default", assetMRN(cfg, "Collection", "shop", "_default", "_default"))
}

func TestCollectionMRN_SameNameInTwoScopesGetsDifferentMRNs(t *testing.T) {
	cfg := &Config{}

	assert.NotEqual(t,
		assetMRN(cfg, "Collection", "shop", "sales", "orders"),
		assetMRN(cfg, "Collection", "shop", "archive", "orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Collection", "shop", "sales", "orders")
	assert.Equal(t, "mrn://asset/collection/couchbase@prod-eu/shop.sales.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestBucketAsset_NameAndMRN(t *testing.T) {
	s := &Source{config: &Config{}}

	a := s.bucketAsset(gocb.BucketSettings{Name: "Travel Sample", BucketType: gocb.CouchbaseBucketType}, nil, nil, "")

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://asset/bucket/couchbase/travel-sample", *a.MRN)
	assert.Equal(t, "Travel Sample", *a.Name, "the name people read is the bucket's own name")
}

func TestCollectionAsset_NameIsTheCollectionsOwnName(t *testing.T) {
	s := &Source{config: &Config{}}

	a, _ := s.collectionAsset(t.Context(), "travel-sample", "inventory", gocb.CollectionSpec{Name: "airline", ScopeName: "inventory"}, nil, nil, 1)

	require.NotNil(t, a.MRN)
	assert.Equal(t, "airline", *a.Name)
	assert.Equal(t, "mrn://asset/collection/couchbase/travel-sample.inventory.airline", *a.MRN)
	assert.Empty(t, a.PreviousMRNs)
}

func TestCollectionAsset_ListsTheZonelessMRNAsPreviousWhenAnZoneIsSet(t *testing.T) {
	s := &Source{config: &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}}

	a, _ := s.collectionAsset(t.Context(), "travel-sample", "inventory", gocb.CollectionSpec{Name: "airline", ScopeName: "inventory"}, nil, nil, 1)

	assert.Equal(t, []string{"mrn://asset/collection/couchbase/travel-sample.inventory.airline"}, a.PreviousMRNs)
}

func TestConnectionHosts(t *testing.T) {
	assert.Equal(t, []string{"cb1.example.com:11210", "cb2.example.com:11210"}, connectionHosts("couchbase://CB1.example.com,cb2.example.com"))
	assert.Equal(t, []string{"cb.example.com:11207"}, connectionHosts("couchbases://cb.example.com/shop?network=external"))
	assert.Equal(t, []string{"cb.example.com:12000"}, connectionHosts("couchbase://cb.example.com:12000"))
}
