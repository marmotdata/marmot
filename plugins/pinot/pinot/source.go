// Package pinot discovers tables, their schemas and the streams that feed
// them from Apache Pinot clusters.
package pinot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Pinot"

// Config for the Pinot plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ControllerURL string `json:"controller_url" label:"Controller URL" description:"Pinot controller URL, for example http://localhost:9000" validate:"required,http_url"`
	BrokerURL     string `json:"broker_url" label:"Broker URL" description:"Pinot broker URL for queries. When empty, queries go through the controller" validate:"omitempty,http_url"`
	Username      string `json:"username" description:"Username for basic authentication"`
	Password      string `json:"password" description:"Password for basic authentication" sensitive:"true"`
	Token         string `json:"token" description:"Bearer token, used instead of a username and password" sensitive:"true"`
	Database      string `json:"database" description:"Logical database to discover (Pinot 1.1 and later). Empty means the default database"`
	VerifySSL     bool   `json:"verify_ssl" label:"Verify SSL" description:"Whether to verify the TLS certificate of the controller and broker" default:"true"`

	IncludeColumns      bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeRowCounts    bool `json:"include_row_counts" description:"Whether to count rows with a COUNT(*) query per table" default:"true"`
	IncludeSizes        bool `json:"include_sizes" description:"Whether to include the segment size reported by the controller" default:"true"`
	DiscoverLineage     bool `json:"discover_lineage" description:"Whether to link realtime tables to the Kafka topic or Kinesis stream that feeds them" default:"true"`
	ExcludeSystemTables bool `json:"exclude_system_tables" description:"Whether to skip tables whose name starts with an underscore" default:"true"`
}

// Example configuration for the plugin
var _ = `
controller_url: "http://pinot-controller.internal:9000"
broker_url: "http://pinot-broker.internal:8099"
username: "marmot"
password: "${PINOT_PASSWORD}"
include_columns: true
include_row_counts: true
include_sizes: true
discover_lineage: true
tags:
  - "pinot"
  - "analytics"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "pinot",
		Name:        "Pinot",
		Description: "Discover tables, schemas and stream lineage from Apache Pinot clusters",
		Icon:        "pinot",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Pinot plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	config.ControllerURL = strings.TrimSuffix(config.ControllerURL, "/")
	config.BrokerURL = strings.TrimSuffix(config.BrokerURL, "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	if config.Token != "" && config.Username != "" {
		return nil, fmt.Errorf("token and username are mutually exclusive: use one authentication method")
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) newClient() *Client {
	return NewClient(ClientConfig{
		ControllerURL: s.config.ControllerURL,
		BrokerURL:     s.config.BrokerURL,
		Username:      s.config.Username,
		Password:      s.config.Password,
		Token:         s.config.Token,
		Database:      s.config.Database,
		VerifySSL:     s.config.VerifySSL,
	})
}

// Discover discovers Pinot tables, their columns, sizes, row counts and
// the streams feeding realtime tables.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client := s.newClient()

	if err := client.Health(ctx); err != nil {
		return nil, fmt.Errorf("checking controller health: %w", err)
	}

	version, err := client.Version(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Pinot version")
	}

	tables, err := client.ListTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	log.Debug().Int("count", len(tables)).Msg("Listed Pinot tables")

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	// Two tables may consume the same topic; the topic is one asset.
	streamAssets := make(map[string]pluginsdk.Asset)

	for _, table := range tables {
		if s.config.ExcludeSystemTables && strings.HasPrefix(table, "_") {
			log.Debug().Str("table", table).Msg("Skipping system table")
			continue
		}

		discovered, err := s.discoverTable(ctx, client, table, version)
		if err != nil {
			log.Warn().Err(err).Str("table", table).Msg("Failed to discover table")
			continue
		}

		assets = append(assets, discovered.asset)
		statistics = append(statistics, discovered.statistics...)

		if discovered.stream != nil {
			streamAssets[*discovered.stream.MRN] = *discovered.stream
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: *discovered.stream.MRN,
				Target: *discovered.asset.MRN,
				Type:   "FEEDS",
			})
		}
	}

	// Sorted so two runs over the same cluster produce the same order.
	streamMRNs := make([]string, 0, len(streamAssets))
	for m := range streamAssets {
		streamMRNs = append(streamMRNs, m)
	}
	sort.Strings(streamMRNs)
	for _, m := range streamMRNs {
		assets = append(assets, streamAssets[m])
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Pinot discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// discoveredTable is everything one logical table contributes to the run.
type discoveredTable struct {
	asset      pluginsdk.Asset
	statistics []pluginsdk.Statistic
	// stream is the Kafka topic or Kinesis stream feeding a realtime
	// table, nil for batch tables.
	stream *pluginsdk.Asset
}

// column is the per-column shape serialised into an asset's Schema. Pinot
// groups fields into dimensions, metrics and date-times; the role is kept
// alongside the standard column fields.
type column struct {
	pluginsdk.Column
	FieldType        string `json:"field_type"`
	SingleValue      bool   `json:"single_value"`
	Format           string `json:"format,omitempty"`
	Granularity      string `json:"granularity,omitempty"`
	DefaultNullValue any    `json:"default_null_value,omitempty"`
}

func (s *Source) discoverTable(ctx context.Context, client *Client, table, version string) (*discoveredTable, error) {
	configs, err := client.GetTableConfigs(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("getting table config: %w", err)
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("controller returned no config for table")
	}

	// A hybrid table has an OFFLINE and a REALTIME config. The OFFLINE one
	// describes the settled data, so it is the one the shared fields come
	// from; the REALTIME one contributes the stream settings.
	tableTypes := make([]string, 0, len(configs))
	for tableType := range configs {
		tableTypes = append(tableTypes, tableType)
	}
	sort.Strings(tableTypes)
	primary := configs[tableTypes[0]]

	metadata := map[string]interface{}{
		"table_name":  table,
		"table_types": tableTypes,
		"url":         s.tableURL(primary.TableName),
	}
	if version != "" {
		metadata["pinot_version"] = version
	}
	addTableConfigMetadata(metadata, primary)

	ingestionType := "batch"
	if realtime, ok := configs["REALTIME"]; ok {
		ingestionType = "stream"
		if _, hybrid := configs["OFFLINE"]; hybrid {
			ingestionType = "hybrid"
		}
		addStreamMetadata(metadata, realtime.streamConfig())
	}
	metadata["ingestion_type"] = ingestionType

	segments, err := client.ListSegments(ctx, table)
	if err != nil {
		log.Warn().Err(err).Str("table", table).Msg("Failed to list segments")
	} else {
		total := 0
		for tableType, names := range segments {
			metadata[strings.ToLower(tableType)+"_segment_count"] = len(names)
			total += len(names)
		}
		metadata["segment_count"] = total
	}

	mrnValue := assetMRN("Table", table)
	name := table

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Table",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Pinot",
			URL:  s.tableURL(primary.TableName),
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	var statistics []pluginsdk.Statistic

	if s.config.IncludeColumns {
		schema, err := s.tableSchema(ctx, client, table, primary.SegmentsConfig.SchemaName)
		if err != nil {
			log.Warn().Err(err).Str("table", table).Msg("Failed to get table schema")
		} else {
			columns := buildColumns(schema)
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("table", table).Msg("Failed to serialise columns")
			} else {
				if len(schema.PrimaryKeyColumns) > 0 {
					metadata["primary_key_columns"] = schema.PrimaryKeyColumns
				}
				statistics = append(statistics, pluginsdk.Statistic{
					AssetMRN:   mrnValue,
					MetricName: "asset.column_count",
					Value:      float64(len(columns)),
				})
			}
		}
	}

	if s.config.IncludeSizes {
		size, err := client.GetTableSize(ctx, table)
		if err != nil {
			log.Warn().Err(err).Str("table", table).Msg("Failed to get table size")
		} else if size.ReportedSizeInBytes >= 0 {
			// The controller reports -1 while a segment is still consuming
			// or a server has not answered; that is not a size of zero.
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "asset.size_bytes",
				Value:      float64(size.ReportedSizeInBytes),
			})
		}
	}

	if s.config.IncludeRowCounts {
		count, err := s.rowCount(ctx, client, table)
		if err != nil {
			log.Warn().Err(err).Str("table", table).Msg("Failed to count rows")
		} else {
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "asset.row_count",
				Value:      float64(count),
			})
		}
	}

	discovered := &discoveredTable{asset: asset, statistics: statistics}

	if s.config.DiscoverLineage {
		if realtime, ok := configs["REALTIME"]; ok {
			discovered.stream = s.streamAsset(table, realtime.streamConfig())
		}
	}

	return discovered, nil
}

// tableURL deep links into the controller UI's page for one table type.
func (s *Source) tableURL(tableNameWithType string) string {
	return s.config.ControllerURL + "/#/tenants/table/" + tableNameWithType
}

// addTableConfigMetadata copies the settings worth showing from one table
// config. Empty values are left out so the metadata stays readable.
func addTableConfigMetadata(metadata map[string]interface{}, config *tableConfig) {
	setString := func(key, value string) {
		if value != "" {
			metadata[key] = value
		}
	}

	setString("schema_name", config.SegmentsConfig.SchemaName)
	setString("time_column", config.SegmentsConfig.TimeColumnName)
	setString("time_type", config.SegmentsConfig.TimeType)
	setString("replication", config.SegmentsConfig.replicas())
	setString("broker_tenant", config.Tenants.Broker)
	setString("server_tenant", config.Tenants.Server)
	setString("load_mode", config.TableIndexConfig.LoadMode)

	if config.SegmentsConfig.RetentionTimeValue != "" && config.SegmentsConfig.RetentionTimeUnit != "" {
		metadata["retention"] = config.SegmentsConfig.RetentionTimeValue + " " + config.SegmentsConfig.RetentionTimeUnit
	}
	metadata["is_dim_table"] = config.IsDimTable
	if len(config.TableIndexConfig.SortedColumn) > 0 {
		metadata["sorted_column"] = config.TableIndexConfig.SortedColumn[0]
	}
	if len(config.TableIndexConfig.InvertedIndexColumns) > 0 {
		metadata["inverted_index_columns"] = config.TableIndexConfig.InvertedIndexColumns
	}
}

// replicas returns the replication factor. Offline tables call it
// replication, realtime tables replicasPerPartition.
func (c segmentsConfig) replicas() string {
	if c.Replication != "" {
		return c.Replication
	}
	return c.ReplicasPerPartition
}

// addStreamMetadata records where a realtime table reads from. Pinot keys
// every stream setting by type: stream.<type>.topic.name is the Kafka
// topic or Kinesis stream, stream.<type>.broker.list the Kafka brokers.
func addStreamMetadata(metadata map[string]interface{}, stream map[string]string) {
	streamType := streamType(stream)
	if streamType == "" {
		return
	}
	metadata["stream_type"] = streamType
	if topic := stream["stream."+streamType+".topic.name"]; topic != "" {
		metadata["stream_topic"] = topic
	}
	if brokers := stream["stream."+streamType+".broker.list"]; brokers != "" {
		metadata["stream_brokers"] = brokers
	}
}

func streamType(stream map[string]string) string {
	return strings.ToLower(strings.TrimSpace(stream["streamType"]))
}

// streamAsset builds the asset for the topic or stream feeding a realtime
// table, with the identity the Kafka and Kinesis plugins give it so the
// edge lands on the asset they own. Other stream types have no Marmot
// plugin to agree with, so they get no asset.
func (s *Source) streamAsset(table string, stream map[string]string) *pluginsdk.Asset {
	streamType := streamType(stream)
	topic := stream["stream."+streamType+".topic.name"]
	if topic == "" {
		return nil
	}

	var assetType, streamProvider string
	switch streamType {
	case "kafka":
		assetType, streamProvider = "Topic", "Kafka"
	case "kinesis":
		assetType, streamProvider = "Stream", "Kinesis"
	default:
		log.Debug().Str("table", table).Str("stream_type", streamType).Msg("No lineage for this stream type")
		return nil
	}

	metadata := map[string]interface{}{
		"stream_type": streamType,
	}
	if brokers := stream["stream."+streamType+".broker.list"]; brokers != "" {
		metadata["stream_brokers"] = brokers
	}

	name := topic
	mrnValue := mrn.New(assetType, streamProvider, name)

	return &pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{streamProvider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// tableSchema reads a table's schema, falling back to the schema named in
// the table config when it is not named after the table.
func (s *Source) tableSchema(ctx context.Context, client *Client, table, schemaName string) (*tableSchema, error) {
	schema, err := client.GetTableSchema(ctx, table)
	if err == nil {
		return schema, nil
	}
	if schemaName == "" || schemaName == table {
		return nil, err
	}
	log.Debug().Err(err).Str("table", table).Str("schema", schemaName).Msg("Falling back to the schema named in the table config")
	return client.GetSchema(ctx, schemaName)
}

// buildColumns flattens the three field groups of a Pinot schema into one
// column list, in the order Pinot lists them. Pinot has no NOT NULL
// constraint (a default value stands in for null), so every column is
// nullable unless the schema marks it notNull.
func buildColumns(schema *tableSchema) []column {
	var columns []column

	add := func(fieldType string, specs []fieldSpec) {
		for _, spec := range specs {
			singleValue := spec.SingleValueField == nil || *spec.SingleValueField
			dataType := spec.DataType
			if !singleValue {
				dataType += "[]"
			}
			columns = append(columns, column{
				Column: pluginsdk.Column{
					Name:       spec.Name,
					DataType:   dataType,
					Nullable:   !spec.NotNull,
					PrimaryKey: containsString(schema.PrimaryKeyColumns, spec.Name),
				},
				FieldType:        fieldType,
				SingleValue:      singleValue,
				Format:           spec.Format,
				Granularity:      spec.Granularity,
				DefaultNullValue: spec.DefaultNullValue,
			})
		}
	}

	add("dimension", schema.DimensionFieldSpecs)
	add("metric", schema.MetricFieldSpecs)
	add("datetime", schema.DateTimeFieldSpecs)

	return columns
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// rowCount runs an exact COUNT(*) over the logical table, which for a
// hybrid table spans both the offline and the realtime side.
func (s *Source) rowCount(ctx context.Context, client *Client, table string) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := client.Query(queryCtx, "SELECT COUNT(*) FROM "+quoteIdent(table))
	if err != nil {
		return 0, err
	}
	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return 0, fmt.Errorf("count query returned no rows")
	}

	count, ok := result.Rows[0][0].(float64)
	if !ok {
		return 0, fmt.Errorf("count query returned %T, expected a number", result.Rows[0][0])
	}
	return int64(count), nil
}

// FetchSampleData implements pluginsdk.DataFetcher: it returns the first
// rows of a table through the same query endpoint discovery uses.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	if _, err := s.Validate(config); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	table, _ := a.Metadata["table_name"].(string)
	if table == "" && a.Name != nil {
		table = *a.Name
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Debug().Str("table", table).Msg("Fetching sample data")

	result, err := s.newClient().Query(fetchCtx, "SELECT * FROM "+quoteIdent(table)+" LIMIT 20")
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}

	rows := result.Rows
	if rows == nil {
		rows = [][]interface{}{}
	}
	return result.DataSchema.ColumnNames, rows, nil
}

// assetMRN is the single place a Pinot table MRN is built, so the table
// pass and the lineage pass can never address the same table differently.
// Pinot has no schema layer and the OpenMetadata and Trino plugins already
// file Pinot tables by their bare name, so this does the same.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
