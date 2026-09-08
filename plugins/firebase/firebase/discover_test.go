package firebase

import (
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake serves the admin plane over HTTP. The Firestore data client, which
// speaks gRPC, cannot reach it, so these tests see the database and instance
// assets but no collections. Collections are covered end to end against the
// real Firestore emulator in e2e_test.go.

func discoverAgainst(t *testing.T, fake *fakeAdmin, extra pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := pluginsdk.RawConfig{
		"project_id":   "marmot-demo",
		"endpoint":     fake.start(t),
		"disable_auth": true,
	}
	for key, value := range extra {
		config[key] = value
	}

	// The fake cannot answer the gRPC data client, and Firestore retries an
	// unreachable server until the deadline, so keep the deadline short.
	source := &Source{timeout: 500 * time.Millisecond}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

func assetsByName(result *pluginsdk.DiscoveryResult) map[string]pluginsdk.Asset {
	byName := make(map[string]pluginsdk.Asset, len(result.Assets))
	for _, asset := range result.Assets {
		if asset.Name != nil {
			byName[*asset.Name] = asset
		}
	}
	return byName
}

func TestDiscover_EmitsAnAssetPerFirestoreDatabase(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	assets := assetsByName(discoverAgainst(t, fake, nil))

	assert.Contains(t, assets, "firestore/(default)")
	assert.Contains(t, assets, "firestore/analytics")
	assert.Equal(t, "Database", assets["firestore/(default)"].Type)
	assert.Equal(t, []string{"Firebase"}, assets["firestore/(default)"].Providers)
}

func TestDiscover_FirestoreDatabaseCarriesTheAdminMetadata(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	metadata := assetsByName(discoverAgainst(t, fake, nil))["firestore/(default)"].Metadata

	assert.Equal(t, "(default)", metadata["database_id"])
	assert.Equal(t, "firestore", metadata["database_kind"])
	assert.Equal(t, "nam5", metadata["location_id"])
	assert.Equal(t, "FIRESTORE_NATIVE", metadata["database_type"])
	assert.Equal(t, "PESSIMISTIC", metadata["concurrency_mode"])
	assert.Equal(t, "POINT_IN_TIME_RECOVERY_DISABLED", metadata["point_in_time_recovery"])
	assert.Equal(t, "DELETE_PROTECTION_DISABLED", metadata["delete_protection"])
	assert.Equal(t, "3600s", metadata["version_retention_period"])
	assert.Equal(t, "2026-09-08T10:00:00Z", metadata["earliest_version_time"])
	assert.Equal(t, "2024-01-02T03:04:05Z", metadata["create_time"])
	assert.Equal(t, "2026-05-06T07:08:09Z", metadata["update_time"])
	assert.Equal(t, "7c6f1f4e-2c1f-4a52-9a1e-3b7c0a5d9e11", metadata["uid"])
	assert.Equal(t, true, metadata["free_tier"])
}

func TestDiscover_LeavesOutMetadataTheDatabaseListingDidNotReturn(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	metadata := assetsByName(discoverAgainst(t, fake, nil))["firestore/analytics"].Metadata

	assert.Equal(t, "DATASTORE_MODE", metadata["database_type"])
	assert.NotContains(t, metadata, "concurrency_mode")
	assert.NotContains(t, metadata, "create_time")
}

func TestDiscover_EmitsBothRealtimeDatabaseInstanceTypes(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	assets := assetsByName(discoverAgainst(t, fake, nil))

	require.Contains(t, assets, "rtdb/marmot-demo-default-rtdb")
	require.Contains(t, assets, "rtdb/marmot-demo-events")

	defaultInstance := assets["rtdb/marmot-demo-default-rtdb"].Metadata
	assert.Equal(t, "realtime", defaultInstance["database_kind"])
	assert.Equal(t, "marmot-demo-default-rtdb", defaultInstance["instance_id"])
	assert.Equal(t, "DEFAULT_DATABASE", defaultInstance["instance_type"])
	assert.Equal(t, "ACTIVE", defaultInstance["state"])
	assert.Equal(t, "https://marmot-demo-default-rtdb.firebaseio.com", defaultInstance["database_url"])

	userInstance := assets["rtdb/marmot-demo-events"].Metadata
	assert.Equal(t, "USER_DATABASE", userInstance["instance_type"])
	assert.Equal(t, "DISABLED", userInstance["state"])
}

func TestDiscover_RealtimeInstanceLinksToItsDatabaseURL(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	asset := assetsByName(discoverAgainst(t, fake, nil))["rtdb/marmot-demo-events"]

	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "https://marmot-demo-events.europe-west1.firebasedatabase.app", asset.ExternalLinks[0].URL)
}

func TestDiscover_AddsProjectDetailsToEveryAsset(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	result := discoverAgainst(t, fake, nil)

	require.NotEmpty(t, result.Assets)
	for _, asset := range result.Assets {
		assert.Equal(t, "marmot-demo", asset.Metadata["firebase_project_id"], *asset.Name)
		assert.Equal(t, "Marmot Demo", asset.Metadata["firebase_project_display_name"], *asset.Name)
		assert.Equal(t, int64(451234567890), asset.Metadata["firebase_project_number"], *asset.Name)
	}
}

func TestDiscover_MissingProjectDetailsIsNotFatal(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: nil}

	assets := assetsByName(discoverAgainst(t, fake, nil))

	require.Contains(t, assets, "firestore/(default)")
	assert.NotContains(t, assets["firestore/(default)"].Metadata, "firebase_project_id")
}

func TestDiscover_SkipsProjectDetailsWhenTurnedOff(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	discoverAgainst(t, fake, pluginsdk.RawConfig{"include_project_details": false})

	assert.NotContains(t, fake.paths(), "/v1beta1/projects/marmot-demo")
}

func TestDiscover_SkipsRealtimeDatabasesWhenTurnedOff(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	assets := assetsByName(discoverAgainst(t, fake, pluginsdk.RawConfig{"include_realtime_database": false}))

	assert.NotContains(t, assets, "rtdb/marmot-demo-default-rtdb")
	assert.Contains(t, assets, "firestore/(default)")
}

func TestDiscover_DatabaseListFailureKeepsTheRealtimeInstances(t *testing.T) {
	fake := &fakeAdmin{databases: nil, instances: fakeInstances(), project: fakeProject()}

	assets := assetsByName(discoverAgainst(t, fake, nil))

	assert.NotContains(t, assets, "firestore/(default)")
	assert.Contains(t, assets, "rtdb/marmot-demo-default-rtdb")
}

func TestDiscover_EverythingFailingReturnsAnError(t *testing.T) {
	fake := &fakeAdmin{databases: nil, instances: nil, project: nil}

	source := &Source{timeout: 500 * time.Millisecond}
	_, err := source.Discover(t.Context(), pluginsdk.RawConfig{
		"project_id":   "marmot-demo",
		"endpoint":     fake.start(t),
		"disable_auth": true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing Firestore databases")
}

func TestDiscover_ConfiguredDatabasesSkipTheAdminListing(t *testing.T) {
	// The Firestore emulator answers 404 for the database listing, so an
	// emulator run has to name its databases in the config instead.
	fake := &fakeAdmin{databases: nil, instances: fakeInstances(), project: fakeProject()}

	assets := assetsByName(discoverAgainst(t, fake, pluginsdk.RawConfig{"databases": []any{"(default)"}}))

	require.Contains(t, assets, "firestore/(default)")
	assert.NotContains(t, fake.paths(), "/v1/projects/marmot-demo/databases")

	metadata := assets["firestore/(default)"].Metadata
	assert.Equal(t, "(default)", metadata["database_id"])
	assert.NotContains(t, metadata, "location_id", "nothing was read from the admin API to fill this in")
}

func TestDiscover_FirestoreDatabaseLinksToTheFirebaseConsole(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	asset := assetsByName(discoverAgainst(t, fake, nil))["firestore/(default)"]

	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "https://console.firebase.google.com/project/marmot-demo/firestore", asset.ExternalLinks[0].URL)
}

func TestDiscover_RecordsTheAssetSource(t *testing.T) {
	fake := &fakeAdmin{databases: fakeDatabases(), instances: fakeInstances(), project: fakeProject()}

	asset := assetsByName(discoverAgainst(t, fake, nil))["firestore/(default)"]

	require.Len(t, asset.Sources, 1)
	assert.Equal(t, "Firebase", asset.Sources[0].Name)
	assert.Equal(t, 1, asset.Sources[0].Priority)
}
