package pinot

import (
	"net/http/httptest"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_FindsTables(t *testing.T) {
	result := discover(t, newFakeController().withBaseballStats().withDimBaseballTeams(), nil)

	stats := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, stats)
	assert.Equal(t, []string{"Pinot"}, stats.Providers)
	assert.Equal(t, "mrn://table/pinot/baseballstats", *stats.MRN)

	assert.NotNil(t, findAsset(result, "Table", "dimBaseballTeams"))
}

func TestDiscover_NamesATableByItsLogicalNameNotTheTypedOne(t *testing.T) {
	// The config calls it baseballStats_OFFLINE; the asset must not.
	result := discover(t, newFakeController().withBaseballStats(), nil)

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "baseballStats", *result.Assets[0].Name)
	assert.Equal(t, "baseballStats", result.Assets[0].Metadata["table_name"])
}

func TestDiscover_CarriesTableConfigMetadata(t *testing.T) {
	result := discover(t, newFakeController().withBaseballStats(), nil)

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)

	assert.Equal(t, []string{"OFFLINE"}, a.Metadata["table_types"])
	assert.Equal(t, "batch", a.Metadata["ingestion_type"])
	assert.Equal(t, "1", a.Metadata["replication"])
	assert.Equal(t, "DefaultTenant", a.Metadata["broker_tenant"])
	assert.Equal(t, "DefaultTenant", a.Metadata["server_tenant"])
	assert.Equal(t, "HEAP", a.Metadata["load_mode"])
	assert.Equal(t, false, a.Metadata["is_dim_table"])
	assert.Equal(t, []string{"playerID", "teamID"}, a.Metadata["inverted_index_columns"])
	assert.Equal(t, "1.6.0-SNAPSHOT-1f17ab0ede12e2703e4a037837d4c641c4756d11", a.Metadata["pinot_version"])
	// Settings the config does not have stay out of the metadata.
	assert.NotContains(t, a.Metadata, "time_column")
	assert.NotContains(t, a.Metadata, "retention")
	assert.NotContains(t, a.Metadata, "sorted_column")
	assert.NotContains(t, a.Metadata, "stream_type")
}

func TestDiscover_DeepLinksIntoTheControllerUI(t *testing.T) {
	f := newFakeController().withBaseballStats()
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"controller_url": server.URL + "/"})
	require.NoError(t, err)

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)
	want := server.URL + "/#/tenants/table/baseballStats_OFFLINE"
	assert.Equal(t, want, a.Metadata["url"])
	require.Len(t, a.ExternalLinks, 1)
	assert.Equal(t, want, a.ExternalLinks[0].URL)
}

func TestDiscover_ReadsRealtimeTableSettings(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)

	assert.Equal(t, []string{"REALTIME"}, a.Metadata["table_types"])
	assert.Equal(t, "stream", a.Metadata["ingestion_type"])
	assert.Equal(t, "event_ts", a.Metadata["time_column"])
	assert.Equal(t, "MILLISECONDS", a.Metadata["time_type"])
	assert.Equal(t, "30 DAYS", a.Metadata["retention"])
	assert.Equal(t, "1", a.Metadata["replication"], "realtime tables call it replicasPerPartition")
	assert.Equal(t, "customer_id", a.Metadata["sorted_column"])
	assert.Equal(t, []string{"order_id"}, a.Metadata["inverted_index_columns"])
	assert.Equal(t, "kafka", a.Metadata["stream_type"])
	assert.Equal(t, "orders-events", a.Metadata["stream_topic"])
	assert.Equal(t, "localhost:9092", a.Metadata["stream_brokers"])
}

func TestDiscover_MarksDimensionTablesAndTheirPrimaryKeys(t *testing.T) {
	result := discover(t, newFakeController().withDimBaseballTeams(), nil)

	a := findAsset(result, "Table", "dimBaseballTeams")
	require.NotNil(t, a)
	assert.Equal(t, true, a.Metadata["is_dim_table"])
	assert.Equal(t, []string{"teamID"}, a.Metadata["primary_key_columns"])

	_, byName := columnsOf(t, a)
	assert.Equal(t, true, byName["teamID"]["is_primary_key"])
	assert.NotContains(t, byName["teamName"], "is_primary_key")
}

func TestDiscover_FlattensDimensionsMetricsAndDateTimesIntoColumns(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)

	columns, byName := columnsOf(t, a)

	var order []string
	for _, c := range columns {
		order = append(order, c["column_name"].(string))
	}
	assert.Equal(t, []string{"order_id", "customer_id", "item_ids", "amount", "event_ts"}, order,
		"dimensions, then metrics, then date-times, each in Pinot's order")

	assert.Equal(t, "dimension", byName["order_id"]["field_type"])
	assert.Equal(t, "metric", byName["amount"]["field_type"])
	assert.Equal(t, "datetime", byName["event_ts"]["field_type"])
	assert.Equal(t, "STRING", byName["order_id"]["data_type"])
	assert.Equal(t, "DOUBLE", byName["amount"]["data_type"])
}

func TestDiscover_RendersMultiValueFieldsAsArrays(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)
	_, byName := columnsOf(t, a)

	assert.Equal(t, "INT[]", byName["item_ids"]["data_type"])
	assert.Equal(t, false, byName["item_ids"]["single_value"])
	assert.Equal(t, true, byName["order_id"]["single_value"], "singleValueField is omitted when true")
}

func TestDiscover_KeepsDateTimeFormatAndGranularity(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)
	_, byName := columnsOf(t, a)

	assert.Equal(t, "1:MILLISECONDS:EPOCH", byName["event_ts"]["format"])
	assert.Equal(t, "1:MILLISECONDS", byName["event_ts"]["granularity"])
	assert.NotContains(t, byName["order_id"], "format")
}

func TestDiscover_ColumnsAreNullableUnlessMarkedNotNull(t *testing.T) {
	result := discover(t, newFakeController().
		withTable("strict", dimBaseballTeamsConfigJSON).
		withSchema("strict", notNullSchemaJSON), nil)

	a := findAsset(result, "Table", "strict")
	require.NotNil(t, a)
	_, byName := columnsOf(t, a)

	assert.Equal(t, false, byName["id"]["is_nullable"])
	assert.Equal(t, true, byName["note"]["is_nullable"])
	assert.Equal(t, "n/a", byName["note"]["default_null_value"])
}

func TestDiscover_EmitsRowCountSizeAndColumnCountStatistics(t *testing.T) {
	result := discover(t, newFakeController().withBaseballStats(), nil)

	stats := statsOf(result, assetMRN("Table", "baseballStats"))
	assert.Equal(t, float64(97889), stats["asset.row_count"])
	assert.Equal(t, float64(3538585), stats["asset.size_bytes"])
	assert.Equal(t, float64(5), stats["asset.column_count"])
}

func TestDiscover_SkipsTheSizeWhileTheControllerReportsUnknown(t *testing.T) {
	// A consuming segment has no size yet, so the controller answers -1.
	// That is an unknown, not a table of zero bytes.
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	stats := statsOf(result, assetMRN("Table", "orders_events"))
	assert.NotContains(t, stats, "asset.size_bytes")
	assert.Equal(t, float64(3), stats["asset.row_count"])
}

func TestDiscover_CountsSegmentsPerType(t *testing.T) {
	result := discover(t, newFakeController().
		withTable("clicks", clicksConfigJSON).
		withSchema("clicks", clicksSchemaJSON).
		withSegments("clicks", clicksSegmentsJSON), nil)

	a := findAsset(result, "Table", "clicks")
	require.NotNil(t, a)
	assert.Equal(t, 2, a.Metadata["offline_segment_count"])
	assert.Equal(t, 1, a.Metadata["realtime_segment_count"])
	assert.Equal(t, 3, a.Metadata["segment_count"])
}

func TestDiscover_TreatsAHybridTableAsOneAsset(t *testing.T) {
	result := discover(t, newFakeController().
		withTable("clicks", clicksConfigJSON).
		withSchema("clicks", clicksSchemaJSON), nil)

	require.Len(t, result.Assets, 2, "one table and one topic")
	a := findAsset(result, "Table", "clicks")
	require.NotNil(t, a)

	assert.Equal(t, []string{"OFFLINE", "REALTIME"}, a.Metadata["table_types"])
	assert.Equal(t, "hybrid", a.Metadata["ingestion_type"])
	// Shared settings come from the offline side, the stream from the
	// realtime side.
	assert.Equal(t, "365 DAYS", a.Metadata["retention"])
	assert.Equal(t, "clicks", a.Metadata["stream_topic"])
	assert.Equal(t, "kafka-1:9092,kafka-2:9092", a.Metadata["stream_brokers"])
}

func TestDiscover_LinksARealtimeTableToItsKafkaTopic(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), nil)

	topic := findAsset(result, "Topic", "orders-events")
	require.NotNil(t, topic)
	assert.Equal(t, []string{"Kafka"}, topic.Providers, "the topic belongs to the Kafka plugin's identity")
	assert.Equal(t, "mrn://topic/kafka/orders-events", *topic.MRN)
	assert.Equal(t, "kafka", topic.Metadata["stream_type"])
	assert.Equal(t, "localhost:9092", topic.Metadata["stream_brokers"])
	require.Len(t, topic.Sources, 1)
	assert.Equal(t, "Pinot", topic.Sources[0].Name, "this plugin is the source that saw it")

	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-events", "mrn://table/pinot/orders_events", "FEEDS"))
}

func TestDiscover_ReadsStreamSettingsFromTheLegacyLocation(t *testing.T) {
	result := discover(t, newFakeController().withTable("legacy_events", legacyEventsConfigJSON), nil)

	a := findAsset(result, "Table", "legacy_events")
	require.NotNil(t, a)
	assert.Equal(t, "legacy-events", a.Metadata["stream_topic"])
	assert.NotNil(t, findAsset(result, "Topic", "legacy-events"))
	assert.True(t, hasEdge(result, "mrn://topic/kafka/legacy-events", "mrn://table/pinot/legacy_events", "FEEDS"))
}

func TestDiscover_LinksAKinesisFedTableToItsStream(t *testing.T) {
	result := discover(t, newFakeController().withTable("kinesis_orders", kinesisOrdersConfigJSON), nil)

	stream := findAsset(result, "Stream", "orders-stream")
	require.NotNil(t, stream)
	assert.Equal(t, []string{"Kinesis"}, stream.Providers)
	assert.Equal(t, "mrn://stream/kinesis/orders-stream", *stream.MRN)
	assert.NotContains(t, stream.Metadata, "stream_brokers", "Kinesis has no broker list")

	assert.True(t, hasEdge(result, "mrn://stream/kinesis/orders-stream", "mrn://table/pinot/kinesis_orders", "FEEDS"))
}

func TestDiscover_EmitsNoLineageForStreamTypesWithoutAMarmotPlugin(t *testing.T) {
	result := discover(t, newFakeController().withTable("pulsar_logs", pulsarLogsConfigJSON), nil)

	require.Len(t, result.Assets, 1)
	assert.Empty(t, result.Lineage)
	// The stream is still described on the table itself.
	a := findAsset(result, "Table", "pulsar_logs")
	require.NotNil(t, a)
	assert.Equal(t, "pulsar", a.Metadata["stream_type"])
	assert.Equal(t, "persistent://public/default/logs", a.Metadata["stream_topic"])
}

func TestDiscover_SharesOneTopicAssetBetweenTables(t *testing.T) {
	// Two tables consuming the same topic must not produce two topic
	// assets, or the server would see the same MRN twice in one batch.
	result := discover(t, newFakeController().
		withTable("orders_events", ordersEventsConfigJSON).
		withTable("orders_events_copy", ordersEventsConfigJSON), nil)

	var topics int
	for _, a := range result.Assets {
		if a.Type == "Topic" {
			topics++
		}
	}
	assert.Equal(t, 1, topics)
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-events", "mrn://table/pinot/orders_events", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://topic/kafka/orders-events", "mrn://table/pinot/orders_events_copy", "FEEDS"))
}

func TestDiscover_FallsBackToTheSchemaNamedInTheTableConfig(t *testing.T) {
	result := discover(t, newFakeController().
		withTable("renamed", renamedSchemaConfigJSON).
		withNamedSchema("renamed_schema", renamedSchemaJSON), nil)

	a := findAsset(result, "Table", "renamed")
	require.NotNil(t, a)
	assert.Equal(t, "renamed_schema", a.Metadata["schema_name"])
	_, byName := columnsOf(t, a)
	assert.Contains(t, byName, "id")
}

func TestDiscover_ExcludesSystemTablesByDefault(t *testing.T) {
	result := discover(t, newFakeController().
		withBaseballStats().
		withTable("_internal", dimBaseballTeamsConfigJSON), nil)

	assert.Nil(t, findAsset(result, "Table", "_internal"))
	assert.NotNil(t, findAsset(result, "Table", "baseballStats"))
}

func TestDiscover_IncludesSystemTablesWhenConfigured(t *testing.T) {
	result := discover(t, newFakeController().
		withTable("_internal", dimBaseballTeamsConfigJSON),
		pluginsdk.RawConfig{"exclude_system_tables": false})

	assert.NotNil(t, findAsset(result, "Table", "_internal"))
}

func TestDiscover_SkipsColumnsWhenDisabled(t *testing.T) {
	f := newFakeController().withBaseballStats()
	result := discover(t, f, pluginsdk.RawConfig{"include_columns": false})

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)
	assert.NotContains(t, a.Schema, "columns")
	assert.NotContains(t, statsOf(result, *a.MRN), "asset.column_count")
	assert.Empty(t, f.requestsTo("/tables/baseballStats/schema"))
}

func TestDiscover_SkipsRowCountsWhenDisabled(t *testing.T) {
	f := newFakeController().withBaseballStats()
	result := discover(t, f, pluginsdk.RawConfig{"include_row_counts": false})

	stats := statsOf(result, assetMRN("Table", "baseballStats"))
	assert.NotContains(t, stats, "asset.row_count")
	assert.Contains(t, stats, "asset.size_bytes")
	assert.Empty(t, f.requestsTo("/sql"), "no query should be sent")
}

func TestDiscover_SkipsSizesWhenDisabled(t *testing.T) {
	f := newFakeController().withBaseballStats()
	result := discover(t, f, pluginsdk.RawConfig{"include_sizes": false})

	assert.NotContains(t, statsOf(result, assetMRN("Table", "baseballStats")), "asset.size_bytes")
	assert.Empty(t, f.requestsTo("/tables/baseballStats/size"))
}

func TestDiscover_SkipsLineageWhenDisabled(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), pluginsdk.RawConfig{"discover_lineage": false})

	assert.Empty(t, result.Lineage)
	assert.Nil(t, findAsset(result, "Topic", "orders-events"))
	// The stream is still described in the table metadata.
	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)
	assert.Equal(t, "orders-events", a.Metadata["stream_topic"])
}

func TestDiscover_ContinuesWhenOneTableFails(t *testing.T) {
	// "broken" is listed but its config answers 404, the way a table that
	// is being deleted does.
	f := newFakeController().withBaseballStats()
	f.tables = append(f.tables, "broken")

	result := discover(t, f, nil)

	assert.NotNil(t, findAsset(result, "Table", "baseballStats"))
	assert.Nil(t, findAsset(result, "Table", "broken"))
}

func TestDiscover_KeepsATableWhoseRowCountFails(t *testing.T) {
	// The count is a query against the broker; a broker-side failure
	// arrives as a 200 with an exception in the body and must not drop
	// the table.
	f := newFakeController().withBaseballStats()
	delete(f.counts, "baseballStats")

	result := discover(t, f, nil)

	a := findAsset(result, "Table", "baseballStats")
	require.NotNil(t, a)
	stats := statsOf(result, *a.MRN)
	assert.NotContains(t, stats, "asset.row_count")
	assert.Contains(t, stats, "asset.size_bytes")
}

func TestDiscover_KeepsATableWithoutASchema(t *testing.T) {
	result := discover(t, newFakeController().withTable("schemaless", dimBaseballTeamsConfigJSON), nil)

	a := findAsset(result, "Table", "schemaless")
	require.NotNil(t, a)
	assert.NotContains(t, a.Schema, "columns")
}

func TestDiscover_FailsWhenTheControllerIsUnreachable(t *testing.T) {
	server := httptest.NewServer(nil)
	url := server.URL
	server.Close()

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"controller_url": url})
	require.Error(t, err)
}

func TestDiscover_FailsWhenTheControllerIsNotHealthy(t *testing.T) {
	f := newFakeController().withBaseballStats().withHealth("Pinot controller status is NOT_STARTED")
	server := f.start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"controller_url": server.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_STARTED")
}

func TestDiscover_SendsTheBearerToken(t *testing.T) {
	f := newFakeController().withBaseballStats()
	discover(t, f, pluginsdk.RawConfig{"token": "secret-token"})

	requests := f.requestsTo("/tables")
	require.NotEmpty(t, requests)
	assert.Equal(t, "Bearer secret-token", requests[0].Header.Get("Authorization"))
}

func TestDiscover_SendsBasicAuth(t *testing.T) {
	f := newFakeController().withBaseballStats()
	discover(t, f, pluginsdk.RawConfig{"username": "admin", "password": "verysecret"})

	requests := f.requestsTo("/tables")
	require.NotEmpty(t, requests)
	assert.Equal(t, "Basic YWRtaW46dmVyeXNlY3JldA==", requests[0].Header.Get("Authorization"))
}

func TestDiscover_SendsTheDatabaseHeader(t *testing.T) {
	f := newFakeController().withBaseballStats()
	discover(t, f, pluginsdk.RawConfig{"database": "analytics"})

	for _, path := range []string{"/tables", "/tables/baseballStats", "/sql"} {
		requests := f.requestsTo(path)
		require.NotEmpty(t, requests, path)
		assert.Equal(t, "analytics", requests[0].Header.Get("database"), path)
	}
}

func TestDiscover_SendsNoDatabaseHeaderByDefault(t *testing.T) {
	f := newFakeController().withBaseballStats()
	discover(t, f, nil)

	requests := f.requestsTo("/tables")
	require.NotEmpty(t, requests)
	assert.Empty(t, requests[0].Header.Get("database"))
}

func TestDiscover_QueriesTheControllerWhenNoBrokerIsConfigured(t *testing.T) {
	f := newFakeController().withBaseballStats()
	discover(t, f, nil)

	assert.NotEmpty(t, f.requestsTo("/sql"))
	assert.Empty(t, f.requestsTo("/query/sql"))
}

func TestDiscover_QueriesTheBrokerWhenConfigured(t *testing.T) {
	f := newFakeController().withBaseballStats()
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{
		"controller_url": server.URL,
		"broker_url":     server.URL,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, f.requestsTo("/query/sql"))
	assert.Empty(t, f.requestsTo("/sql"))
	assert.Equal(t, float64(97889), statsOf(result, assetMRN("Table", "baseballStats"))["asset.row_count"])
}

func TestDiscover_QuotesTheTableNameInQueries(t *testing.T) {
	f := newFakeController().withBaseballStats()
	server := f.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"controller_url": server.URL})
	require.NoError(t, err)

	// The fake only recognises a quoted name, so a row count proves the
	// query was SELECT COUNT(*) FROM "baseballStats".
	assert.Equal(t, float64(97889), statsOf(result, assetMRN("Table", "baseballStats"))["asset.row_count"])
}

func TestDiscover_AppliesConfiguredTags(t *testing.T) {
	result := discover(t, newFakeController().withOrdersEvents(), pluginsdk.RawConfig{
		"tags": []interface{}{"pinot", "${stream_type}"},
	})

	a := findAsset(result, "Table", "orders_events")
	require.NotNil(t, a)
	assert.Contains(t, a.Tags, "pinot")
	assert.Contains(t, a.Tags, "kafka")
}

func TestFetchSampleData_ReturnsColumnsAndRows(t *testing.T) {
	f := newFakeController().withDimBaseballTeams()
	server := f.start(t)

	name := "dimBaseballTeams"
	columns, rows, err := (&Source{}).FetchSampleData(t.Context(),
		pluginsdk.RawConfig{"controller_url": server.URL},
		&pluginsdk.Asset{Name: &name, Metadata: map[string]interface{}{"table_name": name}})
	require.NoError(t, err)

	assert.Equal(t, []string{"teamID", "teamName"}, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, []interface{}{"ANA", "Anaheim Angels"}, rows[0])
}

func TestFetchSampleData_FallsBackToTheAssetName(t *testing.T) {
	f := newFakeController().withDimBaseballTeams()
	server := f.start(t)

	name := "dimBaseballTeams"
	columns, _, err := (&Source{}).FetchSampleData(t.Context(),
		pluginsdk.RawConfig{"controller_url": server.URL},
		&pluginsdk.Asset{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, []string{"teamID", "teamName"}, columns)
}

func TestFetchSampleData_FailsForAnUnknownTable(t *testing.T) {
	f := newFakeController().withDimBaseballTeams()
	server := f.start(t)

	name := "missing"
	_, _, err := (&Source{}).FetchSampleData(t.Context(),
		pluginsdk.RawConfig{"controller_url": server.URL},
		&pluginsdk.Asset{Name: &name})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TableDoesNotExistError")
}

func TestFetchSampleData_FailsWithoutAnAsset(t *testing.T) {
	_, _, err := (&Source{}).FetchSampleData(t.Context(), pluginsdk.RawConfig{"controller_url": "http://localhost:9000"}, nil)
	require.Error(t, err)
}
