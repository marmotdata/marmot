package nifi

import (
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_LogsInAndSendsTheBearerTokenOnEveryRequest(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow())

	discover(t, f, nil)

	assert.Equal(t, "POST /access/token", f.paths()[0])
	for _, auth := range f.authHeaders() {
		assert.Equal(t, "Bearer "+f.jwt, auth)
	}
}

func TestDiscover_UsesAConfiguredTokenWithoutLoggingIn(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow())

	discover(t, f, pluginsdk.RawConfig{"username": "", "password": "", "token": f.jwt})

	assert.NotContains(t, f.paths(), "POST /access/token")
}

func TestDiscover_FailsWhenTheLoginIsRejected(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow())

	_, err := tryDiscover(t, f, pluginsdk.RawConfig{"password": "wrong"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in")
	assert.NotContains(t, err.Error(), "wrong", "the password must not leak into the error")
}

func TestDiscover_FailsWhenNiFiIsUnreachable(t *testing.T) {
	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1", "token": "t"})

	require.Error(t, err)
}

func TestDiscover_FailsWhenTheRootGroupCannotBeRead(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow()).failingAt("/flow/process-groups/root")

	_, err := tryDiscover(t, f, nil)

	require.Error(t, err)
}

func TestDiscover_WalksEveryGroupOnce(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow())

	discover(t, f, nil)

	paths := f.paths()
	assert.Contains(t, paths, "GET /flow/process-groups/root")
	assert.Contains(t, paths, "GET /flow/process-groups/pg-ingest")
	assert.Contains(t, paths, "GET /flow/process-groups/pg-deliver")
	assert.Len(t, paths, 6, "login, about, root group, root flow, two child flows")
}

func TestDiscover_NamesTheRootPipelineByItsBreadcrumb(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	root := findAsset(result, "Pipeline", "NiFi Flow")
	require.NotNil(t, root)
	assert.Equal(t, "mrn://pipeline/nifi/nifi-flow", *root.MRN)
	assert.Equal(t, []string{"NiFi"}, root.Providers)
}

func TestDiscover_NamesAChildPipelineByItsPathFromTheRoot(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	assert.ElementsMatch(t, []string{"NiFi Flow", "NiFi Flow/Ingest", "NiFi Flow/Deliver"}, assetNames(result, "Pipeline"))
}

func TestDiscover_NamesANestedPipelineByTheWholePath(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").child(newGroup("pg-a", "Ingest").child(newGroup("pg-b", "Parse")))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	assert.NotNil(t, findAsset(result, "Pipeline", "NiFi Flow/Ingest/Parse"))
}

func TestDiscover_NamesATaskByPipelinePathAndProcessorName(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	task := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	require.NotNil(t, task)
	assert.Equal(t, "mrn://task/nifi/nifi-flow-ingest-generate-orders", *task.MRN)
}

func TestDiscover_DisambiguatesSiblingProcessorsSharingAName(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").
		add(processor("p-a", "LogAttribute", "org.apache.nifi.processors.standard.LogAttribute")).
		add(processor("p-b", "LogAttribute", "org.apache.nifi.processors.standard.LogAttribute"))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	assert.ElementsMatch(t, []string{"NiFi Flow/LogAttribute (p-a)", "NiFi Flow/LogAttribute (p-b)"}, assetNames(result, "Task"))
}

func TestDiscover_ProcessorsInDifferentGroupsMayShareAName(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").
		child(newGroup("pg-a", "A").add(processor("p-a", "LogAttribute", "org.apache.nifi.processors.standard.LogAttribute"))).
		child(newGroup("pg-b", "B").add(processor("p-b", "LogAttribute", "org.apache.nifi.processors.standard.LogAttribute")))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	assert.ElementsMatch(t, []string{"NiFi Flow/A/LogAttribute", "NiFi Flow/B/LogAttribute"}, assetNames(result, "Task"))
}

func TestDiscover_DisambiguatesSiblingGroupsSharingAName(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").child(newGroup("pg-a", "Ingest")).child(newGroup("pg-b", "Ingest"))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	assert.ElementsMatch(t, []string{"NiFi Flow", "NiFi Flow/Ingest (pg-a)", "NiFi Flow/Ingest (pg-b)"}, assetNames(result, "Pipeline"))
}

func TestDiscover_PipelineContainsItsTasks(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	ingest := assetMRN("Pipeline", "NiFi Flow/Ingest")
	assert.True(t, hasEdge(result, ingest, assetMRN("Task", "NiFi Flow/Ingest/Generate Orders"), "CONTAINS"))
	assert.True(t, hasEdge(result, ingest, assetMRN("Task", "NiFi Flow/Ingest/Land in S3"), "CONTAINS"))
}

func TestDiscover_PipelineContainsItsChildPipelines(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	root := assetMRN("Pipeline", "NiFi Flow")
	assert.True(t, hasEdge(result, root, assetMRN("Pipeline", "NiFi Flow/Ingest"), "CONTAINS"))
	assert.True(t, hasEdge(result, root, assetMRN("Pipeline", "NiFi Flow/Deliver"), "CONTAINS"))
}

func TestDiscover_AConnectionBetweenProcessorsIsADependsOnEdge(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	gen := assetMRN("Task", "NiFi Flow/Ingest/Generate Orders")
	stamp := assetMRN("Task", "NiFi Flow/Ingest/Stamp Attributes")
	assert.True(t, hasEdge(result, gen, stamp, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, stamp, assetMRN("Task", "NiFi Flow/Ingest/Land in S3"), "DEPENDS_ON"))
	assert.False(t, hasEdge(result, stamp, gen, "DEPENDS_ON"), "edges follow the flow direction")
}

func TestDiscover_AnOutputPortFeedingAnInputPortLinksThePipelines(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	assert.True(t, hasEdge(result, assetMRN("Pipeline", "NiFi Flow/Ingest"), assetMRN("Pipeline", "NiFi Flow/Deliver"), "DEPENDS_ON"))
}

func TestDiscover_PortsAreLeftOutByDefault(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	assert.Nil(t, findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver"))
	assert.Nil(t, findAsset(result, "Task", "NiFi Flow/Deliver/from-ingest"))
}

func TestDiscover_PortsBecomeTasksWhenIncluded(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"include_ports": true})

	out := findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver")
	require.NotNil(t, out)
	assert.Equal(t, "OUTPUT_PORT", out.Metadata["port_type"])
	assert.Equal(t, "STOPPED", out.Metadata["state"])
	assert.True(t, hasEdge(result, assetMRN("Pipeline", "NiFi Flow/Ingest"), *out.MRN, "CONTAINS"))

	in := findAsset(result, "Task", "NiFi Flow/Deliver/from-ingest")
	require.NotNil(t, in)
	assert.Equal(t, "INPUT_PORT", in.Metadata["port_type"])
}

func TestDiscover_PortsJoinTheDependsOnChain(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"include_ports": true})

	stamp := assetMRN("Task", "NiFi Flow/Ingest/Stamp Attributes")
	out := assetMRN("Task", "NiFi Flow/Ingest/to-deliver")
	in := assetMRN("Task", "NiFi Flow/Deliver/from-ingest")
	logOrders := assetMRN("Task", "NiFi Flow/Deliver/Log Orders")

	assert.True(t, hasEdge(result, stamp, out, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, out, in, "DEPENDS_ON"))
	assert.True(t, hasEdge(result, in, logOrders, "DEPENDS_ON"))
}

func TestDiscover_ProcessorsCanBeLeftOut(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"include_processors": false})

	assert.Empty(t, assetNames(result, "Task"))
	assert.Empty(t, assetNames(result, "Topic"), "data lineage hangs off tasks")
	assert.Len(t, assetNames(result, "Pipeline"), 3)
}

func TestDiscover_SkipsAChildGroupThatFailsToLoad(t *testing.T) {
	f := newFakeNiFi().withRoot(standardFlow()).failingAt("/flow/process-groups/pg-deliver")

	result := discover(t, f, nil)

	assert.ElementsMatch(t, []string{"NiFi Flow", "NiFi Flow/Ingest"}, assetNames(result, "Pipeline"))
	assert.NotNil(t, findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders"))
}

func TestDiscover_CanStartFromAConfiguredGroup(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"root_process_group": "pg-ingest"})

	assert.Equal(t, []string{"NiFi Flow/Ingest"}, assetNames(result, "Pipeline"), "the name still comes from the breadcrumb, so it matches a full run")
	assert.NotNil(t, findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders"))
	assert.Nil(t, findAsset(result, "Task", "NiFi Flow/Deliver/Log Orders"))
}

func TestDiscover_PipelineCarriesTheGroupsMetadata(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	require.NotNil(t, ingest)
	assert.Equal(t, "pg-ingest", ingest.Metadata["id"])
	assert.Equal(t, "pg-root", ingest.Metadata["parent_id"])
	assert.Equal(t, "NiFi Flow/Ingest", ingest.Metadata["path"])
	assert.Equal(t, "Lands raw orders in S3 and Kafka", ingest.Metadata["comments"])
	assert.Equal(t, 1, ingest.Metadata["running_count"])
	assert.Equal(t, 2, ingest.Metadata["stopped_count"])
	assert.Equal(t, 2, ingest.Metadata["invalid_count"])
	assert.Equal(t, 0, ingest.Metadata["disabled_count"])
	assert.Equal(t, 4, ingest.Metadata["processor_count"])
	assert.Equal(t, 4, ingest.Metadata["connection_count"])
	assert.Equal(t, 0, ingest.Metadata["input_port_count"])
	assert.Equal(t, 1, ingest.Metadata["output_port_count"])
	assert.Equal(t, "2.11.0", ingest.Metadata["nifi_version"])
	assert.Contains(t, ingest.Metadata["url"], "pg-ingest")
}

func TestDiscover_RootPipelineHasNoParent(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	root := findAsset(result, "Pipeline", "NiFi Flow")
	require.NotNil(t, root)
	assert.NotContains(t, root.Metadata, "parent_id")
	assert.NotContains(t, root.Metadata, "comments")
	assert.Nil(t, root.Description)
}

func TestDiscover_GroupCommentsBecomeTheDescription(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	require.NotNil(t, ingest)
	require.NotNil(t, ingest.Description)
	assert.Equal(t, "Lands raw orders in S3 and Kafka", *ingest.Description)
}

func TestDiscover_TaskCarriesTheProcessorsMetadata(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	require.NotNil(t, gen)
	assert.Equal(t, "p-gen", gen.Metadata["id"])
	assert.Equal(t, "pg-ingest", gen.Metadata["group_id"])
	assert.Equal(t, "NiFi Flow/Ingest", gen.Metadata["pipeline"])
	assert.Equal(t, "GenerateFlowFile", gen.Metadata["type"])
	assert.Equal(t, "org.apache.nifi.processors.standard.GenerateFlowFile", gen.Metadata["type_full"])
	assert.Equal(t, "RUNNING", gen.Metadata["state"])
	assert.Equal(t, "1 hour", gen.Metadata["scheduling_period"])
	assert.Equal(t, "TIMER_DRIVEN", gen.Metadata["scheduling_strategy"])
	assert.Equal(t, []string{"success"}, gen.Metadata["relationships"])
	assert.Contains(t, gen.Metadata["url"], "p-gen")
	assert.Contains(t, gen.Metadata["url"], "pg-ingest")
}

func TestDiscover_TaskPropertiesAreKept(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	pub := findAsset(result, "Task", "NiFi Flow/Ingest/Publish Orders")
	require.NotNil(t, pub)
	assert.Equal(t, map[string]any{"Topic Name": "orders-events"}, pub.Metadata["properties"])
}

func TestDiscover_MasksSensitiveProperties(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").add(
		processor("p1", "Notify Webhook", "org.apache.nifi.processors.standard.InvokeHTTP").
			with("HTTP URL", "https://hooks.example.com/orders").with("Request Username", "nifi").secret("Request Password"))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	task := findAsset(result, "Task", "NiFi Flow/Notify Webhook")
	require.NotNil(t, task)
	assert.Equal(t, map[string]any{
		"HTTP URL":         "https://hooks.example.com/orders",
		"Request Username": "nifi",
		"Request Password": "****",
	}, task.Metadata["properties"])
}

func TestDiscover_ReportsAStoppedInvalidProcessorAsInvalid(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	s3 := findAsset(result, "Task", "NiFi Flow/Ingest/Land in S3")
	require.NotNil(t, s3)
	assert.Equal(t, "INVALID", s3.Metadata["state"])
}

func TestDiscover_ReportsADisabledProcessor(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").add(processor("p1", "Old", "org.apache.nifi.processors.standard.LogAttribute").disabled())
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	assert.Equal(t, "DISABLED", findAsset(result, "Task", "NiFi Flow/Old").Metadata["state"])
}

func TestDiscover_ProcessorCommentsBecomeTheDescription(t *testing.T) {
	root := newGroup("pg-root", "NiFi Flow").add(processor("p1", "Notify", "org.apache.nifi.processors.standard.InvokeHTTP").commented("Posts each order to the webhook"))
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	task := findAsset(result, "Task", "NiFi Flow/Notify")
	require.NotNil(t, task.Description)
	assert.Equal(t, "Posts each order to the webhook", *task.Description)
	assert.Equal(t, "Posts each order to the webhook", task.Metadata["comments"])
}

func TestDiscover_AssetsLinkBackToNiFi(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	for _, asset := range result.Assets {
		if asset.Type == "Topic" {
			continue
		}
		require.Len(t, asset.ExternalLinks, 1, *asset.Name)
		assert.Equal(t, "Open in NiFi", asset.ExternalLinks[0].Name)
		assert.True(t, strings.HasPrefix(asset.ExternalLinks[0].URL, "http://127.0.0.1:"), asset.ExternalLinks[0].URL)
		assert.Equal(t, asset.Metadata["url"], asset.ExternalLinks[0].URL)
	}
}

func TestDiscover_AssetsRecordNiFiAsTheirSource(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	for _, asset := range result.Assets {
		require.Len(t, asset.Sources, 1, *asset.Name)
		assert.Equal(t, "NiFi", asset.Sources[0].Name)
		assert.Equal(t, 1, asset.Sources[0].Priority)
		assert.False(t, asset.Sources[0].LastSyncAt.IsZero())
	}
}

func TestDiscover_AppliesConfiguredTags(t *testing.T) {
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"tags": []any{"nifi", "team-data"}})

	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	require.NotNil(t, gen)
	assert.Equal(t, []string{"nifi", "team-data"}, gen.Tags)
}

func TestDiscover_EmitsEachEdgeOnce(t *testing.T) {
	// Two connections from Stamp to Land in S3 (say, success and failure)
	// express one dependency.
	root := newGroup("pg-root", "NiFi Flow").
		add(processor("p-a", "A", "org.apache.nifi.processors.standard.LogAttribute")).
		add(processor("p-b", "B", "org.apache.nifi.processors.standard.LogAttribute")).
		connect("p-a", "p-b", "success").
		connect("p-a", "p-b", "failure")
	result := discover(t, newFakeNiFi().withRoot(root), nil)

	count := 0
	for _, edge := range result.Lineage {
		if edge.Type == "DEPENDS_ON" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestDiscover_EveryEdgeEndsOnAnAssetOfThisRunOrAnotherPlugins(t *testing.T) {
	// The server drops edges whose endpoint does not exist, so a NiFi
	// endpoint must be an asset of this run; the only foreign endpoints
	// are the buckets, tables and topics other plugins own.
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), nil)

	own := make(map[string]bool)
	for _, asset := range result.Assets {
		own[*asset.MRN] = true
	}
	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.Parse(endpoint)
			require.NoError(t, err)
			if parsed.Service == "nifi" {
				assert.True(t, own[endpoint], "NiFi endpoint %s is not an asset of this run", endpoint)
			}
		}
	}
	assert.True(t, hasEdge(result, assetMRN("Task", "NiFi Flow/Ingest/Land in S3"), "mrn://bucket/s3/marmot-landing", "PRODUCES"))
}

func TestDiscover_UsesAnUnverifiedTLSConnectionWhenAsked(t *testing.T) {
	// httptest's TLS server has a certificate nothing trusts, which is
	// exactly the self-signed situation verify_ssl: false is for.
	f := newFakeNiFi().withRoot(standardFlow())
	f.root.register(f, nil)
	server := f.startTLS(t)

	config := pluginsdk.RawConfig{"host": server.URL, "username": f.username, "password": f.password}

	_, err := (&Source{}).Discover(t.Context(), config)
	require.Error(t, err, "the default is to verify the certificate")

	config["verify_ssl"] = false
	result, err := (&Source{}).Discover(t.Context(), config)
	require.NoError(t, err)
	assert.NotNil(t, findAsset(result, "Pipeline", "NiFi Flow"))
}

// The real NiFi 2.11.0 responses, captured after seeding the same flow the
// e2e tests use, prove the typed structs read what NiFi actually sends.

func realNiFi() *fakeNiFi {
	return newFakeNiFi().
		withRawGroup(fixtureRootID, fixtureRootGroup).
		withRawFlow(fixtureRootID, fixtureRootFlow).
		withRawFlow(fixtureIngestID, fixtureIngestFlow).
		withRawFlow(fixtureDeliverID, fixtureDeliverFlow).
		withRawService(fixturePoolID, fixturePool)
}

func TestDiscover_ReadsTheRealProcessGroupTree(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	assert.ElementsMatch(t, []string{"NiFi Flow", "NiFi Flow/Ingest", "NiFi Flow/Deliver"}, assetNames(result, "Pipeline"))
	assert.ElementsMatch(t, []string{
		"NiFi Flow/Ingest/Generate Orders", "NiFi Flow/Ingest/Stamp Attributes", "NiFi Flow/Ingest/Publish Orders",
		"NiFi Flow/Deliver/Log Orders", "NiFi Flow/Deliver/Notify Webhook",
	}, assetNames(result, "Task"))
}

func TestDiscover_ReadsTheRealProcessorDetails(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	require.NotNil(t, gen)
	assert.Equal(t, "RUNNING", gen.Metadata["state"])
	assert.Equal(t, "1 hour", gen.Metadata["scheduling_period"])
	assert.Equal(t, "GenerateFlowFile", gen.Metadata["type"])

	pub := findAsset(result, "Task", "NiFi Flow/Ingest/Publish Orders")
	require.NotNil(t, pub)
	assert.Equal(t, "INVALID", pub.Metadata["state"])
	assert.Equal(t, []string{"failure", "success"}, pub.Metadata["relationships"])
}

func TestDiscover_ReadsTheRealConnections(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	assert.True(t, hasEdge(result, assetMRN("Task", "NiFi Flow/Ingest/Generate Orders"), assetMRN("Task", "NiFi Flow/Ingest/Stamp Attributes"), "DEPENDS_ON"))
	assert.True(t, hasEdge(result, assetMRN("Task", "NiFi Flow/Ingest/Stamp Attributes"), assetMRN("Task", "NiFi Flow/Ingest/Publish Orders"), "DEPENDS_ON"))
	assert.True(t, hasEdge(result, assetMRN("Pipeline", "NiFi Flow/Ingest"), assetMRN("Pipeline", "NiFi Flow/Deliver"), "DEPENDS_ON"))
	assert.True(t, hasEdge(result, assetMRN("Task", "NiFi Flow/Deliver/Log Orders"), assetMRN("Task", "NiFi Flow/Deliver/Notify Webhook"), "DEPENDS_ON"))
}

func TestDiscover_ReadsTheRealGroupCounts(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	require.NotNil(t, ingest)
	assert.Equal(t, fixtureIngestID, ingest.Metadata["id"])
	assert.Equal(t, fixtureRootID, ingest.Metadata["parent_id"])
	assert.Equal(t, 1, ingest.Metadata["running_count"])
	assert.Equal(t, 2, ingest.Metadata["invalid_count"])
	assert.Equal(t, "2.11.0", ingest.Metadata["nifi_version"])
	assert.Equal(t, "Lands raw orders in S3 and Kafka", *ingest.Description)
}

func TestDiscover_MasksTheRealSensitiveProperty(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	hook := findAsset(result, "Task", "NiFi Flow/Deliver/Notify Webhook")
	require.NotNil(t, hook)
	assert.Equal(t, map[string]any{
		"HTTP URL":         "https://hooks.example.com/orders",
		"HTTP Method":      "POST",
		"Request Username": "nifi",
		"Request Password": "****",
	}, hook.Metadata["properties"])
	assert.Equal(t, "Posts each order to the fulfilment webhook", *hook.Description)
}

func TestDiscover_ReadsTheRealKafkaTopic(t *testing.T) {
	result := discover(t, realNiFi(), nil)

	assert.True(t, hasEdge(result, assetMRN("Task", "NiFi Flow/Ingest/Publish Orders"), "mrn://topic/kafka/orders-events", "PRODUCES"))
	assert.NotNil(t, findAsset(result, "Topic", "orders-events"))
}

func TestDiscover_ReadsTheRealPortsWhenIncluded(t *testing.T) {
	result := discover(t, realNiFi(), pluginsdk.RawConfig{"include_ports": true})

	out := findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver")
	require.NotNil(t, out)
	assert.Equal(t, "OUTPUT_PORT", out.Metadata["port_type"])
	assert.True(t, hasEdge(result, *out.MRN, assetMRN("Task", "NiFi Flow/Deliver/from-ingest"), "DEPENDS_ON"))
}

func TestDiscover_ReadsTheRealConnectionPool(t *testing.T) {
	// The captured pool is PostgreSQL; a QueryDatabaseTable pointing at it
	// resolves to the PostgreSQL plugin's table.
	f := realNiFi()
	ingest := strings.Replace(fixtureIngestFlow, `"processors": [`, `"processors": [`+marshal(
		processor("p-read", "Read Customers", "org.apache.nifi.processors.standard.QueryDatabaseTable").
			with("Table Name", "customers").with("Database Connection Pooling Service", fixturePoolID).render(fixtureIngestID))+",", 1)
	f.withRawFlow(fixtureIngestID, ingest)

	result := discover(t, f, nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", assetMRN("Task", "NiFi Flow/Ingest/Read Customers"), "FEEDS"))
}

func TestDiscover_LinksIntoTheNiFi2UIByHashRoute(t *testing.T) {
	// NiFi 2 selects a component through /nifi/#/process-groups/<group>/<kind>/<id>.
	result := discover(t, newFakeNiFi().withRoot(standardFlow()), pluginsdk.RawConfig{"include_ports": true})

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	assert.True(t, strings.HasSuffix(ingest.Metadata["url"].(string), "/nifi/#/process-groups/pg-ingest"), ingest.Metadata["url"])

	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	assert.True(t, strings.HasSuffix(gen.Metadata["url"].(string), "/nifi/#/process-groups/pg-ingest/processors/p-gen"), gen.Metadata["url"])

	out := findAsset(result, "Task", "NiFi Flow/Ingest/to-deliver")
	assert.True(t, strings.HasSuffix(out.Metadata["url"].(string), "/nifi/#/process-groups/pg-ingest/output-ports/port-out"), out.Metadata["url"])

	in := findAsset(result, "Task", "NiFi Flow/Deliver/from-ingest")
	assert.True(t, strings.HasSuffix(in.Metadata["url"].(string), "/nifi/#/process-groups/pg-deliver/input-ports/port-in"), in.Metadata["url"])
}

func TestDiscover_LinksIntoTheNiFi1UIByQueryString(t *testing.T) {
	// The NiFi 1 UI selected components through query parameters.
	f := newFakeNiFi().withRoot(standardFlow())
	f.version = "1.28.1"
	result := discover(t, f, nil)

	ingest := findAsset(result, "Pipeline", "NiFi Flow/Ingest")
	assert.True(t, strings.HasSuffix(ingest.Metadata["url"].(string), "/nifi/?processGroupId=pg-ingest"), ingest.Metadata["url"])

	gen := findAsset(result, "Task", "NiFi Flow/Ingest/Generate Orders")
	assert.True(t, strings.HasSuffix(gen.Metadata["url"].(string), "/nifi/?processGroupId=pg-ingest&componentIds=p-gen"), gen.Metadata["url"])
}
