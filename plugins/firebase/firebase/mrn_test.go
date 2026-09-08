package firebase

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Firestore database id and a Realtime Database instance id can be the same
// string, so the kind is folded into the asset name to keep the two apart.
// mrn.New only lowercases and turns slashes and spaces into hyphens, so the
// parentheses of the default database id survive into the MRN.

func TestFirestoreDatabaseMRN_KeepsTheDefaultDatabaseParentheses(t *testing.T) {
	name := firestoreDatabaseName("(default)")

	assert.Equal(t, "firestore/(default)", name)
	assert.Equal(t, "mrn://database/firebase/firestore-(default)", assetMRN("Database", name))
}

func TestFirestoreDatabaseMRN_NamedDatabase(t *testing.T) {
	assert.Equal(t, "mrn://database/firebase/firestore-analytics",
		assetMRN("Database", firestoreDatabaseName("analytics")))
}

func TestRealtimeDatabaseMRN_IsPrefixedWithItsKind(t *testing.T) {
	assert.Equal(t, "mrn://database/firebase/rtdb-marmot-demo-default-rtdb",
		assetMRN("Database", realtimeDatabaseName("marmot-demo-default-rtdb")))
}

func TestCollectionMRN_RootCollection(t *testing.T) {
	assert.Equal(t, "mrn://collection/firebase/firestore-(default)-orders",
		assetMRN("Collection", collectionName("(default)", "orders")))
}

func TestCollectionMRN_SubcollectionLeavesOutTheParentDocumentID(t *testing.T) {
	// The subcollection "lines" exists once per order document. They share a
	// shape, so they are catalogued as one asset with the document id elided.
	assert.Equal(t, "mrn://collection/firebase/firestore-(default)-orders-lines",
		assetMRN("Collection", collectionName("(default)", "orders/lines")))
}

func TestDatabaseMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI. The parentheses are the risky
	// part here.
	original := assetMRN("Database", firestoreDatabaseName("(default)"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestCollectionMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Collection", collectionName("(default)", "orders/lines"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// The Marmot server rebuilds an asset's identity from its type, first
// provider and name, so what the plugin sets has to agree with that.
func TestCollectionAsset_MRNMatchesTheServersDerivation(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"project_id": "marmot-demo"})
	require.NoError(t, err)

	group := collectionGroup{id: "lines", path: "orders/lines", parent: "orders", depth: 2}
	asset, err := s.collectionAsset("(default)", group, 3, nil, nil)
	require.NoError(t, err)

	require.NotNil(t, asset.Name)
	require.NotNil(t, asset.MRN)
	assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
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
