package dagster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringPtr(s string) *string { return &s }

func floatPtr(f float64) *float64 { return &f }

func textEntry(label, text string) MetadataEntry {
	return MetadataEntry{Typename: "TextMetadataEntry", Label: label, Text: text}
}

// Run status mapping. Marmot records START, RUNNING, COMPLETE, FAIL, ABORT
// and OTHER, so every Dagster status has to land on one of those.

func TestMapRunStatus_SuccessCompletes(t *testing.T) {
	assert.Equal(t, "COMPLETE", mapRunStatusToEventType("SUCCESS"))
}

func TestMapRunStatus_FailureFails(t *testing.T) {
	assert.Equal(t, "FAIL", mapRunStatusToEventType("FAILURE"))
}

func TestMapRunStatus_CanceledAborts(t *testing.T) {
	assert.Equal(t, "ABORT", mapRunStatusToEventType("CANCELED"))
}

func TestMapRunStatus_StartedIsRunning(t *testing.T) {
	assert.Equal(t, "RUNNING", mapRunStatusToEventType("STARTED"))
	assert.Equal(t, "RUNNING", mapRunStatusToEventType("STARTING"))
}

func TestMapRunStatus_QueuedHasOnlyStarted(t *testing.T) {
	assert.Equal(t, "START", mapRunStatusToEventType("QUEUED"))
	assert.Equal(t, "START", mapRunStatusToEventType("NOT_STARTED"))
}

func TestMapRunStatus_UnknownStatusIsOther(t *testing.T) {
	assert.Equal(t, "OTHER", mapRunStatusToEventType("SOMETHING_NEW"))
}

// Warehouse table resolution. An edge only lands when it names a table the
// matching Marmot plugin would create, so the mapping stays deliberately
// narrow.

func TestWarehouseTarget_DuckDBUsesTheBareTableName(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("duckdb"),
		MetadataEntries: []MetadataEntry{
			textEntry("database", "analytics"),
			textEntry("schema", "staging"),
			textEntry("table", "clean_orders"),
		},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "DuckDB", target.Provider)
	assert.Equal(t, "clean_orders", target.Name)
}

func TestWarehouseTarget_PostgresUsesTheBareTableName(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("postgres"),
		MetadataEntries: []MetadataEntry{
			textEntry("database", "app"),
			textEntry("schema", "public"),
			textEntry("table", "orders"),
		},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "PostgreSQL", target.Provider)
	assert.Equal(t, "orders", target.Name)
}

func TestWarehouseTarget_PostgresqlSpellingResolvesTheSameWay(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("PostgreSQL"),
		AssetKey:    AssetKey{Path: []string{"app", "public", "orders"}},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "PostgreSQL", target.Provider)
}

func TestWarehouseTarget_BigQueryUsesTheBareTableName(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("bigquery"),
		AssetKey:    AssetKey{Path: []string{"project", "analytics", "orders"}},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "BigQuery", target.Provider)
	assert.Equal(t, "orders", target.Name)
}

func TestWarehouseTarget_SnowflakeUsesTheFullyQualifiedName(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("snowflake"),
		AssetKey:    AssetKey{Path: []string{"ANALYTICS", "STAGING", "ORDERS"}},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "Snowflake", target.Provider)
	assert.Equal(t, "ANALYTICS.STAGING.ORDERS", target.Name)
}

func TestWarehouseTarget_ReadsTheDagsterTableNameConvention(t *testing.T) {
	node := AssetNode{
		ComputeKind:     stringPtr("snowflake"),
		MetadataEntries: []MetadataEntry{textEntry("dagster/table_name", "analytics.staging.orders")},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "analytics.staging.orders", target.Name)
}

func TestWarehouseTarget_ExplicitEntriesBeatTheAssetKey(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("duckdb"),
		AssetKey:    AssetKey{Path: []string{"a", "b", "c"}},
		MetadataEntries: []MetadataEntry{
			textEntry("database", "analytics"),
			textEntry("schema", "staging"),
			textEntry("table", "clean_orders"),
		},
	}

	target, ok := warehouseTarget(node)
	require.True(t, ok)
	assert.Equal(t, "clean_orders", target.Name)
}

func TestWarehouseTarget_UnknownComputeKindResolvesToNothing(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("spark"),
		AssetKey:    AssetKey{Path: []string{"a", "b", "c"}},
	}

	_, ok := warehouseTarget(node)
	assert.False(t, ok)
}

func TestWarehouseTarget_MissingComputeKindResolvesToNothing(t *testing.T) {
	node := AssetNode{AssetKey: AssetKey{Path: []string{"a", "b", "c"}}}

	_, ok := warehouseTarget(node)
	assert.False(t, ok)
}

func TestWarehouseTarget_ShortAssetKeyResolvesToNothing(t *testing.T) {
	// A single-segment key says nothing about a database or schema, so
	// guessing one would invent lineage.
	node := AssetNode{
		ComputeKind: stringPtr("duckdb"),
		AssetKey:    AssetKey{Path: []string{"clean_orders"}},
	}

	_, ok := warehouseTarget(node)
	assert.False(t, ok)
}

func TestWarehouseTarget_PartialEntriesResolveToNothing(t *testing.T) {
	node := AssetNode{
		ComputeKind: stringPtr("duckdb"),
		AssetKey:    AssetKey{Path: []string{"clean_orders"}},
		MetadataEntries: []MetadataEntry{
			textEntry("database", "analytics"),
			textEntry("table", "clean_orders"),
		},
	}

	_, ok := warehouseTarget(node)
	assert.False(t, ok)
}

// Job naming.

func TestPipelineNames_UsesTheBareJobName(t *testing.T) {
	repos := []Repository{{
		Name:     "__repository__",
		Location: Location{Name: "defs.py"},
		Jobs:     []Job{{Name: "etl_job"}},
	}}

	names := pipelineNames(repos)
	assert.Equal(t, "etl_job", names[jobKey(repos[0], "etl_job")])
}

func TestPipelineNames_SkipsTheImplicitAssetJob(t *testing.T) {
	repos := []Repository{{
		Name:     "__repository__",
		Location: Location{Name: "defs.py"},
		Jobs:     []Job{{Name: "__ASSET_JOB"}},
	}}

	assert.Empty(t, pipelineNames(repos))
}

func TestPipelineNames_QualifiesAClashWithItsCodeLocation(t *testing.T) {
	// Two locations can each define a job called etl_job. They are different
	// pipelines, so the second one has to get a distinct name.
	repos := []Repository{
		{Name: "repo", Location: Location{Name: "alpha"}, Jobs: []Job{{Name: "etl_job"}}},
		{Name: "repo", Location: Location{Name: "beta"}, Jobs: []Job{{Name: "etl_job"}}},
	}

	names := pipelineNames(repos)
	assert.Equal(t, "etl_job", names[jobKey(repos[0], "etl_job")])
	assert.Equal(t, "etl_job (beta)", names[jobKey(repos[1], "etl_job")])
}

func TestPipelineNames_ClashResolutionDoesNotDependOnInputOrder(t *testing.T) {
	forward := []Repository{
		{Name: "repo", Location: Location{Name: "alpha"}, Jobs: []Job{{Name: "etl_job"}}},
		{Name: "repo", Location: Location{Name: "beta"}, Jobs: []Job{{Name: "etl_job"}}},
	}
	reversed := []Repository{forward[1], forward[0]}

	assert.Equal(t, pipelineNames(forward), pipelineNames(reversed))
}

func TestSplitPipelineName_RecoversTheJobNameFromAQualifiedName(t *testing.T) {
	assert.Equal(t, "etl_job", splitPipelineName("etl_job (beta)"))
}

func TestSplitPipelineName_LeavesAnUnqualifiedNameAlone(t *testing.T) {
	assert.Equal(t, "etl_job", splitPipelineName("etl_job"))
}

func TestDatasetName_JoinsTheAssetKeyPath(t *testing.T) {
	assert.Equal(t, "raw/orders", datasetName(AssetKey{Path: []string{"raw", "orders"}}))
}

// Metadata handling.

func TestMetadataKey_LowercasesAndReplacesTheNamespaceSeparator(t *testing.T) {
	assert.Equal(t, "dagster_table_name", metadataKey("dagster/table_name"))
}

func TestMetadataKey_ReplacesSpacesAndPunctuation(t *testing.T) {
	assert.Equal(t, "row_count", metadataKey("Row Count"))
}

func TestMetadataKey_DropsSurroundingSeparators(t *testing.T) {
	assert.Equal(t, "label", metadataKey("/label/"))
}

func TestFlattenMetadataEntry_ReadsATextEntry(t *testing.T) {
	key, value, ok := flattenMetadataEntry(textEntry("owner", "data-platform"))
	require.True(t, ok)
	assert.Equal(t, "owner", key)
	assert.Equal(t, "data-platform", value)
}

func TestFlattenMetadataEntry_ReadsAUrlEntry(t *testing.T) {
	key, value, ok := flattenMetadataEntry(MetadataEntry{Typename: "UrlMetadataEntry", Label: "docs_url", URL: "https://example.com"})
	require.True(t, ok)
	assert.Equal(t, "docs_url", key)
	assert.Equal(t, "https://example.com", value)
}

func TestFlattenMetadataEntry_SkipsAnEmptyValue(t *testing.T) {
	_, _, ok := flattenMetadataEntry(textEntry("owner", ""))
	assert.False(t, ok)
}

func TestFlattenMetadataEntry_SkipsAnUnsupportedEntryKind(t *testing.T) {
	_, _, ok := flattenMetadataEntry(MetadataEntry{Typename: "FloatMetadataEntry", Label: "rows"})
	assert.False(t, ok)
}

func TestTagMap_DropsDagstersHiddenTags(t *testing.T) {
	tags := tagMap([]Tag{{Key: "team", Value: "data"}, {Key: ".dagster/internal", Value: "x"}})

	assert.Equal(t, map[string]string{"team": "data"}, tags)
}

func TestTagMap_ReturnsNilWhenNothingIsLeft(t *testing.T) {
	assert.Nil(t, tagMap([]Tag{{Key: ".dagster/internal", Value: "x"}}))
}

func TestCleanMetadata_DropsEmptyValues(t *testing.T) {
	cleaned := cleanMetadata(map[string]any{
		"kept":         "value",
		"empty_string": "",
		"empty_list":   []string{},
		"nil_value":    nil,
		"false_flag":   false,
		"zero_count":   0,
	})

	assert.Equal(t, map[string]any{"kept": "value", "false_flag": false, "zero_count": 0}, cleaned)
}

// Timestamps.

func TestSecondsToTime_ConvertsAnEpochSecondsValue(t *testing.T) {
	at, ok := secondsToTime(floatPtr(1788853304.552549))
	require.True(t, ok)
	assert.Equal(t, "2026-09-08T07:41:44Z", at.UTC().Format("2006-01-02T15:04:05Z"))
}

func TestSecondsToTime_RejectsAMissingValue(t *testing.T) {
	_, ok := secondsToTime(nil)
	assert.False(t, ok)
}

func TestSecondsToTime_RejectsAZeroValue(t *testing.T) {
	_, ok := secondsToTime(floatPtr(0))
	assert.False(t, ok)
}

func TestMillisToTime_ConvertsAMaterializationTimestamp(t *testing.T) {
	at, ok := millisToTime("1788853307723")
	require.True(t, ok)
	assert.Equal(t, "2026-09-08T07:41:47Z", at.UTC().Format("2006-01-02T15:04:05Z"))
}

func TestMillisToTime_RejectsANonNumericValue(t *testing.T) {
	_, ok := millisToTime("not-a-number")
	assert.False(t, ok)
}

// Run summaries.

func TestRunSummary_IsEmptyWithoutRuns(t *testing.T) {
	assert.Nil(t, runSummary(nil))
}

func TestRunSummary_ReportsAPerfectSuccessRate(t *testing.T) {
	summary := runSummary([]Run{{Status: "SUCCESS"}, {Status: "SUCCESS"}})

	assert.Equal(t, 2, summary["run_count"])
	assert.Equal(t, float64(100), summary["success_rate"])
}

func TestRunSummary_CountsFailuresAgainstTheRate(t *testing.T) {
	summary := runSummary([]Run{{Status: "SUCCESS"}, {Status: "FAILURE"}, {Status: "CANCELED"}})

	assert.InDelta(t, 33.33, summary["success_rate"], 0.01)
}

func TestRunSummary_IgnoresUnfinishedRunsInTheRate(t *testing.T) {
	// A run still going has not succeeded or failed yet, so counting it would
	// drag the rate down for no reason.
	summary := runSummary([]Run{{Status: "STARTED"}, {Status: "SUCCESS"}})

	assert.Equal(t, float64(100), summary["success_rate"])
}

func TestRunSummary_OmitsTheRateWhenNothingHasFinished(t *testing.T) {
	summary := runSummary([]Run{{Status: "QUEUED"}})

	assert.NotContains(t, summary, "success_rate")
	assert.Equal(t, "QUEUED", summary["last_run_status"])
}

// Run history events.

func TestRunHistory_AQueuedRunOnlyEmitsAStart(t *testing.T) {
	history := runHistoryFor("mrn://pipeline/dagster/etl_job", "etl_job", []Run{
		{RunID: "r1", Status: "QUEUED", StartTime: floatPtr(1788853304)},
	})

	require.Len(t, history.Runs, 1)
	assert.Equal(t, "START", history.Runs[0].EventType)
}

func TestRunHistory_ARunningRunIsTimedFromItsUpdate(t *testing.T) {
	history := runHistoryFor("mrn://pipeline/dagster/etl_job", "etl_job", []Run{
		{RunID: "r1", Status: "STARTED", StartTime: floatPtr(1788853304), UpdateTime: floatPtr(1788853310)},
	})

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "RUNNING", history.Runs[1].EventType)
	assert.Equal(t, int64(1788853310), history.Runs[1].EventTime.Unix())
}

func TestRunHistory_ARunThatNeverStartedEmitsNoStart(t *testing.T) {
	history := runHistoryFor("mrn://pipeline/dagster/etl_job", "etl_job", []Run{
		{RunID: "r1", Status: "CANCELED", UpdateTime: floatPtr(1788853310)},
	})

	require.Len(t, history.Runs, 1)
	assert.Equal(t, "ABORT", history.Runs[0].EventType)
}
