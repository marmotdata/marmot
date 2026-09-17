package firebase_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/type/latlng"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Firestore emulator:
// plugintest.Build compiles the main package and every call spawns the
// process, runs one RPC and kills it again.
//
// Start the emulator with:
//
//	docker run -d --name marmot-test-firebase-firestore -p 18811:8080 \
//	  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
//	  gcloud emulators firestore start --host-port=0.0.0.0:8080
//
// then run the tests with MARMOT_TEST_FIRESTORE_EMULATOR_HOST=127.0.0.1:18811.
//
// The emulator does not implement the admin API that lists databases, so the
// database id is passed through the databases config field.

const emulatorDatabase = "(default)"

func emulatorHost(t *testing.T) string {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("set MARMOT_TEST_FIRESTORE_EMULATOR_HOST to a running Firestore emulator, for example 127.0.0.1:18811")
	}
	return host
}

// emulatorAuth is the fixed token the Firestore emulator accepts as admin.
// Listing collection ids needs it; writing documents does not.
type emulatorAuth struct{}

func (emulatorAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer owner"}, nil
}

func (emulatorAuth) RequireTransportSecurity() bool { return false }

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// seedEmulator writes a project's worth of documents into the emulator and
// returns the project id it used. Each test gets its own project id so the
// emulator's shared state cannot leak between tests.
func seedEmulator(t *testing.T, host string) string {
	t.Helper()

	projectID := fmt.Sprintf("marmot-e2e-%d", time.Now().UnixNano())

	conn, err := grpc.NewClient(host,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(emulatorAuth{}))
	require.NoError(t, err)

	client, err := firestore.NewClientWithDatabase(t.Context(), projectID, emulatorDatabase, option.WithGRPCConn(conn))
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	customers := client.Collection("customers")
	set(t, customers.Doc("c1"), map[string]any{"name": "Alice", "email": "alice@example.com"})
	// email is absent here, which is what makes the column nullable.
	set(t, customers.Doc("c2"), map[string]any{"name": "Bob"})

	placedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	orders := client.Collection("orders")
	set(t, orders.Doc("o1"), map[string]any{
		"total":     42.5,
		"quantity":  int64(3),
		"paid":      true,
		"placed_at": placedAt,
		"tags":      []any{"new", "priority"},
		"address":   map[string]any{"city": "Amsterdam", "zip": "1011"},
		"customer":  customers.Doc("c1"),
		"blob":      []byte{0x01, 0x02},
		"location":  &latlng.LatLng{Latitude: 52.37, Longitude: 4.89},
		"note":      "rush",
	})
	set(t, orders.Doc("o2"), map[string]any{
		"total":        10.0,
		"quantity":     int64(1),
		"paid":         false,
		"placed_at":    placedAt,
		"tags":         []any{"repeat"},
		"address":      map[string]any{"city": "Utrecht", "zip": "3511"},
		"customer":     customers.Doc("c2"),
		"blob":         []byte{0x03},
		"location":     &latlng.LatLng{Latitude: 52.09, Longitude: 5.12},
		"cancelled_at": nil,
	})
	// quantity is a string here, which is what makes the column mixed.
	set(t, orders.Doc("o3"), map[string]any{
		"total":     5.25,
		"quantity":  "three",
		"paid":      true,
		"placed_at": placedAt,
		"tags":      []any{},
		"address":   map[string]any{"city": "Delft"},
		"customer":  customers.Doc("c1"),
		"blob":      []byte{0x04},
		"location":  &latlng.LatLng{Latitude: 52.01, Longitude: 4.36},
	})

	// The same subcollection under two orders. It is catalogued once, and the
	// sample is drawn across both parents.
	for _, order := range []string{"o1", "o2"} {
		lines := orders.Doc(order).Collection("lines")
		set(t, lines.Doc("l1"), map[string]any{"sku": "SKU-1", "quantity": int64(2)})
		set(t, lines.Doc("l2"), map[string]any{"sku": "SKU-2", "quantity": int64(1), "discount": 0.1})
	}

	return projectID
}

func set(t *testing.T, doc *firestore.DocumentRef, data map[string]any) {
	t.Helper()
	_, err := doc.Set(t.Context(), data)
	require.NoError(t, err)
}

func emulatorConfig(projectID, host string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"project_id":   projectID,
		"databases":    []any{emulatorDatabase},
		"endpoint":     "http://" + host,
		"disable_auth": true,
		// Neither has an emulator, so leave them out of the runs that assert
		// on Firestore. One test below turns them back on.
		"include_realtime_database": false,
		"include_project_details":   false,
	}
}

func discoverFromEmulator(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	host := emulatorHost(t)
	projectID := seedEmulator(t, host)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), emulatorConfig(projectID, host))
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

func assetNames(result *pluginsdk.DiscoveryResult) []string {
	names := make([]string, 0, len(result.Assets))
	for _, asset := range result.Assets {
		if asset.Name != nil {
			names = append(names, *asset.Name)
		}
	}
	return names
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	require.FailNowf(t, "asset not found", "no asset named %q, have %v", name, assetNames(result))
	return pluginsdk.Asset{}
}

func columns(t *testing.T, asset pluginsdk.Asset) map[string]pluginsdk.Column {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.True(t, ok, "asset %s has no columns", *asset.Name)

	var parsed []pluginsdk.Column
	require.NoError(t, json.Unmarshal([]byte(raw), &parsed))

	byName := make(map[string]pluginsdk.Column, len(parsed))
	for _, column := range parsed {
		byName[column.Name] = column
	}
	return byName
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "firebase", meta.ID)
	assert.Equal(t, "Google Firebase", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingProjectIDFails(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAnEmulatorConfig(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), emulatorConfig("marmot-e2e", "127.0.0.1:18811"))
	require.NoError(t, err)
}

func TestE2E_DiscoversTheDatabaseAndItsCollections(t *testing.T) {
	result := discoverFromEmulator(t)

	assert.ElementsMatch(t, []string{
		"firestore/(default)",
		"firestore/(default)/customers",
		"firestore/(default)/orders",
		"firestore/(default)/orders/lines",
	}, assetNames(result))
}

func TestE2E_CollectionAssetTypesAndProviders(t *testing.T) {
	result := discoverFromEmulator(t)

	database := findAsset(t, result, "firestore/(default)")
	assert.Equal(t, "Database", database.Type)
	assert.Equal(t, []string{"Firebase"}, database.Providers)

	orders := findAsset(t, result, "firestore/(default)/orders")
	assert.Equal(t, "Collection", orders.Type)
	assert.Equal(t, []string{"Firebase"}, orders.Providers)
}

func TestE2E_InfersTheFirestoreValueTypes(t *testing.T) {
	orders := findAsset(t, discoverFromEmulator(t), "firestore/(default)/orders")

	byName := columns(t, orders)

	assert.Equal(t, "double", byName["total"].DataType)
	assert.Equal(t, "boolean", byName["paid"].DataType)
	assert.Equal(t, "timestamp", byName["placed_at"].DataType)
	assert.Equal(t, "array", byName["tags"].DataType)
	assert.Equal(t, "map", byName["address"].DataType)
	assert.Equal(t, "reference", byName["customer"].DataType)
	assert.Equal(t, "bytes", byName["blob"].DataType)
	assert.Equal(t, "geopoint", byName["location"].DataType)
}

func TestE2E_AFieldSeenWithTwoTypesIsMixed(t *testing.T) {
	orders := findAsset(t, discoverFromEmulator(t), "firestore/(default)/orders")

	quantity := columns(t, orders)["quantity"]

	assert.Equal(t, "mixed", quantity.DataType)
	assert.False(t, quantity.Nullable, "quantity was set in every order")
}

func TestE2E_AFieldMissingFromSomeDocumentsIsNullable(t *testing.T) {
	result := discoverFromEmulator(t)

	orders := columns(t, findAsset(t, result, "firestore/(default)/orders"))
	assert.True(t, orders["note"].Nullable, "only o1 has a note")
	assert.Equal(t, "string", orders["note"].DataType)
	assert.False(t, orders["total"].Nullable, "every order has a total")

	customers := columns(t, findAsset(t, result, "firestore/(default)/customers"))
	assert.True(t, customers["email"].Nullable, "c2 has no email")
	assert.False(t, customers["name"].Nullable)
}

func TestE2E_AnExplicitNullMakesTheColumnNullable(t *testing.T) {
	orders := findAsset(t, discoverFromEmulator(t), "firestore/(default)/orders")

	cancelledAt := columns(t, orders)["cancelled_at"]

	assert.True(t, cancelledAt.Nullable)
	assert.Equal(t, "null", cancelledAt.DataType, "the field was only ever null")
}

func TestE2E_SubcollectionIsCataloguedOnceAcrossItsParents(t *testing.T) {
	lines := findAsset(t, discoverFromEmulator(t), "firestore/(default)/orders/lines")

	assert.Equal(t, "lines", lines.Metadata["collection_id"])
	assert.Equal(t, "orders/lines", lines.Metadata["collection_path"])
	assert.Equal(t, "lines", lines.Metadata["collection_group_id"])
	assert.Equal(t, "orders", lines.Metadata["parent_path"])
	assert.EqualValues(t, 2, lines.Metadata["depth"])
	// Two lines under o1 and two under o2, sampled as one collection.
	assert.EqualValues(t, 4, lines.Metadata["sampled_documents"])

	byName := columns(t, lines)
	assert.Equal(t, "string", byName["sku"].DataType)
	assert.Equal(t, "integer", byName["quantity"].DataType)
	assert.True(t, byName["discount"].Nullable, "only l2 has a discount")
}

func TestE2E_EmitsContainsEdges(t *testing.T) {
	result := discoverFromEmulator(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://database/firebase/firestore-(default)",
		Target: "mrn://collection/firebase/firestore-(default)-orders",
		Type:   "CONTAINS",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://database/firebase/firestore-(default)",
		Target: "mrn://collection/firebase/firestore-(default)-customers",
		Type:   "CONTAINS",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://collection/firebase/firestore-(default)-orders",
		Target: "mrn://collection/firebase/firestore-(default)-orders-lines",
		Type:   "CONTAINS",
	})
}

func TestE2E_EmitsAColumnCountPerCollection(t *testing.T) {
	result := discoverFromEmulator(t)

	byMRN := make(map[string]float64, len(result.Statistics))
	for _, statistic := range result.Statistics {
		require.Equal(t, "asset.column_count", statistic.MetricName)
		byMRN[statistic.AssetMRN] = statistic.Value
	}

	// total, quantity, paid, placed_at, tags, address, customer, blob,
	// location, note, cancelled_at.
	assert.Equal(t, float64(11), byMRN["mrn://collection/firebase/firestore-(default)-orders"])
	assert.Equal(t, float64(2), byMRN["mrn://collection/firebase/firestore-(default)-customers"])
	assert.Equal(t, float64(3), byMRN["mrn://collection/firebase/firestore-(default)-orders-lines"])
}

func TestE2E_StopsAtTheConfiguredDepth(t *testing.T) {
	host := emulatorHost(t)
	projectID := seedEmulator(t, host)
	bin := buildBinary(t)

	config := emulatorConfig(projectID, host)
	config["include_subcollections"] = false

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	assert.NotContains(t, assetNames(result), "firestore/(default)/orders/lines")
	assert.Contains(t, assetNames(result), "firestore/(default)/orders")
}

// The emulator answers 404 for the admin plane. That has to stay a warning:
// the Firestore assets are still worth returning.
func TestE2E_MissingAdminPlaneOnlyWarns(t *testing.T) {
	host := emulatorHost(t)
	projectID := seedEmulator(t, host)
	bin := buildBinary(t)

	config := emulatorConfig(projectID, host)
	config["include_realtime_database"] = true
	config["include_project_details"] = true

	result, err := bin.Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Contains(t, assetNames(result), "firestore/(default)/orders")
	orders := findAsset(t, result, "firestore/(default)/orders")
	assert.NotContains(t, orders.Metadata, "firebase_project_id")
}

// The Google client libraries redirect themselves to an emulator when
// FIRESTORE_EMULATOR_HOST is set, and the plugin process inherits the
// environment, so a run with no endpoint configured still reaches it.
func TestE2E_FollowsTheFirestoreEmulatorHostEnvironmentVariable(t *testing.T) {
	host := emulatorHost(t)
	projectID := seedEmulator(t, host)
	t.Setenv("FIRESTORE_EMULATOR_HOST", host)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"project_id":                projectID,
		"databases":                 []any{emulatorDatabase},
		"include_realtime_database": false,
		"include_project_details":   false,
	})
	require.NoError(t, err)

	assert.Contains(t, assetNames(result), "firestore/(default)/orders")
}
