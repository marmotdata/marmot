package mongodb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A collection's MRN is built from database.collection, so an "events" collection in two databases stays two assets.

func TestCollectionMRN_IsDatabaseQualified(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/collection/mongodb/shop.orders", assetMRN(cfg, "Collection", "shop", "orders"))
	assert.NotEqual(t, assetMRN(cfg, "Collection", "shop", "events"), assetMRN(cfg, "Collection", "crm", "events"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/mongodb/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Collection", "shop", "orders")
	assert.Equal(t, "mrn://asset/collection/mongodb@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestMongoHosts(t *testing.T) {
	assert.Equal(t, []string{"db.example.com:27017"}, mongoHosts(&Config{Host: "DB.example.com", Port: 27017}))
	assert.Equal(t,
		[]string{"a.example.com:27018", "b.example.com:27017"},
		mongoHosts(&Config{ConnectionURI: "mongodb://user:p%40ss@A.example.com:27018,b.example.com/shop?replicaSet=rs0"}))
	assert.Empty(t, mongoHosts(&Config{ConnectionURI: "mongodb+srv://user:pw@cluster0.example.mongodb.net/shop"}))
}
