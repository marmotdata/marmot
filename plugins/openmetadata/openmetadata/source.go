// Package openmetadata catalogues the contents of an OpenMetadata
// instance in Marmot.
//
// OpenMetadata is itself a catalog, so everything in it describes
// something that lives somewhere else: a table under a Postgres service
// is a Postgres table. This plugin reads OpenMetadata's REST API once
// and projects every entity onto the Marmot provider and MRN its native
// Marmot plugin would use, so a catalog imported from OpenMetadata looks
// the same as one built by Marmot's own plugins, and running both merges
// rather than duplicates. See projection.go for that mapping.
package openmetadata

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

const (
	// namingNative names assets the way the technology's own Marmot
	// plugin names them, so the two runs land on the same asset.
	namingNative = "native"
	// namingQualified names assets by their full OpenMetadata path,
	// keeping two services of the same technology apart.
	namingQualified = "qualified"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "openmetadata",
		Name:        "OpenMetadata",
		Description: "Import tables, topics, dashboards, pipelines, models, glossary and lineage from an OpenMetadata instance",
		Icon:        "openmetadata",
		Category:    "catalog",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage", "Run History", "Glossary"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the OpenMetadata plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection
	Host               string `json:"host" description:"OpenMetadata server URL, for example https://openmetadata.company.com" validate:"required,url"`
	JWTToken           string `json:"jwt_token" label:"JWT Token" description:"Bot token or personal access token from OpenMetadata" sensitive:"true" validate:"required"`
	TimeoutSeconds     int    `json:"timeout_seconds" description:"Per-request timeout" default:"60" validate:"omitempty,min=1"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" description:"Skip TLS certificate verification" default:"false"`

	// Scope
	Services            []string `json:"services,omitempty" description:"Only import these OpenMetadata services (all if empty)"`
	ExcludeServices     []string `json:"exclude_services,omitempty" description:"OpenMetadata services to skip"`
	ServiceTypes        []string `json:"service_types,omitempty" description:"Only import these OpenMetadata service types, for example Postgres or Kafka (all if empty)"`
	ExcludeServiceTypes []string `json:"exclude_service_types,omitempty" description:"OpenMetadata service types to skip"`
	IncludeDeleted      bool     `json:"include_deleted" description:"Import entities OpenMetadata has soft deleted" default:"false"`

	// What to import
	IncludeTables           bool `json:"include_tables" description:"Import databases, tables and views" default:"true"`
	IncludeStoredProcedures bool `json:"include_stored_procedures" description:"Import stored procedures as functions" default:"true"`
	IncludeTopics           bool `json:"include_topics" description:"Import messaging topics" default:"true"`
	IncludeContainers       bool `json:"include_containers" description:"Import object storage buckets and containers" default:"true"`
	// Off by default because Marmot's own S3, GCS and Azure Blob plugins
	// stop at the bucket, so an imported prefix is an asset no later
	// native run would ever touch again.
	IncludeContainerPrefixes bool `json:"include_container_prefixes" description:"Also import the prefixes and folders inside a storage container. Marmot's own object storage plugins catalogue only the container itself" default:"false"`
	IncludeDrives            bool `json:"include_drives" description:"Import drive directories, files, spreadsheets and worksheets" default:"true"`
	IncludeDashboards        bool `json:"include_dashboards" description:"Import dashboards, charts and dashboard data models" default:"true"`
	IncludePipelines         bool `json:"include_pipelines" description:"Import orchestration pipelines" default:"true"`
	IncludeTasks             bool `json:"include_tasks" description:"Import the individual tasks of each pipeline" default:"true"`
	IncludeRunHistory        bool `json:"include_run_history" description:"Import recent pipeline executions as run history" default:"true"`
	RunHistoryDays           int  `json:"run_history_days" description:"How many days of pipeline executions to import" default:"7" validate:"omitempty,min=1"`
	RunHistoryLimit          int  `json:"run_history_limit" description:"Maximum executions to import per pipeline" default:"50" validate:"omitempty,min=1,max=1000"`
	IncludeMLModels          bool `json:"include_mlmodels" label:"Include ML Models" description:"Import machine learning models" default:"true"`
	IncludeSearchIndexes     bool `json:"include_search_indexes" description:"Import search indices" default:"true"`
	IncludeAPIs              bool `json:"include_apis" label:"Include APIs" description:"Import API collections and endpoints" default:"true"`
	IncludeColumns           bool `json:"include_columns" description:"Import column, field and feature definitions" default:"true"`
	IncludeLineage           bool `json:"include_lineage" description:"Import lineage between imported assets" default:"true"`
	IncludeGlossary          bool `json:"include_glossary" description:"Import the business glossary as Marmot glossary terms, and assign them to the assets they are curated onto" default:"true"`

	// How to import
	Naming               string `json:"naming" description:"native names assets the way Marmot's own plugin for each technology names them, so a later native run merges with the imported assets. qualified uses the full OpenMetadata path, which keeps two services of the same technology apart" default:"native" validate:"omitempty,oneof=native qualified"`
	TagsFromOpenMetadata bool   `json:"tags_from_openmetadata" description:"Copy OpenMetadata classification tags onto assets" default:"true"`
	GlossaryTermsAsTags  bool   `json:"glossary_terms_as_tags" description:"Also copy assigned glossary terms onto assets as tags. They are imported as glossary terms either way" default:"false"`
	LinkToOpenMetadata   bool   `json:"link_to_openmetadata" label:"Link to OpenMetadata" description:"Add a link back to the entity in OpenMetadata on every asset" default:"true"`
	SourcePriority       int    `json:"source_priority" description:"Priority of OpenMetadata against other sources of the same asset. Lower wins" default:"2" validate:"omitempty,min=1"`

	// Performance
	PageSize    int `json:"page_size" description:"Entities per API request" default:"250" validate:"omitempty,min=1,max=1000"`
	Concurrency int `json:"concurrency" description:"Parallel lineage requests" default:"8" validate:"omitempty,min=1,max=64"`
}

// Example configuration for the plugin
var _ = `
host: "https://openmetadata.company.com"
jwt_token: "eyJraWQiOiJHYjM4OWEtOWY3Ni1nZGpzLWE5..."
exclude_service_types:
  - "Metadata"
tags:
  - "openmetadata"
`

// Source represents the OpenMetadata plugin
type Source struct {
	config *Config
}

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	if config.Naming == "" {
		config.Naming = namingNative
	}

	// ApplyDefaults leaves a key alone when the user wrote it, so an
	// explicit zero reaches here. A zero concurrency would build an
	// unbuffered semaphore and deadlock the lineage pass; the rest would
	// just behave nonsensically.
	applyFloor(&config.Concurrency, 8)
	applyFloor(&config.PageSize, 250)
	applyFloor(&config.TimeoutSeconds, 60)
	applyFloor(&config.SourcePriority, 2)
	applyFloor(&config.RunHistoryDays, 7)
	applyFloor(&config.RunHistoryLimit, 50)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover
	// cannot rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	client := newClient(s.config.Host, s.config.JWTToken, time.Duration(s.config.TimeoutSeconds)*time.Second, s.config.InsecureSkipVerify)
	c := newCollector(s.config)

	if err := s.collect(ctx, client, c); err != nil {
		return nil, err
	}

	for _, run := range c.deferred {
		run()
	}

	if s.config.IncludeLineage {
		c.discoverLineage(ctx, client)
	}

	c.report()

	return &pluginsdk.DiscoveryResult{
		Assets:        c.assets,
		Lineage:       c.lineage,
		RunHistory:    c.runHistory,
		GlossaryTerms: c.terms,
	}, nil
}

// collect runs each enabled reader in turn. Readers run in sequence
// because later ones link to assets earlier ones produced, and because a
// catalog import should be gentle on the OpenMetadata server it is
// reading; the parallelism that matters is in the lineage pass.
func (s *Source) collect(ctx context.Context, client *client, c *collector) error {
	// The glossary is read first because it is the vocabulary the assets
	// that follow are described in.
	if s.config.IncludeGlossary {
		if err := c.discoverGlossary(ctx, client); err != nil {
			return err
		}
	}

	if s.config.IncludeTables || s.config.IncludeStoredProcedures {
		groups, err := c.discoverDatabases(ctx, client)
		if err != nil {
			return err
		}
		if s.config.IncludeTables {
			if err := c.discoverTables(ctx, client, groups); err != nil {
				return err
			}
		}
		if s.config.IncludeStoredProcedures {
			if err := c.discoverStoredProcedures(ctx, client, groups); err != nil {
				return err
			}
		}
	}

	if s.config.IncludeTopics {
		if err := c.discoverTopics(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeContainers {
		if err := c.discoverContainers(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeDrives {
		if err := c.discoverDrives(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeDashboards {
		if err := c.discoverDashboards(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludePipelines {
		if err := c.discoverPipelines(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeMLModels {
		if err := c.discoverMLModels(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeSearchIndexes {
		if err := c.discoverSearchIndexes(ctx, client); err != nil {
			return err
		}
	}
	if s.config.IncludeAPIs {
		if err := c.discoverAPIs(ctx, client); err != nil {
			return err
		}
	}

	return nil
}

// wanted reports whether an entity is in scope for this run.
func (c *collector) wanted(base entityBase) bool {
	if base.Deleted && !c.config.IncludeDeleted {
		return false
	}

	service := base.Service.Name
	if len(c.config.Services) > 0 && !slices.Contains(c.config.Services, service) {
		c.skipped[base.ServiceType]++
		return false
	}
	if slices.Contains(c.config.ExcludeServices, service) {
		c.skipped[base.ServiceType]++
		return false
	}
	if len(c.config.ServiceTypes) > 0 && !containsFold(c.config.ServiceTypes, base.ServiceType) {
		c.skipped[base.ServiceType]++
		return false
	}
	if containsFold(c.config.ExcludeServiceTypes, base.ServiceType) {
		c.skipped[base.ServiceType]++
		return false
	}

	return true
}

// report logs what the run produced, per technology, so a large import
// is auditable without reading the catalog.
func (c *collector) report() {
	byProvider := make(map[string]int, len(c.assets))
	for _, asset := range c.assets {
		if len(asset.Providers) > 0 {
			byProvider[asset.Providers[0]]++
		}
	}

	for provider, count := range byProvider {
		log.Info().Str("provider", provider).Int("assets", count).Msg("Imported from OpenMetadata")
	}
	for serviceType, count := range c.skipped {
		log.Info().Str("service_type", serviceType).Int("entities", count).Msg("Skipped by configuration")
	}

	if len(c.collisions) == 0 {
		return
	}

	// Merging is deliberate, but it must never be silent: a run that
	// folds a hundred entities into fifty assets should say so.
	log.Warn().
		Int("entities", len(c.collisions)).
		Str("naming", c.config.Naming).
		Msg("OpenMetadata entities share a Marmot asset because they resolve to the same name. Set naming: qualified to keep them apart")

	for i, merged := range c.collisions {
		if i >= maxReportedCollisions {
			log.Warn().Int("remaining", len(c.collisions)-maxReportedCollisions).Msg("Further merged entities not listed")
			break
		}
		log.Warn().
			Str("mrn", merged.MRN).
			Str("first", openMetadataFQN(merged.First)).
			Str("second", openMetadataFQN(merged.Second)).
			Msg("Merged into one asset")
	}
}

// maxReportedCollisions caps how many merged entities a run lists
// individually; the total is always reported.
const maxReportedCollisions = 20

// openMetadataFQN pulls the OpenMetadata path back out of an asset's
// metadata for reporting.
func openMetadataFQN(metadata map[string]interface{}) string {
	om, ok := metadata["openmetadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	fqn, _ := om["fqn"].(string)
	return fqn
}

// applyFloor replaces a zero or negative setting with its default.
func applyFloor(value *int, fallback int) {
	if *value <= 0 {
		*value = fallback
	}
}

func containsFold(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if strings.EqualFold(candidate, needle) {
			return true
		}
	}
	return false
}
