package couchbase

import (
	"testing"

	"github.com/couchbase/gocb/v2"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A collection's MRN is built from bucket.scope.collection, the same shape
// the OpenMetadata projection produces for a Couchbase service, so both
// routes land on one asset. A bucket's MRN is its bare name.

func TestBucketMRN_IsTheBareBucketName(t *testing.T) {
	assert.Equal(t, "mrn://bucket/couchbase/travel-sample", assetMRN("Bucket", "travel-sample"))
}

func TestCollectionMRN_IsBucketScopeCollection(t *testing.T) {
	assert.Equal(t, "mrn://collection/couchbase/travel-sample.inventory.airline",
		assetMRN("Collection", collectionName("travel-sample", "inventory", "airline")))
}

func TestCollectionMRN_DefaultScopeAndCollectionAreSpelledOut(t *testing.T) {
	// The default collection is a real keyspace, so it keeps the same
	// three-part shape as every other collection.
	assert.Equal(t, "mrn://collection/couchbase/shop._default._default",
		assetMRN("Collection", collectionName("shop", "_default", "_default")))
}

func TestCollectionMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Collection", collectionName("travel-sample", "inventory", "airline"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestBucketMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Bucket", "travel-sample")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestBucketAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from Type, Providers[0] and Name, so
	// the MRN the asset carries must be exactly mrn.New over those.
	s := &Source{config: &Config{}}

	a := s.bucketAsset(gocb.BucketSettings{Name: "Travel Sample", BucketType: gocb.CouchbaseBucketType}, nil, nil, "")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://bucket/couchbase/travel-sample", *a.MRN)
	assert.Equal(t, "Travel Sample", *a.Name, "the name people read is the bucket's own name")
}

func TestCollectionAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	s := &Source{config: &Config{}}

	a, _ := s.collectionAsset(t.Context(), "travel-sample", "inventory", gocb.CollectionSpec{Name: "airline", ScopeName: "inventory"}, nil, nil, 1)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://collection/couchbase/travel-sample.inventory.airline", *a.MRN)
	assert.Equal(t, "travel-sample.inventory.airline", *a.Name)
}

func TestBucketAsset_IsNotAPrefixOfTheCollectionsItHolds(t *testing.T) {
	// The Contents tree is built from the CONTAINS edges Discover emits,
	// not by matching MRN prefixes: bucket and collection MRNs live under
	// different types.
	bucket := assetMRN("Bucket", "shop")
	collection := assetMRN("Collection", collectionName("shop", "sales", "orders"))

	assert.Equal(t, "mrn://bucket/couchbase/shop", bucket)
	assert.Equal(t, "mrn://collection/couchbase/shop.sales.orders", collection)
	assert.False(t, len(collection) > len(bucket) && collection[:len(bucket)] == bucket)
}
