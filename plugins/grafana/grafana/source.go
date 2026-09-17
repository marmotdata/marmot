// Package grafana discovers dashboards, panels and data sources from
// Grafana, and links charts to the tables their SQL queries read.
package grafana

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// providerName is the provider string of every asset this plugin
// creates and the service component of their MRNs.
const providerName = "Grafana"

// Config for the Grafana plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host      string `json:"host" description:"Grafana URL, for example https://grafana.company.com" validate:"required,url"`
	APIKey    string `json:"api_key" label:"Service Account Token" description:"Token of a Grafana service account" sensitive:"true" validate:"required"`
	VerifySSL bool   `json:"verify_ssl" label:"Verify SSL" description:"Verify the TLS certificate of the Grafana host" default:"true"`

	IncludePanels      bool `json:"include_panels" description:"Discover dashboard panels as Chart assets" default:"true"`
	IncludeDatasources bool `json:"include_datasources" description:"Discover data sources as DataSource assets" default:"true"`
	DiscoverLineage    bool `json:"discover_lineage" description:"Link charts and dashboards to the tables their SQL queries read" default:"true"`
	PageSize           int  `json:"page_size" description:"Dashboards per search request" default:"100" validate:"min=1,max=5000"`
}

// Example configuration for the plugin
var _ = `
host: "https://grafana.company.com"
api_key: "glsa_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx_xxxxxxxx"
include_panels: true
include_datasources: true
discover_lineage: true
tags:
  - "grafana"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "grafana",
		Name:        "Grafana",
		Description: "Discover dashboards, panels and data sources from Grafana",
		Icon:        "grafana",
		Category:    "dashboard",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(GrafanaDashboardFields{}, "Dashboard",
				"The metadata fields the Grafana plugin emits for Dashboard assets."),
			pluginsdk.AssetSchemaOf(GrafanaChartFields{}, "Chart",
				"The metadata fields emitted for Chart assets, one per dashboard panel."),
			pluginsdk.AssetSchemaOf(GrafanaDatasourceFields{}, "Datasource",
				"The metadata fields emitted for DataSource assets. Credentials are never read."),
		},
	}
}

// Source represents the Grafana plugin.
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

	// Every API path is joined onto the host, so a copied-from-the-browser
	// trailing slash would produce //api/... which Grafana serves as HTML.
	config.Host = strings.TrimSuffix(strings.TrimSpace(config.Host), "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	// Grafana still accepts the API keys it deprecated in 9.1, so an
	// unfamiliar token is worth a warning but not a refusal.
	if !strings.HasPrefix(config.APIKey, "glsa_") {
		log.Warn().Msg("api_key does not look like a service account token (expected a glsa_ prefix)")
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Grafana dashboards, panels and data sources.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	d := &discovery{
		config:    s.config,
		client:    newClient(s.config.Host, s.config.APIKey, s.config.VerifySSL),
		library:   make(map[string]*panel),
		seenEdges: make(map[string]bool),
	}

	log.Debug().Str("host", s.config.Host).Msg("Starting Grafana discovery")

	// The dashboard listing is the first call, so a wrong host or token
	// fails the run here instead of producing an empty catalog.
	hits, err := d.client.searchDashboards(ctx, s.config.PageSize)
	if err != nil {
		return nil, fmt.Errorf("listing dashboards: %w", err)
	}
	log.Debug().Int("count", len(hits)).Msg("Found dashboards")

	// Data sources are read even when they are not catalogued: a chart
	// records the data source it queries, and SQL lineage needs the
	// source's type. A token that may not list them still yields
	// dashboards and charts, just without those details.
	sources, err := d.client.datasources(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list data sources")
	}
	d.index = indexDatasources(sources)

	if s.config.IncludeDatasources {
		for _, ds := range sources {
			d.assets = append(d.assets, d.datasourceAsset(ds))
		}
		log.Debug().Int("count", len(sources)).Msg("Discovered data sources")
	}

	dashboards := 0
	for _, hit := range hits {
		resp, err := d.client.dashboard(ctx, hit.UID)
		if err != nil {
			log.Warn().Err(err).Str("uid", hit.UID).Str("title", hit.Title).Msg("Failed to fetch dashboard")
			continue
		}
		d.discoverDashboard(ctx, resp)
		dashboards++
	}

	log.Info().
		Int("dashboards", dashboards).
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Msg("Grafana discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:  d.assets,
		Lineage: d.lineage,
	}, nil
}

// discovery accumulates the output of one Discover call.
type discovery struct {
	config *Config
	client *client
	index  *datasourceIndex

	// library caches shared panels by uid. A nil entry records that
	// the panel could not be read, so it is not requested again.
	library map[string]*panel

	assets    []pluginsdk.Asset
	lineage   []pluginsdk.LineageEdge
	seenEdges map[string]bool
}

// link records an edge once, however many panels produce it.
func (d *discovery) link(source, target, edgeType string) {
	key := source + "|" + target + "|" + edgeType
	if d.seenEdges[key] {
		return
	}
	d.seenEdges[key] = true
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// discoverDashboard creates the Dashboard asset, a Chart per panel, and
// the edges between them, their data sources and the tables they read.
func (d *discovery) discoverDashboard(ctx context.Context, resp *dashboardResponse) {
	dash := resp.Dashboard
	meta := resp.Meta

	name := dashboardName(resp)
	dashMRN := assetMRN("Dashboard", name)
	dashURL := d.config.Host + meta.URL

	panels := d.chartPanels(ctx, dash)

	version := dash.Version
	if version == 0 {
		version = meta.Version
	}

	metadata := map[string]any{
		"uid":         dash.UID,
		"url":         dashURL,
		"folder":      meta.FolderTitle,
		"folder_uid":  meta.FolderUID,
		"version":     version,
		"created":     meta.Created,
		"updated":     meta.Updated,
		"created_by":  meta.CreatedBy,
		"updated_by":  meta.UpdatedBy,
		"provisioned": meta.Provisioned,
		"refresh":     string(dash.Refresh),
		"time_from":   dash.Time.From,
		"time_to":     dash.Time.To,
		"panel_count": len(panels),
	}
	if dash.ID > 0 {
		metadata["id"] = dash.ID
	}
	if dash.SchemaVersion > 0 {
		metadata["schema_version"] = dash.SchemaVersion
	}
	if len(dash.Tags) > 0 {
		metadata["tags"] = dash.Tags
	}
	metadata = cleanMetadata(metadata)

	d.assets = append(d.assets, pluginsdk.Asset{
		Name:          &name,
		MRN:           &dashMRN,
		Type:          "Dashboard",
		Providers:     []string{providerName},
		Description:   optional(dash.Description),
		Metadata:      metadata,
		Tags:          mergeTags(pluginsdk.InterpolateTags(d.config.Tags, metadata), dash.Tags),
		ExternalLinks: []pluginsdk.AssetExternalLink{{Name: "Open in Grafana", URL: dashURL}},
		Sources:       d.sources(metadata),
	})

	log.Debug().Str("dashboard", name).Int("panels", len(panels)).Msg("Discovered dashboard")

	taken := make(map[string]bool)
	for _, p := range panels {
		queries := queriesOf(p, d.index)

		chartMRN := ""
		if d.config.IncludePanels {
			chart := d.chartAsset(resp, name, p, queries, taken)
			chartMRN = *chart.MRN
			d.assets = append(d.assets, chart)
			d.link(dashMRN, chartMRN, "CONTAINS")

			if d.config.IncludeDatasources {
				for _, ds := range distinctDatasources(p, queries, d.index) {
					d.link(assetMRN("DataSource", ds.Name), chartMRN, "FEEDS")
				}
			}
		}

		if !d.config.DiscoverLineage {
			continue
		}
		for _, q := range queries {
			database := ""
			if q.ds != nil {
				database = q.ds.databaseName()
			}
			for _, tableMRN := range sqlTableMRNs(q.datasourceType(), database, q.sql()) {
				if chartMRN != "" {
					d.link(tableMRN, chartMRN, "FEEDS")
				}
				d.link(tableMRN, dashMRN, "FEEDS")
			}
		}
	}
}

// chartPanels returns the panels of a dashboard that become charts,
// with rows flattened and library panels resolved.
func (d *discovery) chartPanels(ctx context.Context, dash dashboard) []panel {
	raw := dash.Panels
	for _, row := range dash.Rows {
		raw = append(raw, row.Panels...)
	}

	var out []panel
	for _, p := range flattenPanels(raw) {
		if p.LibraryPanel != nil && p.LibraryPanel.UID != "" {
			p = d.resolveLibraryPanel(ctx, p)
		}
		if !isChartPanel(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// resolveLibraryPanel replaces a library panel reference with the
// shared panel it stands for, keeping the id the dashboard gave it.
// The reference is kept as it is when the shared panel cannot be read.
func (d *discovery) resolveLibraryPanel(ctx context.Context, p panel) panel {
	uid := p.LibraryPanel.UID

	model, cached := d.library[uid]
	if !cached {
		var err error
		model, err = d.client.libraryPanel(ctx, uid)
		if err != nil {
			log.Warn().Err(err).Str("library_panel", uid).Msg("Failed to fetch library panel")
			model = nil
		}
		d.library[uid] = model
	}
	if model == nil {
		return p
	}

	resolved := *model
	resolved.ID = p.ID
	resolved.LibraryPanel = p.LibraryPanel
	return resolved
}

// chartAsset creates the Chart asset for one panel.
func (d *discovery) chartAsset(resp *dashboardResponse, dashboardName string, p panel, queries []panelQuery, taken map[string]bool) pluginsdk.Asset {
	name := dashboardName + "/" + panelName(p)
	chartMRN := assetMRN("Chart", name)

	// Two panels with the same title would land on one asset, so the
	// second and later ones carry their panel id.
	if taken[chartMRN] {
		name = fmt.Sprintf("%s/%s (panel %d)", dashboardName, panelName(p), p.ID)
		chartMRN = assetMRN("Chart", name)
	}
	taken[chartMRN] = true

	panelURL := fmt.Sprintf("%s%s?viewPanel=%d", d.config.Host, resp.Meta.URL, p.ID)

	metadata := map[string]any{
		"panel_id":      p.ID,
		"panel_type":    p.Type,
		"chart_type":    chartType(p.Type),
		"dashboard":     resp.Dashboard.Title,
		"dashboard_uid": resp.Dashboard.UID,
		"url":           panelURL,
		"target_count":  len(p.Targets),
	}
	if ds, ref := primaryDatasource(p, queries, d.index); ds != nil {
		metadata["datasource"] = ds.Name
		metadata["datasource_type"] = ds.Type
		metadata["datasource_uid"] = ds.UID
	} else if ref != nil {
		metadata["datasource_type"] = ref.Type
		metadata["datasource_uid"] = ref.UID
	}
	metadata = cleanMetadata(metadata)

	asset := pluginsdk.Asset{
		Name:          &name,
		MRN:           &chartMRN,
		Type:          "Chart",
		Providers:     []string{providerName},
		Description:   optional(p.Description),
		Metadata:      metadata,
		Tags:          pluginsdk.InterpolateTags(d.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{Name: "Open in Grafana", URL: panelURL}},
		Sources:       d.sources(metadata),
	}

	if query, language := queryOf(queries); query != "" && language != "" {
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	return asset
}

// datasourceAsset creates the DataSource asset for one Grafana data
// source. Nothing from secureJsonData is ever read.
func (d *discovery) datasourceAsset(ds datasource) pluginsdk.Asset {
	name := ds.Name
	dsMRN := assetMRN("DataSource", name)
	editURL := d.config.Host + "/connections/datasources/edit/" + url.PathEscape(ds.UID)

	metadata := cleanMetadata(map[string]any{
		"uid":        ds.UID,
		"id":         ds.ID,
		"type":       ds.Type,
		"type_name":  ds.TypeName,
		"url":        ds.URL,
		"database":   ds.databaseName(),
		"is_default": ds.IsDefault,
		"read_only":  ds.ReadOnly,
		"access":     ds.Access,
	})

	return pluginsdk.Asset{
		Name:          &name,
		MRN:           &dsMRN,
		Type:          "DataSource",
		Providers:     []string{providerName},
		Metadata:      metadata,
		Tags:          pluginsdk.InterpolateTags(d.config.Tags, metadata),
		ExternalLinks: []pluginsdk.AssetExternalLink{{Name: "Open in Grafana", URL: editURL}},
		Sources:       d.sources(metadata),
	}
}

func (d *discovery) sources(metadata map[string]any) []pluginsdk.AssetSource {
	return []pluginsdk.AssetSource{{
		Name:       providerName,
		LastSyncAt: time.Now(),
		Properties: metadata,
		Priority:   1,
	}}
}

// dashboardName is the name people read and the identity the MRN is
// built from. A dashboard in a folder is qualified by that folder,
// because Grafana only keeps titles unique within one folder. The
// General folder is Grafana's root and is left out.
func dashboardName(resp *dashboardResponse) string {
	meta := resp.Meta
	if (meta.FolderUID != "" || meta.FolderID > 0) && meta.FolderTitle != "" {
		return meta.FolderTitle + "/" + resp.Dashboard.Title
	}
	return resp.Dashboard.Title
}

// primaryDatasource is the data source recorded on a chart: the
// panel's own when it resolves, otherwise the first target's. The raw
// reference is returned alongside so a chart whose source could not be
// resolved still records what the dashboard named.
func primaryDatasource(p panel, queries []panelQuery, ix *datasourceIndex) (*datasource, *datasourceRef) {
	if ds := ix.resolve(p.Datasource); ds != nil {
		return ds, p.Datasource
	}
	for _, q := range queries {
		if q.ds != nil {
			return q.ds, q.ref
		}
	}
	return nil, p.Datasource
}

// distinctDatasources returns every catalogued data source a panel
// reads, once each: the panel's own plus any a target overrides it
// with.
func distinctDatasources(p panel, queries []panelQuery, ix *datasourceIndex) []*datasource {
	var out []*datasource
	seen := make(map[string]bool)
	add := func(ds *datasource) {
		if ds == nil || seen[ds.UID] {
			return
		}
		seen[ds.UID] = true
		out = append(out, ds)
	}

	add(ix.resolve(p.Datasource))
	for _, q := range queries {
		add(q.ds)
	}
	return out
}

// assetMRN is the single place a Grafana MRN is built. Every asset and
// every edge goes through it so the two can never drift into
// addressing the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, providerName, name)
}

// optional returns a pointer to a non-empty string and nil otherwise,
// so an empty description is omitted rather than recorded as "".
func optional(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// mergeTags appends extra tags to base, skipping any already present.
func mergeTags(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	out := make([]string, 0, len(base)+len(extra))
	for _, tag := range base {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	for _, tag := range extra {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out
}

// cleanMetadata removes empty values so metadata only carries what
// Grafana actually set.
func cleanMetadata(metadata map[string]any) map[string]any {
	cleaned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		switch value := v.(type) {
		case nil:
			continue
		case string:
			if value == "" {
				continue
			}
		case []string:
			if len(value) == 0 {
				continue
			}
		}
		cleaned[k] = v
	}
	return cleaned
}
