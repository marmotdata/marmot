// Package bigtable discovers instances, tables and column families from
// Google Cloud Bigtable.
package bigtable

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	bt "cloud.google.com/go/bigtable"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string every Bigtable asset carries. The
// OpenMetadata import writes the same one, so both paths land on the same
// asset.
const provider = "Bigtable"

// Config for the Bigtable plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	pluginsdk.GCPConfig  `json:",inline"`

	ProjectID    string   `json:"project_id" label:"Project ID" description:"Google Cloud project ID" validate:"required"`
	Instances    []string `json:"instances,omitempty" description:"Instance IDs to discover. Leave empty to discover every instance in the project"`
	EmulatorHost string   `json:"emulator_host,omitempty" label:"Emulator Host" description:"host:port of a Bigtable emulator. Connects without credentials and requires instances to be listed"`

	IncludeColumns    bool `json:"include_columns" description:"Read a sample of rows to find the columns each table holds" default:"true"`
	SampleRows        int  `json:"sample_rows" description:"Rows to read per table when finding columns" default:"100" validate:"omitempty,min=1,max=10000"`
	IncludeStatistics bool `json:"include_statistics" description:"Count the rows in each table. This reads the whole table" default:"false"`
	MaxCountRows      int  `json:"max_count_rows" description:"Give up counting a table after this many rows" default:"100000" validate:"omitempty,min=1"`
	IncludeBackups    bool `json:"include_backups" description:"Count the backups held by each instance" default:"false"`
}

// Example configuration for the plugin
var _ = `
project_id: "company-analytics"
instances:
  - "prod-metrics"
credentials:
  credentials_file: "/etc/marmot/bigtable-service-account.json"
include_statistics: true
tags:
  - "bigtable"
  - "nosql"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "bigtable",
		Name:        "Bigtable",
		Description: "Discover instances, tables and column families from Google Cloud Bigtable",
		Icon:        "bigtable",
		Category:    "database",
		Status:      "experimental",
		// Discover emits CONTAINS edges from an instance to its tables, so
		// the manifest declares Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Bigtable plugin.
type Source struct {
	config *Config
}

// column is one column in a table asset's schema. Bigtable stores every
// value as raw bytes and declares no types, so the extras record what a
// sample of rows suggested rather than anything the table promises.
type column struct {
	pluginsdk.Column
	ColumnFamily string `json:"column_family,omitempty"`
	Qualifier    string `json:"qualifier,omitempty"`
	Occurrence   int    `json:"occurrence,omitempty"`
	InferredType string `json:"inferred_type,omitempty"`
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	config.EmulatorHost = strings.TrimSpace(config.EmulatorHost)
	if config.EmulatorHost != "" {
		if config.Credentials.CredentialsJSON != "" || config.Credentials.CredentialsFile != "" {
			return nil, fmt.Errorf("emulator_host cannot be combined with credentials: an emulator has no Google account to authenticate against")
		}
		if len(config.Instances) == 0 {
			return nil, fmt.Errorf("instances must be listed when emulator_host is set: the emulator cannot list its own instances")
		}
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Bigtable instances, tables and columns.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	instances, err := s.listInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolving instances: %w", err)
	}

	log.Debug().Int("count", len(instances)).Msg("Starting Bigtable discovery")

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	failed := 0
	var lastErr error

	for _, instance := range instances {
		result, err := s.discoverInstance(ctx, instance)
		if err != nil {
			// One unreachable instance should not lose the others.
			log.Warn().Err(err).Str("instance", instance.ID).Msg("Failed to discover instance")
			failed++
			lastErr = err
			continue
		}
		assets = append(assets, result.Assets...)
		lineages = append(lineages, result.Lineage...)
		statistics = append(statistics, result.Statistics...)
	}

	// Every instance failing is not a partial failure, it is an
	// unreachable system, and reporting success would look like a project
	// with nothing in it.
	if failed > 0 && failed == len(instances) {
		return nil, fmt.Errorf("discovering instances: %w", lastErr)
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Bigtable discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// discoverInstance builds the instance asset and one asset per table in it.
func (s *Source) discoverInstance(ctx context.Context, instance instanceDetail) (*pluginsdk.DiscoveryResult, error) {
	adminClient, err := bt.NewAdminClient(ctx, s.config.ProjectID, instance.ID, s.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("creating admin client: %w", err)
	}
	defer adminClient.Close()

	adminCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tables, err := adminClient.Tables(adminCtx)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	sort.Strings(tables)

	var dataClient *bt.Client
	if s.config.IncludeColumns || s.config.IncludeStatistics {
		dataClient, err = bt.NewClient(ctx, s.config.ProjectID, instance.ID, s.clientOptions()...)
		if err != nil {
			return nil, fmt.Errorf("creating data client: %w", err)
		}
		defer dataClient.Close()
	}

	backups := 0
	if s.config.IncludeBackups {
		backups = backupCount(ctx, adminClient, instance.Clusters)
	}

	instanceAsset := s.instanceAsset(instance, len(tables), backups)
	assets := []pluginsdk.Asset{instanceAsset}

	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	for _, table := range tables {
		asset, stats, err := s.tableAsset(ctx, adminClient, dataClient, instance, table)
		if err != nil {
			// A table that cannot be read should not lose its neighbours.
			log.Warn().Err(err).Str("instance", instance.ID).Str("table", table).Msg("Failed to discover table")
			continue
		}

		assets = append(assets, *asset)
		statistics = append(statistics, stats...)
		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: *instanceAsset.MRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
	}

	return &pluginsdk.DiscoveryResult{Assets: assets, Lineage: lineages, Statistics: statistics}, nil
}

func (s *Source) instanceAsset(instance instanceDetail, tableCount, backups int) pluginsdk.Asset {
	metadata := map[string]interface{}{
		"project_id":  s.config.ProjectID,
		"instance_id": instance.ID,
		"table_count": tableCount,
		"emulator":    s.config.EmulatorHost != "",
	}
	if instance.DisplayName != "" {
		metadata["display_name"] = instance.DisplayName
	}
	if instance.State != "" {
		metadata["state"] = instance.State
	}
	if instance.Type != "" {
		metadata["instance_type"] = instance.Type
	}
	if len(instance.Labels) > 0 {
		metadata["labels"] = instance.Labels
	}
	if len(instance.Clusters) > 0 {
		metadata["clusters"] = instance.Clusters
	}
	if s.config.IncludeBackups {
		metadata["backup_count"] = backups
	}

	name := instance.ID
	mrnValue := assetMRN("Instance", name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Instance",
		Providers: []string{provider},
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

func (s *Source) tableAsset(ctx context.Context, adminClient *bt.AdminClient, dataClient *bt.Client, instance instanceDetail, table string) (*pluginsdk.Asset, []pluginsdk.Statistic, error) {
	infoCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info, err := adminClient.TableInfo(infoCtx, table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading table info: %w", err)
	}

	families := make([]string, 0, len(info.FamilyInfos))
	policies := make(map[string]string, len(info.FamilyInfos))
	for _, family := range info.FamilyInfos {
		families = append(families, family.Name)
		if text := gcPolicyText(family); text != "" {
			policies[family.Name] = text
		}
	}
	sort.Strings(families)

	metadata := map[string]interface{}{
		"project_id":          s.config.ProjectID,
		"instance_id":         instance.ID,
		"table_name":          table,
		"deletion_protection": info.DeletionProtection == bt.Protected,
	}
	if len(families) > 0 {
		metadata["column_families"] = families
	}
	if len(policies) > 0 {
		metadata["gc_policies"] = policies
	}
	if retention := changeStreamRetentionText(info.ChangeStreamRetention); retention != "" {
		metadata["change_stream_retention"] = retention
	}

	var links []pluginsdk.AssetExternalLink
	if url := s.consoleURL(instance.ID, table); url != "" {
		metadata["url"] = url
		links = append(links, pluginsdk.AssetExternalLink{Name: "Open in Google Cloud Console", URL: url})
	}

	// Every table has a row key, and it needs no sample to find.
	columns := []column{rowKeyColumn()}

	if s.config.IncludeColumns && dataClient != nil {
		sampleCtx, sampleCancel := context.WithTimeout(ctx, 30*time.Second)
		sampled, rows, err := sampleColumns(sampleCtx, dataClient.Open(table), s.config.SampleRows)
		sampleCancel()
		if err != nil {
			log.Warn().Err(err).Str("instance", instance.ID).Str("table", table).Msg("Failed to sample columns")
		} else {
			columns = sampled
			metadata["sampled_rows"] = rows
		}
	}

	name := tableName(instance.ID, table)
	mrnValue := assetMRN("Table", name)

	asset := pluginsdk.Asset{
		Name:          &name,
		MRN:           &mrnValue,
		Type:          "Table",
		Providers:     []string{provider},
		Metadata:      metadata,
		Schema:        make(map[string]string),
		Tags:          pluginsdk.InterpolateTags(s.config.Tags, metadata),
		ExternalLinks: links,
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if err := pluginsdk.SetColumns(&asset, columns); err != nil {
		log.Warn().Err(err).Str("table", table).Msg("Failed to set columns")
	}

	statistics := []pluginsdk.Statistic{{
		AssetMRN:   mrnValue,
		MetricName: "asset.column_count",
		Value:      float64(len(columns)),
	}}

	if s.config.IncludeStatistics && dataClient != nil {
		countCtx, countCancel := context.WithTimeout(ctx, 5*time.Minute)
		count, complete, err := countRows(countCtx, dataClient.Open(table), s.config.MaxCountRows)
		countCancel()
		switch {
		case err != nil:
			log.Warn().Err(err).Str("instance", instance.ID).Str("table", table).Msg("Failed to count rows")
		case !complete:
			// A count that stopped at the limit is a floor, not a row
			// count, and reporting it as one would be wrong.
			log.Warn().Str("instance", instance.ID).Str("table", table).Int("limit", s.config.MaxCountRows).
				Msg("Table has more rows than max_count_rows, skipping the row count")
		default:
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   mrnValue,
				MetricName: "asset.row_count",
				Value:      float64(count),
			})
		}
	}

	return &asset, statistics, nil
}

// FetchSampleData reads a page of rows for an asset preview. Values are
// shown as text when they are readable and base64 otherwise, because
// Bigtable stores everything as raw bytes.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, asset *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if asset == nil || asset.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	instanceID, _ := asset.Metadata["instance_id"].(string)
	table, _ := asset.Metadata["table_name"].(string)
	if instanceID == "" || table == "" {
		return nil, nil, fmt.Errorf("could not determine instance and table from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := bt.NewClient(fetchCtx, s.config.ProjectID, instanceID, s.clientOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to Bigtable: %w", err)
	}
	defer client.Close()

	log.Debug().Str("instance", instanceID).Str("table", table).Msg("Fetching sample data")

	// A row is a set of cells, so it has to be collected into a map before
	// it can be laid out as a table row, and the columns are only known
	// once every sampled row has been seen.
	var keys []string
	var cells []map[string]string
	columns := make(map[string]bool)

	err = client.Open(table).ReadRows(fetchCtx, bt.InfiniteRange(""), func(row bt.Row) bool {
		values := make(map[string]string)
		for _, items := range row {
			for _, item := range items {
				values[item.Column] = renderValue(item.Value)
				columns[item.Column] = true
			}
		}
		keys = append(keys, row.Key())
		cells = append(cells, values)
		return true
	}, bt.LimitRows(sampleDataRows), bt.RowFilter(bt.LatestNFilter(1)))
	if err != nil {
		return nil, nil, fmt.Errorf("reading rows: %w", err)
	}

	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)

	columnNames := append([]string{"row_key"}, names...)
	rows := make([][]interface{}, 0, len(cells))
	for i, values := range cells {
		row := make([]interface{}, 0, len(columnNames))
		row = append(row, keys[i])
		for _, name := range names {
			row = append(row, values[name])
		}
		rows = append(rows, row)
	}

	return columnNames, rows, nil
}

// sampleDataRows is how many rows an asset preview shows.
const sampleDataRows = 20

// tableName is the identity a Bigtable table is addressed by. Table names
// are only unique within an instance, so the instance goes in front, the
// same shape an OpenMetadata import of a Bigtable service produces.
func tableName(instanceID, table string) string {
	return instanceID + "." + table
}

// assetMRN is the single place a Bigtable MRN is built. The asset pass and
// the lineage pass both go through it so the two can never drift into
// addressing the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// consoleURL is the Google Cloud Console page for a table. An emulator has
// no console, so it gets no link.
func (s *Source) consoleURL(instanceID, table string) string {
	if s.config.EmulatorHost != "" {
		return ""
	}
	return fmt.Sprintf("https://console.cloud.google.com/bigtable/instances/%s/tables/%s/overview?project=%s",
		instanceID, table, s.config.ProjectID)
}

// rowKeyColumn is the row key every Bigtable table has. It belongs to no
// column family, so no sample of rows can turn it up, but it is the
// table's only key and belongs at the top of the schema.
func rowKeyColumn() column {
	return column{Column: pluginsdk.Column{
		Name:       "row_key",
		DataType:   "bytes",
		PrimaryKey: true,
	}}
}

// splitColumn splits Bigtable's "family:qualifier" column notation. A
// qualifier may itself contain a colon, so only the first one separates.
func splitColumn(name string) (family, qualifier string) {
	family, qualifier, found := strings.Cut(name, ":")
	if !found {
		return name, ""
	}
	return family, qualifier
}

// inferValueType guesses what a cell holds. Bigtable stores every value as
// raw bytes, so the only evidence is the bytes: readable text first, then
// the 8 byte number the increment operation writes, and binary for
// anything else.
func inferValueType(value []byte) string {
	if len(value) == 0 {
		return "binary"
	}
	if isReadableText(value) {
		return "text"
	}
	if len(value) == 8 {
		return "int64"
	}
	return "binary"
}

func isReadableText(value []byte) bool {
	if !utf8.Valid(value) {
		return false
	}
	for _, r := range string(value) {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// mergeType settles on a type for a column seen in several rows. Bigtable
// does not declare types, so a column only keeps a specific one while
// every sampled value agreed; a disagreement falls back to binary, which
// is what the bytes always are.
func mergeType(current, next string) string {
	if current == "" {
		return next
	}
	if current == next {
		return current
	}
	return "binary"
}

// renderValue turns a cell's bytes into something displayable, falling
// back to base64 for values that are not text.
func renderValue(value []byte) string {
	if isReadableText(value) {
		return string(value)
	}
	return base64.StdEncoding.EncodeToString(value)
}

// gcPolicyText renders a column family's garbage collection rule. The
// client hands back both a rendered string and a policy object; the
// object renders the empty string when a family has no rule and the
// string renders "<never>", so both stand for "no rule" and neither is
// worth showing.
func gcPolicyText(family bt.FamilyInfo) string {
	if family.FullGCPolicy != nil {
		return family.FullGCPolicy.String()
	}
	if family.GCPolicy == "<never>" {
		return ""
	}
	return family.GCPolicy
}

// changeStreamRetentionText renders how long a table keeps change data.
// The field is an optional duration, so it is nil on a table with change
// streams switched off.
func changeStreamRetentionText(retention bt.ChangeStreamRetention) string {
	duration, ok := retention.(time.Duration)
	if !ok || duration <= 0 {
		return ""
	}
	return duration.String()
}

func instanceStateText(state bt.InstanceState) string {
	switch state {
	case bt.Ready:
		return "READY"
	case bt.Creating:
		return "CREATING"
	default:
		return ""
	}
}

func instanceTypeText(instanceType bt.InstanceType) string {
	switch instanceType {
	case bt.PRODUCTION:
		return "PRODUCTION"
	case bt.DEVELOPMENT:
		return "DEVELOPMENT"
	default:
		return ""
	}
}

func storageTypeText(storageType bt.StorageType) string {
	if storageType == bt.HDD {
		return "HDD"
	}
	return "SSD"
}
