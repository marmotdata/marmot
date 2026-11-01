package timescale

import (
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

func ptrInt64(v int64) *int64 { return &v }
func ptrInt(v int) *int       { return &v }
func ptrBool(v bool) *bool    { return &v }

// Internal schema exclusion. TimescaleDB puts a chunk table in
// _timescaledb_internal for every chunk, so a hypertable with a thousand
// chunks would otherwise flood the catalog.

func TestIsInternalSchema_ExcludesTimescaleInternals(t *testing.T) {
	assert.True(t, isInternalSchema("_timescaledb_internal"))
	assert.True(t, isInternalSchema("_timescaledb_catalog"))
	assert.True(t, isInternalSchema("_timescaledb_config"))
	assert.True(t, isInternalSchema("_timescaledb_cache"))
}

func TestIsInternalSchema_ExcludesTheTimescaleCatalogViews(t *testing.T) {
	assert.True(t, isInternalSchema("timescaledb_information"))
	assert.True(t, isInternalSchema("timescaledb_experimental"))
}

func TestIsInternalSchema_ExcludesPostgreSQLInternals(t *testing.T) {
	assert.True(t, isInternalSchema("pg_catalog"))
	assert.True(t, isInternalSchema("pg_toast"))
	assert.True(t, isInternalSchema("information_schema"))
}

func TestIsInternalSchema_KeepsUserSchemas(t *testing.T) {
	assert.False(t, isInternalSchema("public"))
	assert.False(t, isInternalSchema("analytics"))
	// A user schema that merely starts with the same word is not internal.
	assert.False(t, isInternalSchema("timescaledb_reports"))
}

func TestSchemaFilterSQL_NamesEveryInternalSchema(t *testing.T) {
	filter := schemaFilterSQL("n.nspname")

	assert.Contains(t, filter, "n.nspname !~ '^pg_'")
	assert.Contains(t, filter, "'_timescaledb_internal'")
	assert.Contains(t, filter, "'timescaledb_information'")
}

// Interval rendering.

func TestFormatInterval_RendersOneDay(t *testing.T) {
	assert.Equal(t, "1 day", formatInterval(86400))
}

func TestFormatInterval_RendersSeveralDays(t *testing.T) {
	assert.Equal(t, "7 days", formatInterval(7*86400))
}

func TestFormatInterval_RendersHoursRatherThanAClockTime(t *testing.T) {
	// PostgreSQL prints a twelve hour interval as 12:00:00.
	assert.Equal(t, "12 hours", formatInterval(12*3600))
}

func TestFormatInterval_RendersOneHour(t *testing.T) {
	assert.Equal(t, "1 hour", formatInterval(3600))
}

func TestFormatInterval_RendersMinutes(t *testing.T) {
	assert.Equal(t, "30 minutes", formatInterval(1800))
}

func TestFormatInterval_CombinesUnits(t *testing.T) {
	assert.Equal(t, "1 day 6 hours 30 minutes", formatInterval(86400+6*3600+1800))
}

func TestFormatInterval_RendersSeconds(t *testing.T) {
	assert.Equal(t, "45 seconds", formatInterval(45))
}

func TestFormatInterval_RendersNothingForZero(t *testing.T) {
	assert.Equal(t, "", formatInterval(0))
}

// Compression ordering. PostgreSQL sorts nulls last when ascending and first
// when descending, so only a departure from that is worth spelling out.

func TestFormatOrderBy_DescendingIsTheDefaultNullOrder(t *testing.T) {
	assert.Equal(t, "time DESC", formatOrderBy("time", false, true))
}

func TestFormatOrderBy_AscendingIsTheDefaultNullOrder(t *testing.T) {
	assert.Equal(t, "time ASC", formatOrderBy("time", true, false))
}

func TestFormatOrderBy_SpellsOutNullsLastOnDescending(t *testing.T) {
	assert.Equal(t, "time DESC NULLS LAST", formatOrderBy("time", false, false))
}

func TestFormatOrderBy_SpellsOutNullsFirstOnAscending(t *testing.T) {
	assert.Equal(t, "time ASC NULLS FIRST", formatOrderBy("time", true, true))
}

// Space partitions. Dimension 1 is always time; anything above it partitions
// the data by value. OpenMetadata's connector reads only dimension 1.

func TestSpacePartitions_SkipsTheTimeDimension(t *testing.T) {
	partitions := spacePartitions([]dimension{
		{Number: 1, ColumnName: "time", Kind: "Time"},
	})

	assert.Empty(t, partitions)
}

func TestSpacePartitions_RecordsTheColumnAndCount(t *testing.T) {
	partitions := spacePartitions([]dimension{
		{Number: 1, ColumnName: "time", Kind: "Time"},
		{Number: 2, ColumnName: "device_id", Kind: "Space", NumPartitions: ptrInt(4)},
	})

	require.Len(t, partitions, 1)
	assert.Equal(t, "device_id", partitions[0]["column"])
	assert.Equal(t, 4, partitions[0]["partitions"])
}

func TestSpacePartitions_RecordsEverySpaceDimension(t *testing.T) {
	partitions := spacePartitions([]dimension{
		{Number: 1, ColumnName: "time", Kind: "Time"},
		{Number: 2, ColumnName: "device_id", Kind: "Space", NumPartitions: ptrInt(4)},
		{Number: 3, ColumnName: "region", Kind: "Space", NumPartitions: ptrInt(2)},
	})

	require.Len(t, partitions, 2)
	assert.Equal(t, "device_id", partitions[0]["column"])
	assert.Equal(t, "region", partitions[1]["column"])
}

// Optional catalog columns. TimescaleDB dropped is_distributed when it
// removed multi-node, and finalized when every aggregate became finalized.

func TestOptionalColumn_SelectsThePresentColumn(t *testing.T) {
	columns := map[string]map[string]bool{"hypertables": {"is_distributed": true}}

	assert.Equal(t, "is_distributed", optionalColumn(columns, "hypertables", "is_distributed", "boolean"))
}

func TestOptionalColumn_SubstitutesATypedNullWhenAbsent(t *testing.T) {
	columns := map[string]map[string]bool{"hypertables": {"num_chunks": true}}

	assert.Equal(t, "NULL::boolean", optionalColumn(columns, "hypertables", "is_distributed", "boolean"))
}

func TestOptionalColumn_SubstitutesATypedNullForAnUnknownView(t *testing.T) {
	assert.Equal(t, "NULL::boolean", optionalColumn(nil, "continuous_aggregates", "finalized", "boolean"))
}

// Hypertable metadata assembly.

func sampleHypertable() *hypertable {
	return &hypertable{
		Schema:             "public",
		Name:               "conditions",
		Owner:              "postgres",
		NumDimensions:      2,
		NumChunks:          12,
		CompressionEnabled: true,
		Dimensions: []dimension{
			{Number: 1, ColumnName: "time", ColumnType: "timestamp with time zone", Kind: "Time", IntervalSeconds: ptrInt64(86400)},
			{Number: 2, ColumnName: "device_id", ColumnType: "integer", Kind: "Space", NumPartitions: ptrInt(4)},
		},
		SegmentBy: []string{"device_id"},
		OrderBy:   []string{"time DESC"},
		ChunkRange: &chunkRange{
			Oldest: "2026-09-05 00:00:00+00",
			Newest: "2026-09-09 00:00:00+00",
		},
		Policies: []map[string]interface{}{
			{"name": "Columnstore Policy [1000]", "proc": "policy_compression", "schedule": "12 hours"},
		},
	}
}

func TestApplyHypertableMetadata_MarksTheTableAsAHypertable(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{Version: "2.29.2"}, false)

	assert.Equal(t, true, metadata["hypertable"])
	assert.Equal(t, "2.29.2", metadata["timescaledb_version"])
}

func TestApplyHypertableMetadata_RecordsTheTimeDimension(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	assert.Equal(t, "time", metadata["time_column"])
	assert.Equal(t, "timestamp with time zone", metadata["time_column_type"])
	assert.Equal(t, "1 day", metadata["time_interval"])
	assert.Equal(t, int64(86400), metadata["time_interval_seconds"])
}

func TestApplyHypertableMetadata_RecordsSpacePartitions(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	partitions, ok := metadata["space_partitions"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, partitions, 1)
	assert.Equal(t, "device_id", partitions[0]["column"])
}

func TestApplyHypertableMetadata_RecordsTheChunkCount(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	assert.Equal(t, int64(12), metadata["num_chunks"])
}

func TestApplyHypertableMetadata_RecordsCompressionSettings(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	assert.Equal(t, true, metadata["compression_enabled"])
	assert.Equal(t, []string{"device_id"}, metadata["compression_segment_by"])
	assert.Equal(t, []string{"time DESC"}, metadata["compression_order_by"])
}

func TestApplyHypertableMetadata_RecordsPolicies(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	policies, ok := metadata["policies"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, policies, 1)
	assert.Equal(t, "policy_compression", policies[0]["proc"])
}

func TestApplyHypertableMetadata_LeavesOutTheChunkRangeByDefault(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	_, present := metadata["chunk_range"]
	assert.False(t, present)
}

func TestApplyHypertableMetadata_RecordsTheChunkRangeWhenAsked(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, true)

	chunkRange, ok := metadata["chunk_range"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2026-09-05 00:00:00+00", chunkRange["oldest"])
	assert.Equal(t, "2026-09-09 00:00:00+00", chunkRange["newest"])
}

func TestApplyHypertableMetadata_LeavesOutIsDistributedWhenUnreported(t *testing.T) {
	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, sampleHypertable(), &timescaleInfo{}, false)

	_, present := metadata["is_distributed"]
	assert.False(t, present)
}

func TestApplyHypertableMetadata_RecordsIsDistributedWhenReported(t *testing.T) {
	ht := sampleHypertable()
	ht.IsDistributed = ptrBool(false)

	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, ht, &timescaleInfo{}, false)

	assert.Equal(t, false, metadata["is_distributed"])
}

func TestApplyHypertableMetadata_RecordsAnIntegerInterval(t *testing.T) {
	ht := &hypertable{
		Dimensions: []dimension{
			{Number: 1, ColumnName: "epoch", ColumnType: "bigint", Kind: "Time",
				IntegerInterval: ptrInt64(1000000), IntegerNowFunc: "now_epoch"},
		},
	}

	metadata := map[string]interface{}{}
	applyHypertableMetadata(metadata, ht, &timescaleInfo{}, false)

	assert.Equal(t, "epoch", metadata["time_column"])
	assert.Equal(t, int64(1000000), metadata["integer_interval"])
	assert.Equal(t, "now_epoch", metadata["integer_now_func"])
	_, present := metadata["time_interval"]
	assert.False(t, present)
}

// Continuous aggregate metadata assembly.

func sampleAggregate() *aggregate {
	return &aggregate{
		ViewSchema:            "public",
		ViewName:              "conditions_daily",
		MaterializationSchema: "_timescaledb_internal",
		MaterializationName:   "_materialized_hypertable_2",
		Definition:            "SELECT time_bucket('1 day'::interval, \"time\") AS bucket FROM conditions",
		MaterializedOnly:      true,
		HypertableSchema:      "public",
		HypertableName:        "conditions",
		Policies: []map[string]interface{}{
			{"name": "Refresh Continuous Aggregate Policy [1002]", "proc": "policy_refresh_continuous_aggregate", "schedule": "1 hour"},
		},
	}
}

func TestApplyAggregateMetadata_MarksTheViewAsAnAggregate(t *testing.T) {
	metadata := map[string]interface{}{}
	applyAggregateMetadata(metadata, sampleAggregate(), &timescaleInfo{Version: "2.29.2"})

	assert.Equal(t, true, metadata["continuous_aggregate"])
	assert.Equal(t, "2.29.2", metadata["timescaledb_version"])
	assert.Equal(t, true, metadata["materialized_only"])
}

func TestApplyAggregateMetadata_NamesTheSourceHypertable(t *testing.T) {
	metadata := map[string]interface{}{}
	applyAggregateMetadata(metadata, sampleAggregate(), &timescaleInfo{})

	assert.Equal(t, "public.conditions", metadata["source_hypertable"])
	assert.Equal(t, "_timescaledb_internal._materialized_hypertable_2", metadata["materialization_hypertable"])
}

func TestApplyAggregateMetadata_PicksOutTheRefreshPolicy(t *testing.T) {
	metadata := map[string]interface{}{}
	applyAggregateMetadata(metadata, sampleAggregate(), &timescaleInfo{})

	policy, ok := metadata["refresh_policy"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1 hour", policy["schedule"])
}

func TestApplyAggregateMetadata_LeavesOutFinalizedWhenUnreported(t *testing.T) {
	metadata := map[string]interface{}{}
	applyAggregateMetadata(metadata, sampleAggregate(), &timescaleInfo{})

	_, present := metadata["finalized"]
	assert.False(t, present)
}

func TestApplyAggregateMetadata_RecordsFinalizedWhenReported(t *testing.T) {
	agg := sampleAggregate()
	agg.Finalized = ptrBool(true)

	metadata := map[string]interface{}{}
	applyAggregateMetadata(metadata, agg, &timescaleInfo{})

	assert.Equal(t, true, metadata["finalized"])
}

// Reading table references off a view definition.

func TestReferencedObjects_FindsATableAfterFrom(t *testing.T) {
	known := map[string]bool{"conditions": true}

	assert.Equal(t, []string{"conditions"},
		referencedObjects("SELECT avg(temperature) FROM conditions GROUP BY 1", known))
}

func TestReferencedObjects_FindsATableAfterJoin(t *testing.T) {
	known := map[string]bool{"customers": true, "conditions": true}

	assert.Equal(t, []string{"conditions", "customers"},
		referencedObjects("SELECT c.name FROM customers c JOIN conditions co ON co.customer_id = c.id", known))
}

func TestReferencedObjects_StripsTheSchemaQualifier(t *testing.T) {
	known := map[string]bool{"conditions": true}

	assert.Equal(t, []string{"conditions"}, referencedObjects("SELECT * FROM public.conditions", known))
}

func TestReferencedObjects_HandlesQuotedIdentifiers(t *testing.T) {
	known := map[string]bool{"daily readings": true}

	assert.Equal(t, []string{"daily readings"}, referencedObjects(`SELECT * FROM "daily readings"`, known))
}

// An edge to an asset that was never created is dropped by the server, so
// only names this run actually discovered are worth emitting.
func TestReferencedObjects_IgnoresNamesThatWereNotDiscovered(t *testing.T) {
	known := map[string]bool{"conditions": true}

	assert.Equal(t, []string{"conditions"},
		referencedObjects("SELECT * FROM conditions JOIN somewhere_else USING (id)", known))
}

func TestReferencedObjects_IgnoresAKeywordFollowingFrom(t *testing.T) {
	known := map[string]bool{"conditions": true}

	assert.Equal(t, []string{"conditions"}, referencedObjects("SELECT * FROM ONLY conditions", known))
}

func TestReferencedObjects_ReturnsEachTableOnce(t *testing.T) {
	known := map[string]bool{"conditions": true}

	assert.Equal(t, []string{"conditions"},
		referencedObjects("SELECT * FROM conditions UNION ALL SELECT * FROM conditions", known))
}

func TestReferencedObjects_ReturnsNothingForATableDefinition(t *testing.T) {
	assert.Empty(t, referencedObjects("", map[string]bool{"conditions": true}))
}

// View lineage. The edge runs from the table a view reads to the view.

func TestViewLineage_LinksAPlainViewToItsTables(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "customers", Kind: kindTable},
		{Schema: "public", Name: "conditions", Kind: kindTable},
		{Schema: "public", Name: "customer_readings", Kind: kindView,
			Definition: "SELECT c.name FROM customers c JOIN conditions co ON co.customer_id = c.id"},
	}

	edges := viewLineage(objects, nil)

	require.Len(t, edges, 2)
	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/conditions",
		Target: "mrn://view/postgresql/customer_readings",
		Type:   "VIEW_OF",
	})
	assert.Contains(t, edges, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/customers",
		Target: "mrn://view/postgresql/customer_readings",
		Type:   "VIEW_OF",
	})
}

// PostgreSQL rewrites a continuous aggregate to read from the hidden
// materialization hypertable, so its own view definition names an object this
// plugin deliberately never creates. The TimescaleDB catalog names the real
// source, which is why the edge is taken from there.
func TestViewLineage_LinksAnAggregateToItsSourceHypertable(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "conditions", Kind: kindTable},
		{Schema: "public", Name: "conditions_daily", Kind: kindView,
			Definition: "SELECT bucket, device_id, avg_temp FROM _timescaledb_internal._materialized_hypertable_2"},
	}
	ts := &timescaleInfo{aggregates: map[string]*aggregate{
		"public.conditions_daily": sampleAggregate(),
	}}

	edges := viewLineage(objects, ts)

	require.Len(t, edges, 1)
	assert.Equal(t, pluginsdk.LineageEdge{
		Source: "mrn://table/postgresql/conditions",
		Target: "mrn://view/postgresql/conditions_daily",
		Type:   "VIEW_OF",
	}, edges[0])
}

func TestViewLineage_NeverPointsAtTheMaterializationHypertable(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "conditions", Kind: kindTable},
		{Schema: "public", Name: "conditions_daily", Kind: kindView,
			Definition: "SELECT bucket FROM _timescaledb_internal._materialized_hypertable_2"},
	}
	ts := &timescaleInfo{aggregates: map[string]*aggregate{
		"public.conditions_daily": sampleAggregate(),
	}}

	for _, edge := range viewLineage(objects, ts) {
		assert.NotContains(t, edge.Source, "materialized_hypertable")
	}
}

func TestViewLineage_SkipsTables(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "customers", Kind: kindTable},
		{Schema: "public", Name: "conditions", Kind: kindTable},
	}

	assert.Empty(t, viewLineage(objects, nil))
}

func TestViewLineage_SkipsAViewOverAnUndiscoveredTable(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "report", Kind: kindView, Definition: "SELECT * FROM archived_orders"},
	}

	assert.Empty(t, viewLineage(objects, nil))
}

func TestViewLineage_EmitsEachEdgeOnce(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "conditions", Kind: kindTable},
		{Schema: "public", Name: "conditions_daily", Kind: kindView, Definition: "SELECT * FROM conditions"},
	}
	ts := &timescaleInfo{aggregates: map[string]*aggregate{
		"public.conditions_daily": {
			ViewSchema: "public", ViewName: "conditions_daily",
			HypertableSchema: "public", HypertableName: "conditions",
			Definition: "SELECT * FROM conditions",
		},
	}}

	// The catalog names conditions as the source and the definition mentions
	// it too, which must not produce the edge twice.
	assert.Len(t, viewLineage(objects, ts), 1)
}

func TestViewLineage_LinksAViewToAnotherView(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "conditions_daily", Kind: kindView, Definition: "SELECT * FROM conditions"},
		{Schema: "public", Name: "conditions_weekly", Kind: kindView, Definition: "SELECT * FROM conditions_daily"},
	}

	edges := viewLineage(objects, nil)

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://view/postgresql/conditions_daily", edges[0].Source)
	assert.Equal(t, "mrn://view/postgresql/conditions_weekly", edges[0].Target)
}

func TestViewLineage_SkipsASelfReference(t *testing.T) {
	objects := []object{
		{Schema: "public", Name: "loop", Kind: kindView, Definition: "SELECT * FROM loop"},
	}

	assert.Empty(t, viewLineage(objects, nil))
}

// Statistics assembly. A hypertable's parent table holds no rows of its own.

func TestCollectStatistics_UsesTheTimescaleRowCountForAHypertable(t *testing.T) {
	objects := []object{{Schema: "public", Name: "conditions", Kind: kindTable, RowEstimate: 0, SizeBytes: 8192}}
	ts := &timescaleInfo{
		hypertables: map[string]*hypertable{"public.conditions": sampleHypertable()},
		sizes:       map[string]objectSize{"public.conditions": {Rows: ptrInt64(145), Bytes: ptrInt64(303104)}},
	}

	statistics := statisticsByName(buildStatistics(objects, ts, nil))

	assert.Equal(t, float64(145), statistics["asset.row_count"])
	assert.Equal(t, float64(303104), statistics["asset.size_bytes"])
}

func TestCollectStatistics_EmitsAChunkCountForAHypertable(t *testing.T) {
	objects := []object{{Schema: "public", Name: "conditions", Kind: kindTable}}
	ts := &timescaleInfo{
		hypertables: map[string]*hypertable{"public.conditions": sampleHypertable()},
		sizes:       map[string]objectSize{},
	}

	statistics := statisticsByName(buildStatistics(objects, ts, nil))

	assert.Equal(t, float64(12), statistics["asset.chunk_count"])
}

func TestCollectStatistics_KeepsThePostgreSQLNumbersForAPlainTable(t *testing.T) {
	objects := []object{{Schema: "public", Name: "customers", Kind: kindTable, RowEstimate: 3, SizeBytes: 16384}}
	ts := &timescaleInfo{hypertables: map[string]*hypertable{}, sizes: map[string]objectSize{}}

	statistics := statisticsByName(buildStatistics(objects, ts, nil))

	assert.Equal(t, float64(3), statistics["asset.row_count"])
	assert.Equal(t, float64(16384), statistics["asset.size_bytes"])
	_, present := statistics["asset.chunk_count"]
	assert.False(t, present)
}

func TestCollectStatistics_EmitsAColumnCount(t *testing.T) {
	objects := []object{{Schema: "public", Name: "customers", Kind: kindTable}}

	statistics := statisticsByName(buildStatistics(objects, nil, map[string]int64{"public.customers": 4}))

	assert.Equal(t, float64(4), statistics["asset.column_count"])
}

func TestCollectStatistics_WorksWithoutTheTimescaleCatalog(t *testing.T) {
	objects := []object{{Schema: "public", Name: "customers", Kind: kindTable, RowEstimate: 3}}

	statistics := statisticsByName(buildStatistics(objects, nil, nil))

	assert.Equal(t, float64(3), statistics["asset.row_count"])
}

func statisticsByName(statistics []pluginsdk.Statistic) map[string]float64 {
	byName := make(map[string]float64, len(statistics))
	for _, statistic := range statistics {
		byName[statistic.MetricName] = statistic.Value
	}
	return byName
}
