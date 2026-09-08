// Package redash discovers dashboards, charts, queries and data sources
// from a Redash instance.
package redash

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string this plugin publishes. It is the
// service component of every MRN it creates.
const provider = "Redash"

var _ = `
runs:
  - redash:
      host: https://redash.example.com
      api_key: ${REDASH_API_KEY}
      include_queries: true
      include_data_sources: true
      discover_lineage: true
      tags:
        - bi
`

// Config for the Redash plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host   string `json:"host" description:"Redash base URL, for example https://redash.example.com" validate:"required,url"`
	APIKey string `json:"api_key" label:"API Key" description:"Redash user API key, found on the user profile page" validate:"required" sensitive:"true"`

	VerifySSL bool `json:"verify_ssl" description:"Whether to verify the server TLS certificate" default:"true"`

	IncludeQueries     bool `json:"include_queries" description:"Whether to catalogue saved queries" default:"true"`
	IncludeDataSources bool `json:"include_data_sources" description:"Whether to catalogue data sources" default:"true"`
	IncludeArchived    bool `json:"include_archived" description:"Whether to include archived dashboards and queries" default:"false"`
	IncludeDrafts      bool `json:"include_drafts" description:"Whether to include unpublished dashboards and queries" default:"true"`

	DiscoverLineage bool `json:"discover_lineage" description:"Whether to link dashboards, charts, queries and the tables they read" default:"true"`

	PageSize int `json:"page_size" description:"Results to request per API page" default:"100" validate:"omitempty,min=1,max=250"`
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "redash",
		Name:        "Redash",
		Description: "Discover dashboards, charts, queries and data sources from Redash",
		Icon:        "redash",
		Category:    "dashboard",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Redash plugin.
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

	// Every request path is appended to the host, so a trailing slash would
	// produce a double slash Redash answers with a redirect.
	config.Host = strings.TrimRight(strings.TrimSpace(config.Host), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Redash dashboards, charts, queries and data sources.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	client := NewClient(ClientConfig{
		BaseURL:   s.config.Host,
		APIKey:    s.config.APIKey,
		PageSize:  s.config.PageSize,
		VerifySSL: s.config.VerifySSL,
	})

	return s.discover(ctx, client)
}

// discover holds the discovery body so tests can drive it against a fake
// Redash without going through config validation twice.
func (s *Source) discover(ctx context.Context, client *Client) (*pluginsdk.DiscoveryResult, error) {
	version := s.detectVersion(ctx, client)

	collector := &collector{
		config:  s.config,
		client:  client,
		version: version,
		names:   make(map[string]struct{}),
		queries: make(map[int]string),
		sources: make(map[int]DataSource),
	}

	if err := collector.collectDataSources(ctx); err != nil {
		return nil, err
	}
	if err := collector.collectQueries(ctx); err != nil {
		return nil, err
	}
	if err := collector.collectDashboards(ctx); err != nil {
		return nil, err
	}

	log.Info().
		Int("assets", len(collector.assets)).
		Int("lineages", len(collector.lineage)).
		Str("version", version.raw).
		Msg("Redash discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  collector.assets,
		Lineage: collector.lineage,
	}, nil
}

// detectVersion asks Redash which release it is. Only the dashboard URL
// shape depends on it, so a version Redash declines to report degrades to
// the current shape rather than failing discovery.
func (s *Source) detectVersion(ctx context.Context, client *Client) version {
	session, err := client.GetSession(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read the Redash session, assuming a current release")
		return version{}
	}
	return parseVersion(session.ClientConfig.Version)
}

// collector accumulates the assets and edges of one discovery run.
type collector struct {
	config  *Config
	client  *Client
	version version

	assets  []pluginsdk.Asset
	lineage []pluginsdk.LineageEdge

	// names keeps asset names unique per type. Redash lets two dashboards
	// or two queries share a name, and Marmot's identity is the name.
	names map[string]struct{}

	// queries maps a Redash query id to the MRN of the asset built for it,
	// so a chart and a table can both point at the same query asset.
	queries map[int]string

	// sources maps a Redash data source id to the data source, used for a
	// query's language, its connection options and its display name.
	sources map[int]DataSource
}

// collectDataSources reads the data sources. It runs even when the data
// source assets are turned off, because a query's language and the identity
// of the tables it reads both come from its data source.
func (c *collector) collectDataSources(ctx context.Context) error {
	sources, err := c.client.ListDataSources(ctx)
	if err != nil {
		return fmt.Errorf("listing data sources: %w", err)
	}

	// The list endpoint omits connection options; only the detail endpoint
	// returns them, and they are needed both for the data source asset and
	// to qualify table names in lineage.
	needOptions := c.config.IncludeDataSources || c.config.DiscoverLineage

	for _, source := range sources {
		if needOptions {
			detail, err := c.client.GetDataSource(ctx, source.ID)
			if err != nil {
				log.Warn().Err(err).Int("data_source_id", source.ID).Str("name", source.Name).Msg("Failed to read data source options")
			} else {
				source = *detail
			}
		}
		c.sources[source.ID] = source

		if c.config.IncludeDataSources {
			c.assets = append(c.assets, c.dataSourceAsset(source))
		}
	}

	log.Debug().Int("count", len(sources)).Msg("Discovered data sources")
	return nil
}

func (c *collector) dataSourceAsset(source DataSource) pluginsdk.Asset {
	metadata := map[string]any{
		"id":        source.ID,
		"type":      source.Type,
		"view_only": source.ViewOnly,
		"paused":    bool(source.Paused),
	}
	setIf(metadata, "syntax", source.Syntax)
	setIf(metadata, "pause_reason", source.PauseReason)

	// Connection options are copied through an allowlist rather than a
	// denylist. Every Redash runner has its own option set, so anything not
	// known to be safe stays out; Redash redacts the password it returns,
	// but a fork or a future runner could name a secret something else.
	for key, value := range source.Options {
		metadataKey, allowed := connectionOptionKeys[key]
		if !allowed {
			continue
		}
		setIf(metadata, metadataKey, value)
	}

	name := c.takeName("DataSource", source.Name, source.ID)
	mrnValue := assetMRN("DataSource", name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "DataSource",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      c.tagsFor(nil, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// collectQueries catalogues saved queries as Data Model Objects and links
// each one to its data source and to the tables it reads.
func (c *collector) collectQueries(ctx context.Context) error {
	if !c.config.IncludeQueries {
		return nil
	}

	queries, err := c.client.ListQueries(ctx)
	if err != nil {
		return fmt.Errorf("listing queries: %w", err)
	}

	// Redash keeps archived queries out of the main list and serves them
	// from their own endpoint.
	if c.config.IncludeArchived {
		archived, err := c.client.ListArchivedQueries(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to list archived queries")
		} else {
			queries = append(queries, archived...)
		}
	}

	for _, query := range queries {
		if !c.included(query.IsArchived, query.IsDraft) {
			log.Debug().Int("query_id", query.ID).Str("name", query.Name).Msg("Skipping query")
			continue
		}
		// The two list endpoints are disjoint on current Redash, but a
		// query appearing on both would otherwise become two assets, the
		// second one renamed to dodge the first.
		if _, seen := c.queries[query.ID]; seen {
			continue
		}
		c.addQuery(query)
	}

	log.Debug().Int("count", len(c.queries)).Msg("Discovered queries")
	return nil
}

func (c *collector) addQuery(query Query) {
	source, hasSource := c.sources[query.DataSourceID]

	metadata := map[string]any{
		"id":          query.ID,
		"is_draft":    query.IsDraft,
		"is_archived": query.IsArchived,
		"url":         fmt.Sprintf("%s/queries/%d", c.config.Host, query.ID),
	}
	setIf(metadata, "description", query.Description)
	setIf(metadata, "data_source_id", query.DataSourceID)
	setIf(metadata, "created_at", query.CreatedAt)
	setIf(metadata, "updated_at", query.UpdatedAt)
	setIf(metadata, "tags", query.Tags)
	if query.User != nil {
		setIf(metadata, "owner", query.User.Name)
	}
	if hasSource {
		setIf(metadata, "data_source", source.Name)
	}
	if query.Runtime != nil {
		metadata["runtime"] = *query.Runtime
	}
	if schedule := scheduleMetadata(query.Schedule); len(schedule) > 0 {
		metadata["schedule"] = schedule
	}
	if parameters := parameterMetadata(query.Options.Parameters); len(parameters) > 0 {
		metadata["parameters"] = parameters
	}

	name := c.takeName("Data Model Object", query.Name, query.ID)
	mrnValue := assetMRN("Data Model Object", name)
	c.queries[query.ID] = mrnValue

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Data Model Object",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      c.tagsFor(query.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Redash",
			URL:  metadata["url"].(string),
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if query.Description != "" {
		asset.Description = &query.Description
	}
	if query.Query != "" {
		text := query.Query
		asset.Query = &text
		if hasSource {
			if language := queryLanguage(source.Syntax); language != "" {
				asset.QueryLanguage = &language
			}
		}
	}

	c.assets = append(c.assets, asset)

	if !c.config.DiscoverLineage {
		return
	}
	if c.config.IncludeDataSources && hasSource {
		c.addEdge(assetMRN("DataSource", source.Name), mrnValue, "FEEDS")
	}
	if hasSource {
		c.addTableLineage(query, source, mrnValue)
	}
}

// addTableLineage links the tables a query reads to the query asset. The
// table MRNs are built with the provider and naming rule of the plugin that
// owns the queried system, so the edge lands on that plugin's asset instead
// of minting a second one.
func (c *collector) addTableLineage(query Query, source DataSource, queryMRN string) {
	target, known := targetForDataSource(source.Type)
	if !known {
		log.Debug().Str("data_source_type", source.Type).Int("query_id", query.ID).Msg("No Marmot identity known for this data source type, skipping table lineage")
		return
	}

	for _, ref := range extractTableRefs(query.Query) {
		name, ok := target.assetName(source.Options, ref)
		if !ok {
			log.Debug().Str("table", ref.String()).Str("data_source_type", source.Type).Msg("Table reference is missing a qualifier the owning plugin needs, skipping")
			continue
		}
		c.addEdge(mrn.New("Table", target.Provider, name), queryMRN, "FEEDS")
	}
}

// collectDashboards catalogues dashboards and the charts on them. The list
// endpoint leaves widgets out, so each dashboard needs a detail request.
func (c *collector) collectDashboards(ctx context.Context) error {
	dashboards, err := c.client.ListDashboards(ctx)
	if err != nil {
		return fmt.Errorf("listing dashboards: %w", err)
	}

	for _, dashboard := range dashboards {
		if !c.included(dashboard.IsArchived, dashboard.IsDraft) {
			log.Debug().Int("dashboard_id", dashboard.ID).Str("name", dashboard.Name).Msg("Skipping dashboard")
			continue
		}

		detail, err := c.fetchDashboard(ctx, dashboard)
		if err != nil {
			log.Warn().Err(err).Int("dashboard_id", dashboard.ID).Str("name", dashboard.Name).Msg("Failed to read dashboard widgets")
			detail = &dashboard
		}

		c.addDashboard(*detail)
	}

	log.Debug().Int("count", len(dashboards)).Msg("Discovered dashboards")
	return nil
}

// fetchDashboard reads one dashboard's detail. Redash 10 and newer key the
// endpoint on the numeric id and older releases key it on the slug, so the
// version picks the first attempt and the other key is the fallback.
func (c *collector) fetchDashboard(ctx context.Context, dashboard Dashboard) (*Dashboard, error) {
	keys := []string{strconv.Itoa(dashboard.ID), dashboard.Slug}
	if !c.version.atLeast(10) {
		keys = []string{dashboard.Slug, strconv.Itoa(dashboard.ID)}
	}

	var firstErr error
	for _, key := range keys {
		if key == "" {
			continue
		}
		detail, err := c.client.GetDashboard(ctx, key)
		if err == nil {
			return detail, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	return nil, firstErr
}

func (c *collector) addDashboard(dashboard Dashboard) {
	var notes []string
	widgetCount := 0
	queryIDs := make(map[int]struct{})

	for _, widget := range dashboard.Widgets {
		widgetCount++
		if widget.Visualization == nil {
			// A text widget carries prose, not a chart. The OpenMetadata
			// connector fails on this shape; here the text becomes a note
			// on the dashboard.
			if text := strings.TrimSpace(widget.Text); text != "" {
				notes = append(notes, text)
			}
			continue
		}
		if widget.Visualization.Query != nil {
			queryIDs[widget.Visualization.Query.ID] = struct{}{}
		}
	}

	metadata := map[string]any{
		"id":           dashboard.ID,
		"is_draft":     dashboard.IsDraft,
		"is_archived":  dashboard.IsArchived,
		"url":          c.dashboardURL(dashboard),
		"widget_count": widgetCount,
		"query_count":  len(queryIDs),
	}
	setIf(metadata, "slug", dashboard.Slug)
	setIf(metadata, "tags", dashboard.Tags)
	setIf(metadata, "created_at", dashboard.CreatedAt)
	setIf(metadata, "updated_at", dashboard.UpdatedAt)
	setIf(metadata, "redash_version", c.version.raw)
	if dashboard.User != nil {
		setIf(metadata, "owner", dashboard.User.Name)
	}
	// Redash dashboards have no description field. The text widgets are the
	// closest thing to one, so they are recorded as notes rather than
	// silently promoted to the description.
	setIf(metadata, "notes", strings.Join(notes, "\n\n"))

	name := c.takeName("Dashboard", dashboard.Name, dashboard.ID)
	mrnValue := assetMRN("Dashboard", name)

	c.assets = append(c.assets, pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Dashboard",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      c.tagsFor(dashboard.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{
			Name: "Open in Redash",
			URL:  metadata["url"].(string),
		}},
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	})

	for _, widget := range dashboard.Widgets {
		if widget.Visualization == nil {
			continue
		}
		c.addChart(name, mrnValue, widget)
	}
}

func (c *collector) addChart(dashboardName, dashboardMRN string, widget Widget) {
	visualization := widget.Visualization

	metadata := map[string]any{
		"widget_id":          widget.ID,
		"visualization_id":   visualization.ID,
		"visualization_type": visualization.Type,
		"chart_type":         chartType(visualization.Type, visualization.Options),
		"is_hidden":          widget.Options.IsHidden,
		"dashboard":          dashboardName,
	}

	var query *Query
	if visualization.Query != nil {
		query = visualization.Query
		metadata["query_id"] = query.ID
		setIf(metadata, "query_name", query.Name)
		metadata["url"] = fmt.Sprintf("%s/queries/%d#%d", c.config.Host, query.ID, visualization.ID)
		if source, ok := c.sources[query.DataSourceID]; ok {
			setIf(metadata, "data_source", source.Name)
		}
	}

	name := c.takeName("Chart", dashboardName+"/"+visualizationName(widget), widget.ID)
	mrnValue := assetMRN("Chart", name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Chart",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      c.tagsFor(nil, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if visualization.Description != "" {
		asset.Description = &visualization.Description
	}
	if url, ok := metadata["url"].(string); ok {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in Redash", URL: url}}
	}

	c.assets = append(c.assets, asset)

	if !c.config.DiscoverLineage {
		return
	}
	c.addEdge(dashboardMRN, mrnValue, "CONTAINS")
	if query != nil {
		if queryMRN, ok := c.queries[query.ID]; ok {
			c.addEdge(queryMRN, mrnValue, "FEEDS")
		}
	}
}

// visualizationName picks the display name for a chart. A Redash
// visualization is normally named after its query plus its type; the
// fallbacks cover the ones saved without a name.
func visualizationName(widget Widget) string {
	if name := strings.TrimSpace(widget.Visualization.Name); name != "" {
		return name
	}
	if widget.Visualization.Query != nil {
		if name := strings.TrimSpace(widget.Visualization.Query.Name); name != "" {
			return name
		}
	}
	return fmt.Sprintf("widget-%d", widget.ID)
}

// dashboardURL builds the link into the Redash UI. Redash 10 renamed the
// dashboard route from the slug to an id-slug pair, so the URL depends on
// the release. An undetected version takes the current shape.
func (c *collector) dashboardURL(dashboard Dashboard) string {
	if !c.version.atLeast(10) {
		return fmt.Sprintf("%s/dashboards/%s", c.config.Host, dashboard.Slug)
	}
	return fmt.Sprintf("%s/dashboards/%d-%s", c.config.Host, dashboard.ID, dashboard.Slug)
}

// included applies the archived and draft toggles.
func (c *collector) included(isArchived, isDraft bool) bool {
	if isArchived && !c.config.IncludeArchived {
		return false
	}
	if isDraft && !c.config.IncludeDrafts {
		return false
	}
	return true
}

// takeName returns a name unique within this run for the given asset type.
// Redash allows duplicate dashboard and query names but Marmot's identity is
// the name, so a repeat gets its Redash id appended. The comparison is
// case-insensitive because MRNs are lowercased.
func (c *collector) takeName(assetType, name string, id int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("%s-%d", strings.ToLower(assetType), id)
	}

	key := assetType + "\x00" + strings.ToLower(name)
	if _, taken := c.names[key]; !taken {
		c.names[key] = struct{}{}
		return name
	}

	qualified := fmt.Sprintf("%s (%d)", name, id)
	c.names[assetType+"\x00"+strings.ToLower(qualified)] = struct{}{}
	log.Debug().Str("type", assetType).Str("name", name).Str("qualified", qualified).Msg("Name already used in this run, qualifying it with the Redash id")
	return qualified
}

func (c *collector) addEdge(source, target, edgeType string) {
	c.lineage = append(c.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// tagsFor combines the tags Redash carries on an object with the tags
// configured for the run.
func (c *collector) tagsFor(sourceTags []string, metadata map[string]any) []string {
	tags := append([]string{}, sourceTags...)
	return append(tags, pluginsdk.InterpolateTags(c.config.Tags, metadata)...)
}

// assetMRN is the single place a Redash MRN is built, so the asset pass and
// the lineage pass can never address the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// chartType translates a Redash visualization into Marmot's chart
// vocabulary. For a CHART the visible shape lives in options.globalSeriesType
// rather than in the type, so a column chart and a line chart are told apart
// instead of both landing on Other.
func chartType(visualizationType string, options map[string]any) string {
	switch strings.ToLower(strings.TrimSpace(visualizationType)) {
	case "table", "details", "pivot":
		return "Table"
	case "counter":
		return "Text"
	case "map", "choropleth":
		return "Map"
	case "boxplot":
		return "BoxPlot"
	case "sankey":
		return "SanKey"
	case "chart":
		return seriesChartType(options)
	default:
		return "Other"
	}
}

func seriesChartType(options map[string]any) string {
	series, _ := options["globalSeriesType"].(string)
	switch strings.ToLower(strings.TrimSpace(series)) {
	case "line":
		return "Line"
	case "column", "bar":
		return "Bar"
	case "pie":
		return "Pie"
	case "area":
		return "Area"
	case "scatter":
		return "Scatter"
	case "box", "boxplot":
		return "BoxPlot"
	default:
		return "Other"
	}
}

// queryLanguage turns a Redash data source syntax into the language label
// Marmot shows next to a query.
func queryLanguage(syntax string) string {
	syntax = strings.TrimSpace(syntax)
	if syntax == "" {
		return ""
	}
	if strings.EqualFold(syntax, "sql") {
		return "SQL"
	}
	return strings.ToUpper(syntax)
}

// connectionOptionKeys is the allowlist of data source connection options
// published as metadata, mapped to the key each lands on. Keys that differ
// only in spelling between Redash runners are normalised onto one name.
var connectionOptionKeys = map[string]string{
	"host":           "host",
	"server":         "host",
	"url":            "url",
	"port":           "port",
	"dbname":         "dbname",
	"db":             "dbname",
	"database":       "database",
	"catalog":        "catalog",
	"schema":         "schema",
	"user":           "user",
	"username":       "user",
	"account":        "account",
	"warehouse":      "warehouse",
	"projectId":      "project",
	"dataset":        "dataset",
	"region":         "region",
	"servicename":    "service_name",
	"dbpath":         "dbpath",
	"http_path":      "http_path",
	"protocol":       "protocol",
	"sslmode":        "ssl_mode",
	"s3_staging_dir": "s3_staging_dir",
}

func scheduleMetadata(schedule *Schedule) map[string]any {
	if schedule == nil {
		return nil
	}
	out := map[string]any{}
	if schedule.Interval > 0 {
		out["interval"] = schedule.Interval
	}
	setIf(out, "time", schedule.Time)
	setIf(out, "day_of_week", schedule.DayOfWeek)
	setIf(out, "until", schedule.Until)
	return out
}

func parameterMetadata(parameters []QueryParameter) []map[string]any {
	var out []map[string]any
	for _, parameter := range parameters {
		entry := map[string]any{}
		setIf(entry, "name", parameter.Name)
		setIf(entry, "title", parameter.Title)
		setIf(entry, "type", parameter.Type)
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}
	return out
}

// setIf stores a value only when it carries information, keeping empty
// strings, zero ids and empty lists out of the metadata map.
func setIf(metadata map[string]any, key string, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		if strings.TrimSpace(v) == "" {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	case int:
		if v == 0 {
			return
		}
	case float64:
		if v == 0 {
			return
		}
	}
	metadata[key] = value
}

// version is a Redash release number. Only the major part matters here.
type version struct {
	raw   string
	major int
}

// parseVersion reads the leading major number out of a Redash version
// string such as "25.1.0".
func parseVersion(raw string) version {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return version{}
	}

	digits := raw
	if index := strings.IndexFunc(raw, func(r rune) bool { return r < '0' || r > '9' }); index >= 0 {
		digits = raw[:index]
	}

	major, err := strconv.Atoi(digits)
	if err != nil {
		return version{raw: raw}
	}
	return version{raw: raw, major: major}
}

// atLeast reports whether the detected version is at least major. An
// undetected version counts as current, so a Redash that does not report its
// version gets today's behaviour rather than a decade-old fallback.
func (v version) atLeast(major int) bool {
	return v.major == 0 || v.major >= major
}
