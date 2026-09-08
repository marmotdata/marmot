// Package athena discovers data catalogs, databases, tables, workgroups and
// saved queries from Amazon Athena.
package athena

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// defaultCatalogName is the catalog Athena always serves, backed by the Glue
// Data Catalog of the account.
const defaultCatalogName = "AwsDataCatalog"

// The metadata_api choices.
const (
	metadataAPIAuto   = "auto"
	metadataAPIAthena = "athena"
	metadataAPIGlue   = "glue"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "athena",
		Name:        "AWS Athena",
		Description: "Discover databases, tables, workgroups and saved queries from Amazon Athena",
		Icon:        "athena",
		Category:    "data-warehouse",
		Status:      "experimental",
		// Discover emits CONTAINS, FEEDS and PRODUCES edges, so the manifest
		// declares Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Athena plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

	Catalogs         []string `json:"catalogs,omitempty" description:"Data catalogs to discover. All catalogs when empty"`
	ExcludeCatalogs  []string `json:"exclude_catalogs,omitempty" description:"Data catalogs to skip"`
	Databases        []string `json:"databases,omitempty" description:"Databases to discover. All databases when empty"`
	ExcludeDatabases []string `json:"exclude_databases,omitempty" description:"Databases to skip"`
	WorkGroups       []string `json:"workgroups,omitempty" description:"Workgroups to discover. All workgroups when empty"`

	IncludeWorkGroups   bool `json:"include_workgroups" description:"Whether to catalog workgroups" default:"true"`
	IncludeSavedQueries bool `json:"include_saved_queries" description:"Whether to catalog saved queries" default:"true"`
	IncludeColumns      bool `json:"include_columns" description:"Whether to include table columns" default:"true"`
	IncludePartitions   bool `json:"include_partitions" description:"Whether to include partition keys" default:"true"`
	DiscoverLineage     bool `json:"discover_lineage" description:"Whether to discover lineage between catalogs, databases, tables, buckets and saved queries" default:"true"`

	MetadataAPI string `json:"metadata_api" description:"Which API reads databases and tables: auto, athena or glue" default:"auto" validate:"omitempty,oneof=auto athena glue"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
catalogs:
  - "AwsDataCatalog"
exclude_databases:
  - "default"
include_workgroups: true
include_saved_queries: true
tags:
  - "aws"
  - "athena"
`

// Source represents the Athena plugin.
type Source struct {
	config *Config
	athena athenaAPI
	glue   glueAPI

	region    string
	accountID string
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// The embedded AWS section stays nil when the config names no AWS
	// setting at all, which is valid: credentials then come from the
	// environment.
	if config.AWSConfig == nil {
		config.AWSConfig = &pluginsdk.AWSConfig{}
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Athena data catalogs, databases, tables, workgroups and
// saved queries.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	awsConfig, err := pluginsdk.ExtractAWSConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.athena = athena.NewFromConfig(awsCfg)
	s.glue = glue.NewFromConfig(awsCfg)
	s.region = awsCfg.Region

	// Athena does not return ARNs for workgroups or catalogs, so reading
	// their tags means building the ARN, which needs the account id.
	if s.config.TagsToMetadata {
		s.accountID = callerAccountID(ctx, awsCfg)
	}

	return s.discover(ctx)
}

// discover is the discovery pass over the Athena and Glue APIs. It is
// separate from Discover so tests can drive it without an AWS session.
func (s *Source) discover(ctx context.Context) (*pluginsdk.DiscoveryResult, error) {
	catalogs, err := s.listCatalogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering data catalogs: %w", err)
	}
	if len(catalogs) == 0 {
		// Athena always serves the Glue Data Catalog under this name, even in
		// an account that registered no catalog of its own.
		log.Debug().Msg("No data catalogs registered, falling back to the default catalog")
		catalogs = []catalogSummary{{Name: defaultCatalogName, Type: string(athenatypes.DataCatalogTypeGlue)}}
	}

	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge
	tables := newTableIndex()

	for _, summary := range catalogs {
		if !s.wantCatalog(summary.Name) {
			continue
		}

		summary = s.describeCatalog(ctx, summary)
		assets = append(assets, s.catalogAsset(ctx, summary))

		reader := s.readerFor(ctx, summary)
		if reader == nil {
			continue
		}

		catalogAssets, catalogLineage := s.discoverCatalog(ctx, summary, reader, tables)
		assets = append(assets, catalogAssets...)
		lineage = append(lineage, catalogLineage...)
	}

	workGroupAssets, workGroupLineage := s.discoverWorkGroups(ctx, tables)
	assets = append(assets, workGroupAssets...)
	lineage = append(lineage, workGroupLineage...)

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineage)).
		Msg("Athena discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  assets,
		Lineage: lineage,
	}, nil
}

// discoverCatalog walks one data catalog's databases and tables. A database
// that cannot be read is logged and skipped, so one broken database does not
// cost the whole catalog.
func (s *Source) discoverCatalog(ctx context.Context, c catalogSummary, reader catalogReader, tables *tableIndex) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge

	databases, err := reader.databases(ctx, c.Name)
	if err != nil {
		log.Warn().Err(err).Str("catalog", c.Name).Str("api", reader.name()).Msg("Failed to list databases")
		return nil, nil
	}

	for _, db := range databases {
		if db.Name == "" || !s.wantDatabase(db.Name) {
			continue
		}

		assets = append(assets, s.databaseAsset(c, db))
		if s.config.DiscoverLineage {
			lineage = append(lineage, edge(catalogMRN(c.Name), databaseMRN(db.Name), "CONTAINS"))
		}

		catalogTables, err := reader.tables(ctx, c.Name, db.Name)
		if err != nil {
			log.Warn().Err(err).Str("catalog", c.Name).Str("database", db.Name).Str("api", reader.name()).
				Msg("Failed to list tables")
			continue
		}

		for _, t := range catalogTables {
			if t.Name == "" {
				continue
			}

			asset := s.tableAsset(c, db.Name, t)
			assets = append(assets, asset)
			tables.add(db.Name, t.Name, *asset.MRN)

			if !s.config.DiscoverLineage {
				continue
			}
			lineage = append(lineage, edge(databaseMRN(db.Name), *asset.MRN, "CONTAINS"))

			// The bucket is the storage the table reads, so the data flows
			// from the bucket into the table, not the other way round.
			if bucket := s3Bucket(t.Parameters["location"]); bucket != "" {
				lineage = append(lineage, edge(bucketMRN(bucket), *asset.MRN, "FEEDS"))
			}
		}
	}

	return assets, lineage
}

// discoverWorkGroups catalogs workgroups and their saved queries. Saved
// queries are listed per workgroup because Athena has no call that lists them
// across workgroups.
func (s *Source) discoverWorkGroups(ctx context.Context, tables *tableIndex) ([]pluginsdk.Asset, []pluginsdk.LineageEdge) {
	if !s.config.IncludeWorkGroups && !s.config.IncludeSavedQueries {
		return nil, nil
	}

	workGroups, err := s.listWorkGroups(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list workgroups")
		return nil, nil
	}

	var assets []pluginsdk.Asset
	var lineage []pluginsdk.LineageEdge

	for _, wg := range workGroups {
		name := deref(wg.Name)
		if name == "" {
			continue
		}

		if s.config.IncludeWorkGroups {
			assets = append(assets, s.workGroupAsset(ctx, wg))

			if s.config.DiscoverLineage {
				if bucket := s3Bucket(workGroupOutputLocation(wg)); bucket != "" {
					lineage = append(lineage, edge(workGroupMRN(name), bucketMRN(bucket), "PRODUCES"))
				}
			}
		}

		if !s.config.IncludeSavedQueries {
			continue
		}

		queries, err := s.listNamedQueries(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("workgroup", name).Msg("Failed to list saved queries")
			continue
		}

		for _, q := range queries {
			if deref(q.Name) == "" {
				continue
			}

			asset := s.savedQueryAsset(q, name)
			assets = append(assets, asset)

			if !s.config.DiscoverLineage {
				continue
			}
			// The workgroup asset has to exist for the edge to land.
			if s.config.IncludeWorkGroups {
				lineage = append(lineage, edge(workGroupMRN(name), *asset.MRN, "CONTAINS"))
			}
			lineage = append(lineage, tables.queryEdges(deref(q.QueryString), deref(q.Database), *asset.MRN)...)
		}
	}

	return assets, lineage
}

// readerFor picks the API that reads one catalog's databases and tables.
//
// A GLUE catalog is the Glue Data Catalog, so either API describes the same
// objects. The Athena metadata API is preferred because it is the only one
// that answers for federated catalogs, but not every Athena compatible
// endpoint implements it, so auto probes it once per catalog and falls back
// to Glue when the probe fails.
func (s *Source) readerFor(ctx context.Context, c catalogSummary) catalogReader {
	switch s.config.MetadataAPI {
	case metadataAPIAthena:
		return athenaReader{client: s.athena}
	case metadataAPIGlue:
		if !isGlueCatalog(c) {
			log.Warn().Str("catalog", c.Name).Str("type", c.Type).
				Msg("Skipping catalog, the Glue API only serves Glue backed catalogs")
			return nil
		}
		return glueReader{client: s.glue}
	}

	if !isGlueCatalog(c) {
		return athenaReader{client: s.athena}
	}
	if s.athenaMetadataAnswers(ctx, c.Name) {
		return athenaReader{client: s.athena}
	}

	log.Info().Str("catalog", c.Name).Msg("Athena metadata API did not answer, reading this catalog through Glue")
	return glueReader{client: s.glue}
}

// athenaMetadataAnswers reports whether the Athena metadata API can list the
// tables of a catalog. A catalog with no databases counts as no answer, since
// Glue may still hold databases the Athena view is not serving.
func (s *Source) athenaMetadataAnswers(ctx context.Context, catalog string) bool {
	databases, err := (athenaReader{client: s.athena}).databases(ctx, catalog)
	if err != nil || len(databases) == 0 {
		return false
	}

	probeLimit := int32(1)
	_, err = s.athena.ListTableMetadata(ctx, &athena.ListTableMetadataInput{
		CatalogName:  &catalog,
		DatabaseName: &databases[0].Name,
		MaxResults:   &probeLimit,
	})
	return err == nil
}

// isGlueCatalog reports whether a catalog is backed by the Glue Data Catalog.
// A catalog whose type Athena did not report is assumed to be Glue, which is
// what the default catalog is.
func isGlueCatalog(c catalogSummary) bool {
	if c.Type == "" {
		return true
	}
	return strings.EqualFold(c.Type, string(athenatypes.DataCatalogTypeGlue))
}

func (s *Source) catalogAsset(ctx context.Context, c catalogSummary) pluginsdk.Asset {
	metadata := s.tagMetadata(ctx, "datacatalog/"+c.Name)
	putString(metadata, "type", c.Type)
	putString(metadata, "description", c.Description)
	if len(c.Parameters) > 0 {
		metadata["parameters"] = c.Parameters
	}

	return s.asset(typeCatalog, providerAthena, c.Name, catalogMRN(c.Name), c.Description, metadata)
}

func (s *Source) databaseAsset(c catalogSummary, db catalogDatabase) pluginsdk.Asset {
	facts := map[string]any{"catalog": c.Name}
	putString(facts, "catalog_type", c.Type)
	putString(facts, "description", db.Description)
	putString(facts, "location_uri", db.LocationURI)
	if len(db.Parameters) > 0 {
		facts["parameters"] = db.Parameters
	}

	return s.asset(typeDatabase, providerGlue, db.Name, databaseMRN(db.Name), db.Description, athenaFacts(facts))
}

func (s *Source) tableAsset(c catalogSummary, database string, t catalogTable) pluginsdk.Asset {
	assetType := tableAssetType(t.TableType)
	comment := t.Parameters["comment"]

	facts := map[string]any{
		"catalog":  c.Name,
		"database": database,
	}
	putString(facts, "catalog_type", c.Type)
	putString(facts, "table_type", t.TableType)
	putString(facts, "location", t.Parameters["location"])
	putString(facts, "input_format", t.Parameters["inputformat"])
	putString(facts, "output_format", t.Parameters["outputformat"])
	putString(facts, "serde", t.Parameters["serde.serialization.lib"])
	putString(facts, "compression", t.Parameters["compressionType"])
	putString(facts, "classification", t.Parameters["classification"])
	putString(facts, "created", t.CreateTime)
	putString(facts, "last_access", t.LastAccessTime)
	putString(facts, "comment", comment)

	if s.config.IncludePartitions && len(t.PartitionKeys) > 0 {
		names := make([]string, 0, len(t.PartitionKeys))
		for _, pk := range t.PartitionKeys {
			names = append(names, pk.Name)
		}
		facts["partition_keys"] = strings.Join(names, ", ")
		if strings.EqualFold(t.Parameters["projection.enabled"], "true") {
			facts["partition_projection"] = true
		}
	}

	metadata := athenaFacts(facts)
	asset := s.asset(assetType, providerGlue, t.Name, tableMRN(assetType, t.Name), comment, metadata)

	if s.config.IncludeColumns {
		if columns := s.buildColumns(t); len(columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("table", t.Name).Msg("Failed to set columns")
			}
		}
	}

	if assetType == typeView {
		if sql := decodeViewText(t.Parameters["view_original_text"]); sql != "" {
			language := "SQL"
			asset.Query = &sql
			asset.QueryLanguage = &language
		}
	}

	return asset
}

// athenaColumn is a table column plus the one fact Athena adds to the shared
// shape: whether the column is a partition key.
type athenaColumn struct {
	pluginsdk.Column
	IsPartitionKey bool `json:"is_partition_key,omitempty"`
}

// buildColumns lists a table's columns with the Hive type recorded verbatim,
// so a nested type such as array<struct<sku:string>> survives intact.
// Partition keys come last, matching how Athena prints a table.
func (s *Source) buildColumns(t catalogTable) []athenaColumn {
	columns := make([]athenaColumn, 0, len(t.Columns)+len(t.PartitionKeys))

	for _, c := range t.Columns {
		columns = append(columns, athenaColumn{Column: pluginsdk.Column{
			Name:     c.Name,
			DataType: c.Type,
			// Hive and Athena have no NOT NULL constraint, so every column
			// accepts nulls.
			Nullable:    true,
			Description: c.Comment,
		}})
	}

	if !s.config.IncludePartitions {
		return columns
	}

	for _, c := range t.PartitionKeys {
		columns = append(columns, athenaColumn{
			Column: pluginsdk.Column{
				Name:        c.Name,
				DataType:    c.Type,
				Nullable:    true,
				Description: c.Comment,
			},
			IsPartitionKey: true,
		})
	}

	return columns
}

func (s *Source) workGroupAsset(ctx context.Context, wg athenatypes.WorkGroup) pluginsdk.Asset {
	name := deref(wg.Name)
	description := deref(wg.Description)

	metadata := s.tagMetadata(ctx, "workgroup/"+name)
	putString(metadata, "state", string(wg.State))
	putString(metadata, "description", description)
	putString(metadata, "created", formatTime(wg.CreationTime))
	putString(metadata, "region", s.region)

	if c := wg.Configuration; c != nil {
		metadata["enforce_configuration"] = aws.ToBool(c.EnforceWorkGroupConfiguration)
		metadata["publish_metrics"] = aws.ToBool(c.PublishCloudWatchMetricsEnabled)
		metadata["requester_pays"] = aws.ToBool(c.RequesterPaysEnabled)
		if c.BytesScannedCutoffPerQuery != nil {
			metadata["bytes_scanned_cutoff"] = *c.BytesScannedCutoffPerQuery
		}
		if c.EngineVersion != nil {
			putString(metadata, "engine_version", deref(c.EngineVersion.EffectiveEngineVersion))
			putString(metadata, "selected_engine_version", deref(c.EngineVersion.SelectedEngineVersion))
		}
		if r := c.ResultConfiguration; r != nil {
			putString(metadata, "output_location", deref(r.OutputLocation))
			if e := r.EncryptionConfiguration; e != nil {
				putString(metadata, "encryption", string(e.EncryptionOption))
				putString(metadata, "encryption_kms_key", deref(e.KmsKey))
			}
		}
	}

	url := workGroupConsoleURL(s.region, name)
	putString(metadata, "url", url)

	asset := s.asset(typeWorkGroup, providerAthena, name, workGroupMRN(name), description, metadata)
	if url != "" {
		asset.ExternalLinks = append(asset.ExternalLinks, pluginsdk.AssetExternalLink{
			Name: "Open in Athena",
			URL:  url,
		})
	}
	return asset
}

func (s *Source) savedQueryAsset(q athenatypes.NamedQuery, listedWorkGroup string) pluginsdk.Asset {
	workGroup := deref(q.WorkGroup)
	if workGroup == "" {
		workGroup = listedWorkGroup
	}

	name := savedQueryName(workGroup, deref(q.Name))
	description := deref(q.Description)

	metadata := map[string]any{}
	putString(metadata, "named_query_id", deref(q.NamedQueryId))
	putString(metadata, "database", deref(q.Database))
	putString(metadata, "workgroup", workGroup)
	putString(metadata, "description", description)

	asset := s.asset(typeSavedQuery, providerAthena, name, savedQueryMRN(workGroup, deref(q.Name)), description, metadata)

	if sql := strings.TrimSpace(deref(q.QueryString)); sql != "" {
		language := "SQL"
		asset.Query = &sql
		asset.QueryLanguage = &language
	}

	return asset
}

// sourceName labels what contributed an asset's properties. It is the plugin,
// not the provider: a table carries the Glue provider so an Athena run and a
// Glue run reach one asset, but the server merges an asset's sources by name,
// so sharing the name would make each run erase the other's properties
// instead of recording both.
const sourceName = "Athena"

// asset fills in the parts every Athena asset shares.
func (s *Source) asset(assetType, provider, name, mrnValue, description string, metadata map[string]any) pluginsdk.Asset {
	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       sourceName,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if description != "" {
		asset.Description = &description
	}
	return asset
}

// tagMetadata reads an Athena resource's tags into metadata. It returns an
// empty map when tags are not requested, so callers can always write into it.
func (s *Source) tagMetadata(ctx context.Context, resource string) map[string]any {
	if !s.config.TagsToMetadata {
		return map[string]any{}
	}

	tags := s.resourceTags(ctx, athenaARN(s.region, s.accountID, resource))
	return pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tags)
}

// athenaFacts nests what Athena knows about a Glue owned asset under one key.
// The table and database assets are shared with plugins/glue, which writes
// its own flat keys on them, so keeping the Athena view in its own section
// stops the two runs from overwriting each other.
func athenaFacts(facts map[string]any) map[string]any {
	return map[string]any{"athena": facts}
}

func (s *Source) wantCatalog(name string) bool {
	return wanted(name, s.config.Catalogs, s.config.ExcludeCatalogs)
}

func (s *Source) wantDatabase(name string) bool {
	return wanted(name, s.config.Databases, s.config.ExcludeDatabases)
}

func (s *Source) wantWorkGroup(name string) bool {
	return wanted(name, s.config.WorkGroups, nil)
}

// wanted applies an include list and an exclude list by exact name. An empty
// include list means everything.
func wanted(name string, include, exclude []string) bool {
	for _, e := range exclude {
		if e == name {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, i := range include {
		if i == name {
			return true
		}
	}
	return false
}

func workGroupOutputLocation(wg athenatypes.WorkGroup) string {
	if wg.Configuration == nil || wg.Configuration.ResultConfiguration == nil {
		return ""
	}
	return deref(wg.Configuration.ResultConfiguration.OutputLocation)
}

func edge(source, target, edgeType string) pluginsdk.LineageEdge {
	return pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType}
}

func putString(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func callerAccountID(ctx context.Context, cfg aws.Config) string {
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to resolve the AWS account id, resource tags will be skipped")
		return ""
	}
	return deref(out.Account)
}
