// Package superset discovers dashboards, charts, datasets and database
// connections from Apache Superset.
package superset

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Superset"

// Config for the Superset plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host      string `json:"host" description:"Superset URL (e.g. https://superset.example.com)" validate:"required,url"`
	Username  string `json:"username" description:"Username to log in with" validate:"required"`
	Password  string `json:"password" description:"Password to log in with" validate:"required" sensitive:"true"`
	Provider  string `json:"provider" description:"Authentication provider (db or ldap)" default:"db" validate:"oneof=db ldap"`
	VerifySSL bool   `json:"verify_ssl" label:"Verify SSL" description:"Verify the server's TLS certificate" default:"true"`

	IncludeCharts    bool `json:"include_charts" description:"Discover charts" default:"true"`
	IncludeDatasets  bool `json:"include_datasets" description:"Discover datasets" default:"true"`
	IncludeDatabases bool `json:"include_databases" description:"Discover database connections" default:"true"`
	IncludeDraft     bool `json:"include_draft" description:"Include unpublished (draft) dashboards" default:"true"`
	DiscoverLineage  bool `json:"discover_lineage" description:"Link dashboards, charts, datasets and the tables they read" default:"true"`
	PageSize         int  `json:"page_size" description:"Objects per API page" default:"100" validate:"min=1,max=1000"`
}

// Example configuration for the plugin
var _ = `
host: "https://superset.example.com"
username: "marmot"
password: "superset_secure_pass"
provider: "db"
include_draft: false
tags:
  - "superset"
  - "bi"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "superset",
		Name:        "Apache Superset",
		Description: "Discover dashboards, charts, datasets and database connections from Apache Superset",
		Icon:        "superset",
		Category:    "dashboard",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source implements the Superset plugin.
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
	config.Host = strings.TrimSuffix(strings.TrimSpace(config.Host), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Superset dashboards, charts, datasets and databases.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client := newClient(s.config.Host, 30*time.Second, s.config.VerifySSL)
	if err := client.login(ctx, s.config.Username, s.config.Password, s.config.Provider); err != nil {
		return nil, fmt.Errorf("logging in to Superset: %w", err)
	}

	d := &discovery{config: s.config, client: client, seen: make(map[string]bool)}

	// Databases come first even when they are not wanted as assets: the
	// backend and default database of each connection are what turn a
	// dataset into a reference to the table it reads.
	if err := d.discoverDatabases(ctx); err != nil {
		return nil, fmt.Errorf("discovering databases: %w", err)
	}
	if s.config.IncludeDatasets {
		if err := d.discoverDatasets(ctx); err != nil {
			return nil, fmt.Errorf("discovering datasets: %w", err)
		}
	}
	if s.config.IncludeCharts {
		if err := d.discoverCharts(ctx); err != nil {
			return nil, fmt.Errorf("discovering charts: %w", err)
		}
	}
	if err := d.discoverDashboards(ctx); err != nil {
		return nil, fmt.Errorf("discovering dashboards: %w", err)
	}
	if s.config.DiscoverLineage {
		d.linkDataFlow()
	}

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Msg("Superset discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  d.assets,
		Lineage: d.lineage,
	}, nil
}

// discovery accumulates one run's output and the lookups later passes
// need: which asset each Superset id became, so no edge is ever built
// to an asset this run did not create.
type discovery struct {
	config *Config
	client *client

	assets  []pluginsdk.Asset
	lineage []pluginsdk.LineageEdge
	seen    map[string]bool

	databases    map[int]databaseInfo
	datasets     []dataset
	datasetMRNs  map[int]string
	charts       []chart
	chartMRNs    map[int]string
	databaseMRNs map[int]string
}

// databaseInfo is what later passes need to know about a connection.
type databaseInfo struct {
	database
	// defaultDatabase is the database (or catalog) the connection opens,
	// from its parameters or the URI path. It is the first component of
	// a fully qualified table name.
	defaultDatabase string
}

func (d *discovery) add(asset pluginsdk.Asset) {
	d.assets = append(d.assets, asset)
}

// link records an edge once. Both ends must be an asset created in this
// run, or a table another plugin catalogues; the server drops anything
// else, so there is no point emitting it.
func (d *discovery) link(source, target, edgeType string) {
	key := source + ">" + target + ">" + edgeType
	if d.seen[key] {
		return
	}
	d.seen[key] = true
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

func (d *discovery) discoverDatabases(ctx context.Context) error {
	d.databases = make(map[int]databaseInfo)
	d.databaseMRNs = make(map[int]string)

	databases, err := d.client.databases(ctx, d.config.PageSize)
	if err != nil {
		return fmt.Errorf("listing databases: %w", err)
	}

	for _, db := range databases {
		info := databaseInfo{database: db}

		metadata := map[string]interface{}{
			"id":               db.ID,
			"backend":          db.Backend,
			"expose_in_sqllab": db.ExposeInSQLLab,
			"allow_dml":        db.AllowDML,
		}

		conn, err := d.client.connection(ctx, db.ID)
		if err != nil {
			log.Warn().Err(err).Str("database", db.DatabaseName).Msg("Failed to read database connection details")
		} else {
			info.defaultDatabase = strings.Split(conn.Parameters.Database, "/")[0]
			if info.defaultDatabase == "" {
				info.defaultDatabase = uriDatabase(conn.SQLAlchemyURI)
			}
			putIf(metadata, "driver", conn.Driver)
			putIf(metadata, "host", conn.Parameters.Host)
			putIf(metadata, "port", conn.Parameters.Port.String())
			putIf(metadata, "database", info.defaultDatabase)
			putIf(metadata, "sqlalchemy_uri", redactURI(conn.SQLAlchemyURI))
		}
		d.databases[db.ID] = info

		if !d.config.IncludeDatabases {
			continue
		}

		asset := d.newAsset("DataSource", db.DatabaseName, metadata)
		d.add(asset)
		d.databaseMRNs[db.ID] = *asset.MRN
	}

	log.Debug().Int("count", len(databases)).Msg("Discovered databases")
	return nil
}

// datasetName is the identity of a dataset: its schema and table name,
// which is also how Superset itself labels it in the chart list. Two
// datasets with the same table name in different schemas stay apart.
func datasetName(schema, table string) string {
	return join(schema, table)
}

func (d *discovery) discoverDatasets(ctx context.Context) error {
	d.datasetMRNs = make(map[int]string)

	listed, err := d.client.datasets(ctx, d.config.PageSize)
	if err != nil {
		return fmt.Errorf("listing datasets: %w", err)
	}

	for _, entry := range listed {
		ds := entry
		if detail, err := d.client.dataset(ctx, entry.ID); err != nil {
			log.Warn().Err(err).Str("dataset", datasetName(entry.Schema, entry.TableName)).Msg("Failed to read dataset detail, using the listing")
		} else {
			ds = *detail
			// The listing carries the change time; the detail does not.
			ds.ChangedOnUTC = entry.ChangedOnUTC
		}
		if ds.Database.Backend == "" {
			ds.Database.Backend = d.databases[ds.Database.ID].Backend
		}

		metadata := map[string]interface{}{
			"id":          ds.ID,
			"kind":        ds.Kind,
			"database":    ds.Database.DatabaseName,
			"database_id": ds.Database.ID,
			"table_name":  ds.TableName,
		}
		putIf(metadata, "backend", ds.Database.Backend)
		putIf(metadata, "schema", ds.Schema)
		putIf(metadata, "url", d.absoluteURL(ds.URL))
		putIf(metadata, "owners", ownerNames(ds.Owners))
		putIf(metadata, "description", ds.Description)
		putIf(metadata, "changed_on", ds.ChangedOnUTC)

		asset := d.newAsset("Data Model Object", datasetName(ds.Schema, ds.TableName), metadata)
		if desc := strings.TrimSpace(ds.Description); desc != "" {
			asset.Description = &desc
		}
		if ds.Kind == "virtual" {
			if sql := strings.TrimSpace(ds.SQL); sql != "" {
				language := "SQL"
				asset.Query = &sql
				asset.QueryLanguage = &language
			}
		}
		if ds.URL != "" {
			asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in Superset", URL: d.absoluteURL(ds.URL)}}
		}
		if len(ds.Columns) > 0 {
			if err := pluginsdk.SetColumns(&asset, columns(ds.Columns)); err != nil {
				log.Warn().Err(err).Str("dataset", *asset.Name).Msg("Failed to encode columns")
			}
		}

		d.add(asset)
		d.datasetMRNs[ds.ID] = *asset.MRN
		d.datasets = append(d.datasets, ds)
	}

	log.Debug().Int("count", len(d.datasets)).Msg("Discovered datasets")
	return nil
}

// column is a dataset column in the shape Marmot renders, plus the
// expression a calculated column is defined by.
type column struct {
	pluginsdk.Column
	Expression string `json:"expression,omitempty"`
}

func columns(cols []datasetColumn) []column {
	out := make([]column, 0, len(cols))
	for _, c := range cols {
		out = append(out, column{
			Column: pluginsdk.Column{
				Name:     c.ColumnName,
				DataType: c.Type,
				// Superset does not track nullability, so no column is
				// claimed to be required.
				Nullable:    true,
				Description: c.Description,
			},
			Expression: c.Expression,
		})
	}
	return out
}

func (d *discovery) discoverCharts(ctx context.Context) error {
	d.chartMRNs = make(map[int]string)

	charts, err := d.client.charts(ctx, d.config.PageSize)
	if err != nil {
		return fmt.Errorf("listing charts: %w", err)
	}

	for _, ch := range charts {
		if ch.SliceName == "" {
			log.Warn().Int("id", ch.ID).Msg("Skipping chart without a name")
			continue
		}

		url := ch.URL
		if url == "" {
			url = fmt.Sprintf("/explore/?slice_id=%d", ch.ID)
		}

		dashboards := make([]string, 0, len(ch.Dashboards))
		for _, ref := range ch.Dashboards {
			if ref.DashboardTitle != "" {
				dashboards = append(dashboards, ref.DashboardTitle)
			}
		}

		metadata := map[string]interface{}{
			"id":         ch.ID,
			"viz_type":   ch.VizType,
			"chart_type": chartType(ch.VizType),
			"url":        d.absoluteURL(url),
		}
		if ch.DatasourceID != 0 {
			metadata["datasource_id"] = ch.DatasourceID
		}
		putIf(metadata, "datasource_type", ch.DatasourceType)
		putIf(metadata, "dataset", ch.DatasourceNameText)
		putIf(metadata, "owners", ownerNames(ch.Owners))
		putIf(metadata, "dashboards", dashboards)
		putIf(metadata, "changed_on", ch.ChangedOnUTC)
		putIf(metadata, "tags", userTags(ch.Tags))

		asset := d.newAsset("Chart", ch.SliceName, metadata)
		if desc := strings.TrimSpace(ch.Description); desc != "" {
			asset.Description = &desc
		}
		asset.Tags = append(asset.Tags, userTags(ch.Tags)...)
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in Superset", URL: d.absoluteURL(url)}}

		d.add(asset)
		d.chartMRNs[ch.ID] = *asset.MRN
		d.charts = append(d.charts, ch)
	}

	log.Debug().Int("count", len(d.charts)).Msg("Discovered charts")
	return nil
}

func (d *discovery) discoverDashboards(ctx context.Context) error {
	dashboards, err := d.client.dashboards(ctx, d.config.PageSize)
	if err != nil {
		return fmt.Errorf("listing dashboards: %w", err)
	}

	discovered := 0
	for _, db := range dashboards {
		if !db.Published && !d.config.IncludeDraft {
			log.Debug().Str("dashboard", db.DashboardTitle).Msg("Skipping draft dashboard")
			continue
		}
		if db.DashboardTitle == "" {
			log.Warn().Int("id", db.ID).Msg("Skipping dashboard without a title")
			continue
		}

		chartIDs := d.dashboardChartIDs(ctx, db)

		metadata := map[string]interface{}{
			"id":          db.ID,
			"published":   db.Published,
			"chart_count": len(chartIDs),
		}
		putIf(metadata, "slug", db.Slug)
		putIf(metadata, "url", d.absoluteURL(db.URL))
		putIf(metadata, "status", db.Status)
		putIf(metadata, "owners", ownerNames(db.Owners))
		putIf(metadata, "tags", userTags(db.Tags))
		putIf(metadata, "changed_on", db.ChangedOnUTC)

		asset := d.newAsset("Dashboard", db.DashboardTitle, metadata)
		asset.Tags = append(asset.Tags, userTags(db.Tags)...)
		if db.URL != "" {
			asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in Superset", URL: d.absoluteURL(db.URL)}}
		}
		d.add(asset)
		discovered++

		if !d.config.DiscoverLineage {
			continue
		}
		for _, id := range chartIDs {
			if chartMRN, ok := d.chartMRNs[id]; ok {
				d.link(*asset.MRN, chartMRN, "CONTAINS")
			}
		}
	}

	log.Debug().Int("count", discovered).Msg("Discovered dashboards")
	return nil
}

// dashboardChartIDs returns the charts placed on a dashboard. The charts
// endpoint is the direct answer; when a server does not offer it the
// layout still names every chart, so that is read instead.
func (d *discovery) dashboardChartIDs(ctx context.Context, db dashboard) []int {
	ids, err := d.client.dashboardCharts(ctx, db.ID)
	if err == nil {
		return ids
	}
	log.Debug().Err(err).Str("dashboard", db.DashboardTitle).Msg("Charts endpoint failed, reading the dashboard layout")

	ids, err = d.client.dashboardLayoutCharts(ctx, db.ID)
	if err != nil {
		log.Warn().Err(err).Str("dashboard", db.DashboardTitle).Msg("Failed to read the charts on the dashboard")
		return nil
	}
	return ids
}

// linkDataFlow records how data moves: from the tables another system
// owns into datasets, from datasets into charts.
func (d *discovery) linkDataFlow() {
	for _, ch := range d.charts {
		if ch.DatasourceType != "table" {
			continue
		}
		if datasetMRN, ok := d.datasetMRNs[ch.DatasourceID]; ok {
			d.link(datasetMRN, d.chartMRNs[ch.ID], "FEEDS")
		}
	}

	for _, ds := range d.datasets {
		datasetMRN := d.datasetMRNs[ds.ID]

		if databaseMRN, ok := d.databaseMRNs[ds.Database.ID]; ok {
			d.link(databaseMRN, datasetMRN, "FEEDS")
		}

		info := d.databases[ds.Database.ID]
		backend := ds.Database.Backend
		if _, known := backendMap[strings.ToLower(backend)]; !known {
			log.Debug().
				Str("dataset", datasetName(ds.Schema, ds.TableName)).
				Str("backend", backend).
				Msg("No table naming rule for backend, skipping table lineage")
			continue
		}

		for _, ref := range d.sourceTables(ds) {
			database := ref.Database
			if database == "" {
				database = info.defaultDatabase
			}
			if tableMRN := nativeTableMRN(backend, database, ref.Schema, ref.Table); tableMRN != "" {
				d.link(tableMRN, datasetMRN, "FEEDS")
			}
		}
	}
}

// sourceTables returns the tables a dataset reads: its own table for a
// physical dataset, the ones named in its SQL for a virtual one.
func (d *discovery) sourceTables(ds dataset) []tableRef {
	if ds.Kind == "virtual" {
		return tableRefs(ds.SQL, ds.Schema)
	}
	return []tableRef{{Schema: ds.Schema, Table: ds.TableName}}
}

// newAsset builds an asset with the fields every kind shares.
func (d *discovery) newAsset(assetType, name string, metadata map[string]interface{}) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(d.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// absoluteURL turns the site-relative path Superset reports into a link.
func (d *discovery) absoluteURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return d.config.Host + "/" + strings.TrimPrefix(path, "/")
}

// assetMRN is the single place a Superset MRN is built, so every pass
// and every edge address an asset the same way.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

func ownerNames(owners []owner) []string {
	names := make([]string, 0, len(owners))
	for _, o := range owners {
		if name := strings.TrimSpace(o.FirstName + " " + o.LastName); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// userTags returns the names of the tags people applied, leaving out
// the ownership and favourite markers Superset stores alongside them.
func userTags(tags []tag) []string {
	var names []string
	for _, t := range tags {
		if t.Type == userTagType && t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names
}

// putIf sets a metadata key only when the value carries something, so
// the metadata map never holds empty strings or empty lists.
func putIf(metadata map[string]interface{}, key string, value interface{}) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	case nil:
		return
	}
	metadata[key] = value
}

// chartTypes normalises Superset's viz_type to the chart types other BI
// plugins report, so a bar chart is a Bar whichever tool drew it.
var chartTypes = map[string]string{
	"table":                   "Table",
	"pivot_table_v2":          "Table",
	"big_number":              "Line",
	"big_number_total":        "Line",
	"line":                    "Line",
	"echarts_timeseries_line": "Line",
	"echarts_timeseries":      "Line",
	"dist_bar":                "Bar",
	"bar":                     "Bar",
	"echarts_timeseries_bar":  "Bar",
	"pie":                     "Pie",
	"area":                    "Area",
	"echarts_area":            "Area",
	"treemap_v2":              "Area",
	"box_plot":                "BoxPlot",
	"histogram":               "Histogram",
	"scatter":                 "Scatter",
	"gauge_chart":             "Gauge",
	"world_map":               "Map",
	"country_map":             "Map",
	"graph_chart":             "Graph",
	"heatmap":                 "Heatmap",
	"heatmap_v2":              "Heatmap",
	"sankey":                  "SanKey",
	"sankey_v2":               "SanKey",
}

func chartType(vizType string) string {
	if t, ok := chartTypes[vizType]; ok {
		return t
	}
	if strings.HasPrefix(vizType, "deck_") {
		return "Map"
	}
	return "Other"
}
