package nifi

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pipeline is addressed by its path from the root group and a task by that path plus the processor name, one MRN level per group.

func TestPipelineMRN_IsThePathFromTheRoot(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/nifi/nifi-flow.ingest", assetMRN("Pipeline", "NiFi Flow/Ingest"))
}

func TestRootPipelineMRN_IsTheRootGroupsName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/nifi/nifi-flow", assetMRN("Pipeline", "NiFi Flow"))
}

func TestTaskMRN_IsThePipelinePathAndProcessorName(t *testing.T) {
	assert.Equal(t, "mrn://asset/task/nifi/nifi-flow.ingest.generateflowfile", assetMRN("Task", "NiFi Flow/Ingest/GenerateFlowFile"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	d := &discovery{config: &Config{}}
	d.config.Zone = "prod"

	assert.Equal(t, "mrn://asset/pipeline/nifi@prod/nifi-flow.ingest", d.assetMRN("Pipeline", "NiFi Flow", "Ingest"))
}

func TestTaskMRN_SameProcessorNameInDifferentGroupsDiffers(t *testing.T) {
	assert.NotEqual(t, assetMRN("Task", "NiFi Flow/A/Log"), assetMRN("Task", "NiFi Flow/B/Log"))
}

func TestTopicMRN_IsTheKafkaPluginsIdentity(t *testing.T) {
	// The topic an edge points at has to be the asset the Kafka plugin creates, or the edge lands on a NiFi-flavoured duplicate.
	assert.Equal(t, "mrn://asset/topic/kafka/orders-events", nativeMRN("Topic", "Kafka", "", "orders-events"))
	assert.Equal(t, "mrn://asset/topic/kafka@host=broker:9092/orders-events", nativeMRN("Topic", "Kafka", "broker:9092", "orders-events"))
}

func TestBucketMRN_IsTheS3PluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/marmot-landing", nativeMRN("Bucket", "S3", "", "marmot-landing"))
}

func TestTableMRN_IsThePostgreSQLPluginsIdentity(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql@host=db:5432/shop.public.orders", nativeMRN("Table", "PostgreSQL", "db:5432", "shop", "public", "orders"))
}

func TestDiscoveredAssets_MRNsAreKindedAndMatchTheirTypeAndProvider(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, strings.ToLower(a.Providers[0]), parsed.Provider)
	}
}

func TestDiscoveredAssets_AreNamedAfterThemselvesAndKeepTheOldMRN(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	for _, a := range result.Assets {
		if *a.MRN != assetMRN("Task", "NiFi Flow/Ingest/Generate Orders") {
			continue
		}
		assert.Equal(t, "Generate Orders", *a.Name)
		assert.Equal(t, []string{"mrn://asset/task/nifi/nifi-flow-ingest-generate-orders"}, a.PreviousMRNs)
		return
	}
	t.Fatal("task not discovered")
}

func TestDiscoveredAssets_MRNsAreUnique(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	seen := make(map[string]bool)
	for _, a := range result.Assets {
		assert.False(t, seen[*a.MRN], "duplicate MRN %s", *a.MRN)
		seen[*a.MRN] = true
	}
}
