package spline

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://spline:8080"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_RejectsANonURLHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "spline"})
	require.Error(t, err)
}

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://spline:8080"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 7, s.config.Days)
	assert.Equal(t, 1000, s.config.MaxEvents)
	assert.Equal(t, 100, s.config.PageSize)
	assert.True(t, s.config.VerifySSL)
	assert.True(t, s.config.IncludeColumnLineage)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":                   "http://spline:8080",
		"verify_ssl":             false,
		"include_column_lineage": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.VerifySSL)
	assert.False(t, s.config.IncludeColumnLineage)
}

func TestValidate_RejectsBothTokenAndUsername(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":     "http://spline:8080",
		"token":    "secret",
		"username": "admin",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "http://spline:8080",
		"filter": map[string]interface{}{
			"include": []interface{}{"^daily.*"},
			"exclude": []interface{}{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

// Users copy the consumer or producer URL out of their deployment, and both
// sit one level below the root the plugin appends its own paths to.

func TestNormaliseHost_StripsAConsumerSuffix(t *testing.T) {
	assert.Equal(t, "http://spline:8080", normaliseHost("http://spline:8080/consumer"))
}

func TestNormaliseHost_StripsAProducerSuffix(t *testing.T) {
	assert.Equal(t, "http://spline:8080", normaliseHost("http://spline:8080/producer/"))
}

func TestNormaliseHost_StripsATrailingSlash(t *testing.T) {
	assert.Equal(t, "http://spline:8080", normaliseHost("http://spline:8080/"))
}

func TestNormaliseHost_LeavesAPathPrefixAlone(t *testing.T) {
	// A gateway behind a reverse proxy can live under a path of its own.
	assert.Equal(t, "http://proxy/spline", normaliseHost("http://proxy/spline"))
}

func TestValidate_NormalisesTheHost(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "http://spline:8080/consumer"})
	require.NoError(t, err)

	assert.Equal(t, "http://spline:8080", s.config.Host)
}

// Grouping by application is the difference from OpenMetadata's connector,
// which makes a new pipeline entity for every Spark run.

func TestGroupByApplication_MergesRunsOfTheSameApplication(t *testing.T) {
	events := []ExecutionEvent{
		{ExecutionEventID: "p1:b", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 2000},
		{ExecutionEventID: "p1:a", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 1000},
	}

	apps := groupByApplication(events)

	require.Len(t, apps, 1)
	assert.Equal(t, "daily-etl", apps[0].Name)
	assert.Len(t, apps[0].Events, 2)
}

func TestGroupByApplication_KeepsDifferentApplicationsApart(t *testing.T) {
	events := []ExecutionEvent{
		{ExecutionEventID: "p1:a", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 2000},
		{ExecutionEventID: "p2:a", ExecutionPlanID: "p2", ApplicationName: "events-loader", Timestamp: 1000},
	}

	apps := groupByApplication(events)

	require.Len(t, apps, 2)
	assert.Equal(t, "daily-etl", apps[0].Name)
	assert.Equal(t, "events-loader", apps[1].Name)
}

func TestGroupByApplication_OrdersRunsNewestFirst(t *testing.T) {
	events := []ExecutionEvent{
		{ExecutionEventID: "p1:old", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 1000},
		{ExecutionEventID: "p1:new", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 5000},
	}

	apps := groupByApplication(events)

	require.Len(t, apps, 1)
	assert.Equal(t, "p1:new", apps[0].Events[0].ExecutionEventID)
}

func TestGroupByApplication_CollectsDistinctPlanIDs(t *testing.T) {
	events := []ExecutionEvent{
		{ExecutionEventID: "p2:a", ExecutionPlanID: "p2", ApplicationName: "daily-etl", Timestamp: 3000},
		{ExecutionEventID: "p1:b", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 2000},
		{ExecutionEventID: "p1:a", ExecutionPlanID: "p1", ApplicationName: "daily-etl", Timestamp: 1000},
	}

	apps := groupByApplication(events)

	require.Len(t, apps, 1)
	assert.Equal(t, []string{"p2", "p1"}, apps[0].PlanIDs)
}

func TestApplicationName_FallsBackToTheExtraAppName(t *testing.T) {
	event := ExecutionEvent{ExecutionPlanID: "p1", Extra: map[string]any{"appName": "daily-etl"}}

	assert.Equal(t, "daily-etl", applicationName(event))
}

func TestApplicationName_FallsBackToThePlanID(t *testing.T) {
	event := ExecutionEvent{ExecutionPlanID: "p1"}

	assert.Equal(t, "p1", applicationName(event))
}

// Errors arrive as free-form JSON, so the plugin has to cope with more than a
// plain string before it can decide a run failed.

func TestErrorMessage_ReadsTheMessageField(t *testing.T) {
	assert.Equal(t, "Job aborted", errorMessage(map[string]any{"message": "Job aborted"}))
}

func TestErrorMessage_AcceptsAPlainString(t *testing.T) {
	assert.Equal(t, "boom", errorMessage("boom"))
}

func TestErrorMessage_TreatsNilAsSuccess(t *testing.T) {
	assert.Equal(t, "", errorMessage(nil))
}

func TestErrorMessage_TreatsAnEmptyObjectAsSuccess(t *testing.T) {
	assert.Equal(t, "", errorMessage(map[string]any{}))
}

func TestErrorMessage_EncodesAnUnrecognisedShape(t *testing.T) {
	assert.Equal(t, `["a","b"]`, errorMessage([]any{"a", "b"}))
}

// Column lineage walks the attribute graph, where an edge points from a
// derived column to the column it came from.

func originNames(nodes []AttributeNode) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func TestOriginColumns_FollowsEdgesToTheSourceColumns(t *testing.T) {
	graph := AttributeGraph{
		Nodes: []AttributeNode{
			{ID: "revenue", Name: "revenue"},
			{ID: "total", Name: "total"},
			{ID: "order_id", Name: "order_id"},
		},
		Edges: []AttributeEdge{
			{Source: "revenue", Target: "total"},
			{Source: "revenue", Target: "order_id"},
		},
	}

	origins, derived := originColumns(graph, "revenue")
	assert.Equal(t, []string{"order_id", "total"}, originNames(origins))
	assert.True(t, derived)
}

func TestOriginColumns_IsTheColumnItselfWhenReadStraightFromTheInput(t *testing.T) {
	graph := AttributeGraph{
		Nodes: []AttributeNode{{ID: "total", Name: "total", OriginOpID: "read"}},
	}

	origins, derived := originColumns(graph, "total")
	assert.Equal(t, []AttributeNode{{ID: "total", Name: "total", OriginOpID: "read"}}, origins)
	assert.False(t, derived)
}

func TestOriginColumns_SkipsIntermediateColumns(t *testing.T) {
	// revenue is computed inside the plan, so only the column it came from
	// belongs to an input.
	graph := AttributeGraph{
		Nodes: []AttributeNode{
			{ID: "margin", Name: "margin"},
			{ID: "revenue", Name: "revenue"},
			{ID: "total", Name: "total"},
		},
		Edges: []AttributeEdge{
			{Source: "margin", Target: "revenue"},
			{Source: "revenue", Target: "total"},
		},
	}

	origins, _ := originColumns(graph, "margin")
	assert.Equal(t, []string{"total"}, originNames(origins))
}

func TestOriginColumns_SurvivesACycle(t *testing.T) {
	graph := AttributeGraph{
		Nodes: []AttributeNode{{ID: "a", Name: "a"}, {ID: "b", Name: "b"}},
		Edges: []AttributeEdge{{Source: "a", Target: "b"}, {Source: "b", Target: "a"}},
	}

	origins, derived := originColumns(graph, "a")
	assert.Empty(t, origins, "a cycle has no column it was read from")
	assert.True(t, derived)
}

func TestTransformOf_LabelsOnlyRealDerivations(t *testing.T) {
	assert.Equal(t, "", transformOf([]string{"total"}, "total", false))
	assert.Equal(t, "", transformOf([]string{"total"}, "TOTAL", true))
	assert.Equal(t, "expression", transformOf([]string{"id"}, "order_id", true))
	assert.Equal(t, "expression", transformOf([]string{"order_id", "total"}, "revenue", true))
}

// Discovery against the fake server.

func discoverAgainstFake(t *testing.T, fake *fakeSpline, extra pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := pluginsdk.RawConfig{"host": fake.URL()}
	for key, value := range extra {
		config[key] = value
	}

	source := &Source{}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func assetsByName(result *pluginsdk.DiscoveryResult) map[string]pluginsdk.Asset {
	byName := make(map[string]pluginsdk.Asset, len(result.Assets))
	for _, asset := range result.Assets {
		byName[*asset.Name] = asset
	}
	return byName
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestDiscover_MakesOnePipelinePerApplication(t *testing.T) {
	// Four runs, but only three applications: the pipeline count is what
	// grouping by application buys over one asset per run.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.Len(t, result.Assets, 3)
	byName := assetsByName(result)
	assert.Contains(t, byName, "daily-etl")
	assert.Contains(t, byName, "events-loader")
	assert.Contains(t, byName, "nightly-report")
}

func TestDiscover_CountsTheRunsOfARepeatedApplication(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.Equal(t, 2, assetsByName(result)["daily-etl"].Metadata["execution_count"])
	assert.Equal(t, 1, assetsByName(result)["events-loader"].Metadata["execution_count"])
}

func TestDiscover_RecordsTheLatestRunOnThePipeline(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	metadata := assetsByName(discoverAgainstFake(t, fake, nil))["daily-etl"].Metadata

	assert.Equal(t, planDaily+":mtsf3vmm", metadata["last_execution_id"])
	assert.Equal(t, int64(9500), metadata["last_duration_ms"])
	assert.Equal(t, "spark 3.5.0", metadata["framework"])
	assert.NotContains(t, metadata, "last_error")
}

func TestDiscover_RecordsTheErrorOfAFailedRun(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	metadata := assetsByName(discoverAgainstFake(t, fake, nil))["nightly-report"].Metadata

	assert.Equal(t, "Job aborted due to stage failure", metadata["last_error"])
	// The failed run reported no duration.
	assert.NotContains(t, metadata, "last_duration_ms")
}

func TestDiscover_RecordsTheSparkAndAgentVersions(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	metadata := assetsByName(discoverAgainstFake(t, fake, nil))["daily-etl"].Metadata

	assert.Equal(t, "spark", metadata["system_name"])
	assert.Equal(t, "3.5.0", metadata["system_version"])
	assert.Equal(t, "spline", metadata["agent_name"])
	assert.Equal(t, "1.0.0", metadata["agent_version"])
}

func TestDiscover_KeepsTheRecentApplicationIDs(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	metadata := assetsByName(discoverAgainstFake(t, fake, nil))["daily-etl"].Metadata

	assert.Equal(t,
		[]string{"application_1699000000000_0002", "application_1699000000000_0001"},
		metadata["application_ids"])
}

func TestDiscover_KeepsTheRawDataSourceURIs(t *testing.T) {
	// Whatever the URI parser cannot resolve is still visible on the asset.
	fake := newFakeSpline(t, fixtureEvents())

	metadata := assetsByName(discoverAgainstFake(t, fake, nil))["daily-etl"].Metadata

	assert.Equal(t, []string{
		"jdbc:postgresql://pg.internal:5432/warehouse:public.orders",
		"jdbc:postgresql://pg.internal:5432/warehouse:public.customers",
	}, metadata["inputs"])
	assert.Equal(t, []string{
		"jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary",
	}, metadata["outputs"])
}

func TestDiscover_LinksInputTablesToThePipeline(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.True(t, hasEdge(result, "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.orders", "mrn://asset/pipeline/spline/daily-etl", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.customers", "mrn://asset/pipeline/spline/daily-etl", "FEEDS"))
}

func TestDiscover_LinksThePipelineToItsOutputTable(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.True(t, hasEdge(result, "mrn://asset/pipeline/spline/daily-etl", "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.order_summary", "PRODUCES"))
}

func TestDiscover_LinksAnS3BucketToThePipeline(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.True(t, hasEdge(result, "mrn://asset/bucket/s3/marmot-lake", "mrn://asset/pipeline/spline/events-loader", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://asset/pipeline/spline/events-loader", "mrn://asset/table/hive/sales.orders", "PRODUCES"))
}

func TestDiscover_DoesNotCreateTablesOrBuckets(t *testing.T) {
	// The plugins that own those assets create them; Spline only points at them.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	for _, asset := range result.Assets {
		assert.Equal(t, "Pipeline", asset.Type)
	}
}

func TestDiscover_EmitsEachEdgeOnce(t *testing.T) {
	// daily-etl ran twice through the same plan.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	seen := map[string]int{}
	for _, edge := range result.Lineage {
		seen[edge.Source+edge.Target+edge.Type]++
	}
	for key, count := range seen {
		assert.Equal(t, 1, count, key)
	}
}

func TestDiscover_FetchesAnExecutionPlanOncePerPlan(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	discoverAgainstFake(t, fake, nil)

	assert.Equal(t, 3, fake.Requests["/consumer/lineage-detailed"])
}

func TestDiscover_BuildsRunHistoryForEveryRun(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	byMRN := map[string]pluginsdk.AssetRunHistory{}
	for _, history := range result.RunHistory {
		byMRN[history.AssetMRN] = history
	}

	daily := byMRN["mrn://asset/pipeline/spline/daily-etl"]
	require.Len(t, daily.Runs, 2)
	assert.Equal(t, planDaily+":mtsf3vmm", daily.Runs[0].RunID)
	assert.Equal(t, "spline", daily.Runs[0].JobNamespace)
	assert.Equal(t, "daily-etl", daily.Runs[0].JobName)
	assert.Equal(t, "COMPLETE", daily.Runs[0].EventType)
}

func TestDiscover_MarksAFailedRunAsFail(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	for _, history := range result.RunHistory {
		if history.AssetMRN != "mrn://asset/pipeline/spline/nightly-report" {
			continue
		}
		require.Len(t, history.Runs, 1)
		assert.Equal(t, "FAIL", history.Runs[0].EventType)
		return
	}
	t.Fatal("no run history for nightly-report")
}

func TestDiscover_AttachesRunFacets(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	for _, history := range result.RunHistory {
		if history.AssetMRN != "mrn://asset/pipeline/spline/daily-etl" {
			continue
		}
		facets := history.Runs[0].RunFacets
		assert.Equal(t, "application_1699000000000_0002", facets["application_id"])
		assert.Equal(t, int64(9500), facets["duration_ms"])
		assert.Equal(t, planDaily, facets["execution_plan_id"])
		assert.Equal(t, "jdbc:postgresql://pg.internal:5432/warehouse:public.order_summary", facets["output"])
		assert.Equal(t, false, facets["append"])
		return
	}
	t.Fatal("no run history for daily-etl")
}

func findEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) *pluginsdk.LineageEdge {
	for i, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return &result.Lineage[i]
		}
	}
	return nil
}

const (
	ordersMRN       = "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.orders"
	customersMRN    = "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.customers"
	orderSummaryMRN = "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.order_summary"
)

func TestDiscover_PutsColumnLineageOnAnEdgeFromEachInputTable(t *testing.T) {
	// daily-etl joins orders and customers. Each written column is traced back
	// to the Read operation it came from, so the columns land on the edge from
	// the right table.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	orders := findEdge(result, ordersMRN, orderSummaryMRN, "DEPENDS_ON")
	require.NotNil(t, orders)
	assert.Equal(t, "mrn://asset/pipeline/spline/daily-etl", orders.JobMRN)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"total"}, ToColumn: "total"},
		{FromColumns: []string{"order_id", "total"}, ToColumn: "revenue", Transform: "expression"},
	}, orders.ColumnLineage)

	customers := findEdge(result, customersMRN, orderSummaryMRN, "DEPENDS_ON")
	require.NotNil(t, customers)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"customer_name"}, ToColumn: "customer_name"},
	}, customers.ColumnLineage)
}

func TestDiscover_LooksUpOnlyTheColumnsThePlanWrote(t *testing.T) {
	// order_id is in daily-etl's attribute list but not in its write's schema.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	for _, edge := range result.Lineage {
		if edge.Target != orderSummaryMRN {
			continue
		}
		for _, column := range edge.ColumnLineage {
			assert.NotEqual(t, "order_id", column.ToColumn, "order_id was not written by daily-etl")
		}
	}
}

func TestDiscover_ResolvesASingleInputPlanWithoutAskingItsReads(t *testing.T) {
	// nightly-report reads only orders, so every read column came from it. Its
	// write cannot be read, so the plan's attribute list is used instead.
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	edge := findEdge(result, ordersMRN, "mrn://asset/bucket/s3/marmot-lake", "DEPENDS_ON")
	require.NotNil(t, edge)
	assert.Equal(t, "mrn://asset/pipeline/spline/nightly-report", edge.JobMRN)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"id"}, ToColumn: "order_id", Transform: "expression"},
	}, edge.ColumnLineage)

	// daily-etl's write and its two reads, and nightly-report's write.
	assert.Equal(t, 4, fake.Requests["/consumer/operations"])
}

func TestDiscover_EmitsNoColumnEdgeForAPlanWithoutColumnLineage(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.Nil(t, findEdge(result, "mrn://asset/bucket/s3/marmot-lake", "mrn://asset/table/hive/sales.orders", "DEPENDS_ON"))
}

func TestDiscover_NoLongerStoresColumnLineageAsMetadata(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.NotContains(t, assetsByName(result)["daily-etl"].Metadata, "column_lineage")
}

func TestDiscover_SkipsColumnLineageWhenDisabled(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"include_column_lineage": false})

	for _, edge := range result.Lineage {
		assert.Empty(t, edge.ColumnLineage)
		assert.NotEqual(t, "DEPENDS_ON", edge.Type)
	}
	assert.Zero(t, fake.Requests["/consumer/attribute-lineage-and-impact"])
	assert.Zero(t, fake.Requests["/consumer/operations"])
}

func TestAddColumnEdge_MergesColumnsFromAnotherPlan(t *testing.T) {
	var edges []pluginsdk.LineageEdge
	index := map[string]int{}

	addColumnEdge(&edges, index, pluginsdk.LineageEdge{Source: "a", Target: "b", Type: "DEPENDS_ON", JobMRN: "job-1",
		ColumnLineage: []pluginsdk.ColumnEdge{{FromColumns: []string{"x"}, ToColumn: "x"}}})
	addColumnEdge(&edges, index, pluginsdk.LineageEdge{Source: "a", Target: "b", Type: "DEPENDS_ON", JobMRN: "job-2",
		ColumnLineage: []pluginsdk.ColumnEdge{{FromColumns: []string{"x"}, ToColumn: "x"}, {FromColumns: []string{"y"}, ToColumn: "z"}}})

	require.Len(t, edges, 1)
	assert.Equal(t, "job-1", edges[0].JobMRN)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"x"}, ToColumn: "x"},
		{FromColumns: []string{"y"}, ToColumn: "z"},
	}, edges[0].ColumnLineage)
}

func TestDiscover_AddsAnExternalLinkWhenTheUIHostIsSet(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"ui_host": "http://spline-ui:9090"})

	asset := assetsByName(result)["daily-etl"]
	require.Len(t, asset.ExternalLinks, 1)
	assert.Equal(t, "Open in Spline", asset.ExternalLinks[0].Name)
	assert.Equal(t, "http://spline-ui:9090/app/events/overview/"+planDaily+":mtsf3vmm", asset.ExternalLinks[0].URL)
	assert.Equal(t, asset.ExternalLinks[0].URL, asset.Metadata["url"])
}

func TestDiscover_OmitsTheExternalLinkWithoutAUIHost(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, nil)

	assert.Empty(t, assetsByName(result)["daily-etl"].ExternalLinks)
	assert.NotContains(t, assetsByName(result)["daily-etl"].Metadata, "url")
}

func TestDiscover_AppliesTags(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"tags": []any{"spark"}})

	assert.Contains(t, assetsByName(result)["daily-etl"].Tags, "spark")
}

func TestDiscover_SendsTheConfiguredPageSize(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	discoverAgainstFake(t, fake, pluginsdk.RawConfig{"page_size": 2})

	assert.Equal(t, "2", fake.PageSizeSeen)
}

func TestDiscover_FollowsPaging(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"page_size": 1})

	assert.Len(t, result.Assets, 3)
}

func TestDiscover_StopsWhenTheServerIgnoresPageNum(t *testing.T) {
	// A gateway that keeps serving page one would otherwise loop forever.
	fake := newFakeSpline(t, fixtureEvents())
	fake.IgnorePaging = true

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"page_size": 1})

	assert.Len(t, result.Assets, 1)
}

func TestDiscover_StopsAtMaxEvents(t *testing.T) {
	fake := newFakeSpline(t, fixtureEvents())

	result := discoverAgainstFake(t, fake, pluginsdk.RawConfig{"max_events": 1})

	assert.Len(t, result.Assets, 1)
	assert.Equal(t, "nightly-report", *result.Assets[0].Name)
}

func TestDiscover_UnreachableServerIsAnError(t *testing.T) {
	source := &Source{}

	_, err := source.Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1"})

	require.Error(t, err)
}

func TestDiscover_ContinuesWhenOnePlanCannotBeRead(t *testing.T) {
	// One unreadable plan costs its lineage, not the whole run.
	events := fixtureEvents()
	events[0]["executionPlanId"] = "missing-plan"
	fake := newFakeSpline(t, events)

	result := discoverAgainstFake(t, fake, nil)

	assert.Len(t, result.Assets, 3)
	assert.True(t, hasEdge(result, "mrn://asset/pipeline/spline/daily-etl", "mrn://asset/table/postgresql@host=pg.internal:5432/warehouse.public.order_summary", "PRODUCES"))
}

func TestMeta_DeclaresWhatDiscoverEmits(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "spline", meta.ID)
	assert.Equal(t, "Spline", meta.Name)
	assert.Equal(t, "spark", meta.Icon)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage", "Run History"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}
