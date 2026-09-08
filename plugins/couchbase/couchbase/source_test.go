package couchbase

import (
	"testing"

	"github.com/couchbase/gocb/v2"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"connection_string": "couchbase://localhost",
		"username":          "Administrator",
		"password":          "password",
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
}

func TestValidate_MissingConnectionStringFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"username": "a", "password": "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection_string")
}

func TestValidate_MissingUsernameFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"connection_string": "couchbase://localhost", "password": "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_MissingPasswordFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"connection_string": "couchbase://localhost", "username": "a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestValidate_RejectsUnknownScheme(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["connection_string"] = "http://localhost:8091"
	_, err := s.Validate(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "couchbase://")
}

func TestValidate_AcceptsTLSScheme(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["connection_string"] = "couchbases://cb.example.cloud.couchbase.com"
	_, err := s.Validate(config)
	require.NoError(t, err)
}

func TestValidate_AddsSchemeToBareHost(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["connection_string"] = " cb.internal "
	_, err := s.Validate(config)
	require.NoError(t, err)
	assert.Equal(t, "couchbase://cb.internal", s.config.ConnectionString)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(validConfig())
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeIndexes)
	assert.True(t, s.config.IncludeStatistics)
	assert.False(t, s.config.IncludeSystemScopes)
	assert.False(t, s.config.SSLSkipVerify)
	assert.Equal(t, 100, s.config.SampleSize)
	assert.Equal(t, 10, s.config.ConnectTimeoutSeconds)
	assert.Empty(t, s.config.ExcludeBuckets)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["include_columns"] = false
	config["include_statistics"] = false
	_, err := s.Validate(config)
	require.NoError(t, err)

	assert.False(t, s.config.IncludeColumns)
	assert.False(t, s.config.IncludeStatistics)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeIndexes)
}

func TestValidate_RejectsZeroSampleSize(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["sample_size"] = 0
	_, err := s.Validate(config)
	require.Error(t, err)
}

func TestValidate_RejectsOversizedSample(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["sample_size"] = 10001
	_, err := s.Validate(config)
	require.Error(t, err)
}

func TestValidate_AcceptsBucketFilters(t *testing.T) {
	s := &Source{}
	config := validConfig()
	config["bucket"] = "shop"
	config["exclude_buckets"] = []interface{}{"travel-sample"}
	_, err := s.Validate(config)
	require.NoError(t, err)
	assert.Equal(t, "shop", s.config.Bucket)
	assert.Equal(t, []string{"travel-sample"}, s.config.ExcludeBuckets)
}

func TestCollectionName_IsBucketScopeCollection(t *testing.T) {
	assert.Equal(t, "shop.sales.orders", collectionName("shop", "sales", "orders"))
}

func TestQuoteKeyspace_BacktickQuotesEveryPart(t *testing.T) {
	assert.Equal(t, "`travel-sample`.`inventory`.`airline`", quoteKeyspace("travel-sample", "inventory", "airline"))
}

func TestQuoteKeyspace_EscapesBackticks(t *testing.T) {
	assert.Equal(t, "`we``ird`.`_default`.`_default`", quoteKeyspace("we`ird", "_default", "_default"))
}

func TestBucketTypeName_MapsMembaseToCouchbase(t *testing.T) {
	assert.Equal(t, "couchbase", bucketTypeName(gocb.CouchbaseBucketType))
	assert.Equal(t, "ephemeral", bucketTypeName(gocb.EphemeralBucketType))
	assert.Equal(t, "memcached", bucketTypeName(gocb.MemcachedBucketType))
}

func TestDurabilityLevelName_UsesServerNames(t *testing.T) {
	assert.Equal(t, "none", durabilityLevelName(gocb.DurabilityLevelNone))
	assert.Equal(t, "majority", durabilityLevelName(gocb.DurabilityLevelMajority))
	assert.Equal(t, "majorityAndPersistActive", durabilityLevelName(gocb.DurabilityLevelMajorityAndPersistOnMaster))
	assert.Equal(t, "persistToMajority", durabilityLevelName(gocb.DurabilityLevelPersistToMajority))
}

func TestDurabilityLevelName_UnknownIsEmpty(t *testing.T) {
	assert.Equal(t, "", durabilityLevelName(gocb.DurabilityLevelUnknown))
}

func TestManagementURL_PlainSchemeUses8091(t *testing.T) {
	assert.Equal(t, "http://localhost:8091", managementURL("couchbase://localhost"))
}

func TestManagementURL_TLSSchemeUses18091(t *testing.T) {
	assert.Equal(t, "https://cb.abc.cloud.couchbase.com:18091", managementURL("couchbases://cb.abc.cloud.couchbase.com"))
}

func TestManagementURL_TakesFirstHostAndDropsPortAndOptions(t *testing.T) {
	assert.Equal(t, "http://node1:8091", managementURL("couchbase://node1:11210,node2:11210?network=external"))
}

func TestManagementURL_KeepsIPv6Literal(t *testing.T) {
	assert.Equal(t, "http://[::1]:8091", managementURL("couchbase://[::1]:11210"))
}

func TestManagementURL_BareHost(t *testing.T) {
	assert.Equal(t, "http://cb.internal:8091", managementURL("cb.internal"))
}

func TestTabulate_UnionsKeysWithIDFirst(t *testing.T) {
	docs := []map[string]interface{}{
		{"id": "a", "name": "Alice", "age": float64(30)},
		{"id": "b", "name": "Bob", "email": "bob@example.com"},
	}

	columns, rows := tabulate(docs)

	assert.Equal(t, []string{"id", "age", "email", "name"}, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, []interface{}{"a", float64(30), nil, "Alice"}, rows[0])
	assert.Equal(t, []interface{}{"b", nil, "bob@example.com", "Bob"}, rows[1])
}

func TestTabulate_NoDocuments(t *testing.T) {
	columns, rows := tabulate(nil)
	assert.Empty(t, columns)
	assert.Empty(t, rows)
}
