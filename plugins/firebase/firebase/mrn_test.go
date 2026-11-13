package firebase

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Firestore database id and a Realtime Database instance id can be the same string, so the kind is folded into the asset name to keep the two apart. The project is not part of any name, so the zone option is what tells two projects apart. A name is one MRN level, and the MRN turns slashes and spaces into hyphens and lowercases it, so the parentheses of the default database id survive.

func TestFirestoreDatabaseMRN_KeepsTheDefaultDatabaseParentheses(t *testing.T) {
	name := firestoreDatabaseName("(default)")

	assert.Equal(t, "firestore/(default)", name)
	assert.Equal(t, "mrn://asset/database/firebase/firestore-(default)", assetMRN(&Config{}, "Database", name))
}

func TestRealtimeDatabaseMRN_IsPrefixedWithItsKind(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/firebase/rtdb-marmot-demo-default-rtdb",
		assetMRN(&Config{}, "Database", realtimeDatabaseName("marmot-demo-default-rtdb")))
}

func TestCollectionMRN_RootCollectionAndSubcollection(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/collection/firebase/firestore-(default)-orders",
		assetMRN(cfg, "Collection", collectionName("(default)", "orders")))
	// The subcollection "lines" exists once per order document. They share a shape, so they are catalogued as one asset with the document id elided.
	assert.Equal(t, "mrn://asset/collection/firebase/firestore-(default)-orders-lines",
		assetMRN(cfg, "Collection", collectionName("(default)", "orders/lines")))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod"}}

	got := assetMRN(cfg, "Collection", collectionName("(default)", "orders"))
	assert.Equal(t, "mrn://asset/collection/firebase@prod/firestore-(default)-orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod", parsed.Zone)
}

func TestPreviousMRNs_OnlyDifferOnceAnZoneIsSet(t *testing.T) {
	name := collectionName("(default)", "orders")

	plain := assetMRN(&Config{}, "Collection", name)
	assert.Empty(t, previousMRNs("Collection", name, plain))

	withZone := assetMRN(&Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod"}}, "Collection", name)
	assert.Equal(t, []string{plain}, previousMRNs("Collection", name, withZone))
}

func TestCollectionAsset_MRNFollowsItsName(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)

	group := collectionGroup{id: "lines", path: "orders/lines", parent: "orders", depth: 2}
	asset, err := s.collectionAsset("(default)", group, 3, nil, nil)
	require.NoError(t, err)

	require.NotNil(t, asset.MRN)
	assert.Equal(t, "mrn://asset/collection/firebase/firestore-(default)-orders-lines", *asset.MRN)
}

func TestCollectionAsset_RecordsTheSubcollectionsGroupAndParent(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)

	group := collectionGroup{id: "lines", path: "orders/lines", parent: "orders", depth: 2}
	asset, err := s.collectionAsset("(default)", group, 3, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "firestore/(default)/orders/lines", *asset.Name)
	assert.Equal(t, "lines", asset.Metadata["collection_id"])
	assert.Equal(t, "orders/lines", asset.Metadata["collection_path"])
	assert.Equal(t, "lines", asset.Metadata["collection_group_id"])
	assert.Equal(t, "orders", asset.Metadata["parent_path"])
	assert.Equal(t, 2, asset.Metadata["depth"])
	assert.Equal(t, 3, asset.Metadata["sampled_documents"])
}

func TestCollectionAsset_RootCollectionHasNoParentFields(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)

	group := collectionGroup{id: "orders", path: "orders", depth: 1}
	asset, err := s.collectionAsset("(default)", group, 5, nil, nil)
	require.NoError(t, err)

	assert.NotContains(t, asset.Metadata, "collection_group_id")
	assert.NotContains(t, asset.Metadata, "parent_path")
}
