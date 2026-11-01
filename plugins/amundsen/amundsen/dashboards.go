package amundsen

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"context"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// A dashboard sits under a Dashboardgroup under a Cluster, and its
// charts hang off the queries that feed them.
//
// The last successful execution is matched by the suffix of its key
// rather than by the position of a slash in it. Amundsen's own search
// query, and the OpenMetadata connector that copies it, take the fifth
// segment of the key, but a real execution key
// ("superset_dashboard://prod.finance/revenue/_last_successful_execution")
// only has five segments, so the fifth is always null and the filter
// never matches.
//
// Charts are collected as one map per chart for the same reason columns
// are: three parallel lists silently pair the wrong name with the wrong
// id as soon as one chart is missing a field. Read counts are left to
// dashboardUsageQuery so the OPTIONAL MATCH fan-out cannot multiply them.
const dashboardQuery = `
MATCH (dashboard:Dashboard)-[:DASHBOARD_OF]->(dbg:Dashboardgroup)-[:DASHBOARD_GROUP_OF]->(cluster:Cluster)
OPTIONAL MATCH (dashboard)-[:DESCRIPTION]->(db_descr:Description)
OPTIONAL MATCH (dbg)-[:DESCRIPTION]->(dbg_descr:Description)
OPTIONAL MATCH (dashboard)-[:EXECUTED]->(last_exec:Execution) WHERE last_exec.key ENDS WITH '_last_successful_execution'
OPTIONAL MATCH (dashboard)-[:HAS_QUERY]->(query:Query)
OPTIONAL MATCH (query)-[:HAS_CHART]->(chart:Chart)
OPTIONAL MATCH (dashboard)-[:TAGGED_BY]->(tag:Tag) WHERE tag.tag_type = 'default'
OPTIONAL MATCH (dashboard)-[:HAS_BADGE]->(badge:Badge)
RETURN dbg.name AS group_name, dashboard.name AS name, cluster.name AS cluster,
       db_descr.description AS description, dbg_descr.description AS group_description,
       dbg.url AS group_url, dashboard.dashboard_url AS url, dashboard.key AS key,
       split(dashboard.key, '_')[0] AS product,
       toInteger(last_exec.timestamp) AS last_successful_run_timestamp,
       COLLECT(DISTINCT query.name) AS query_names,
       COLLECT(DISTINCT {name: chart.name, id: chart.id, url: chart.url, type: chart.type}) AS charts,
       COLLECT(DISTINCT tag.key) AS tags,
       COLLECT(DISTINCT badge.key) AS badges
ORDER BY key
SKIP $skip LIMIT $limit
`

const dashboardUsageQuery = `
MATCH (dashboard:Dashboard)-[read:READ_BY]->(user:User)
RETURN dashboard.key AS key, SUM(read.read_count) AS total_usage, COUNT(DISTINCT user.email) AS unique_usage
ORDER BY key
SKIP $skip LIMIT $limit
`

// Amundsen writes the dashboard to table relationship in both
// directions and different loaders write different ones, so the match is
// undirected over both types and the duplicates are dropped afterwards.
const dashboardTableQuery = `
MATCH (dashboard:Dashboard)-[:DASHBOARD_WITH_TABLE|TABLE_OF_DASHBOARD]-(table:Table)
RETURN DISTINCT dashboard.key AS dashboard_key, table.key AS table_key
ORDER BY dashboard_key, table_key
SKIP $skip LIMIT $limit
`

// dashboardRow is one row of dashboardQuery.
type dashboardRow struct {
	GroupName        string
	Name             string
	Cluster          string
	Key              string
	Product          string
	Description      string
	GroupDescription string
	GroupURL         string
	URL              string
	LastRun          int64
	QueryNames       []string
	Charts           []chartRow
	Tags             []string
	Badges           []string
}

// chartRow is one Chart node hanging off a dashboard's queries.
type chartRow struct {
	Name string
	ID   string
	URL  string
	Type string
}

func (c *collector) discoverDashboards(ctx context.Context, r reader) error {
	usage, err := c.readUsage(ctx, r, "dashboard usage", dashboardUsageQuery)
	if err != nil {
		return err
	}

	dashboards, charts := 0, 0
	err = eachPage(ctx, r, "dashboards", dashboardQuery, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			dashboard := decodeDashboardRow(row)
			if dashboard.Name == "" {
				log.Warn().Str("key", dashboard.Key).Msg("Skipping an Amundsen dashboard with no name")
				continue
			}
			added, chartCount := c.addDashboard(dashboard, usage[dashboard.Key])
			if added {
				dashboards++
				charts += chartCount
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Int("dashboards", dashboards).Int("charts", charts).Msg("Discovered Amundsen dashboards")
	return nil
}

func (c *collector) addDashboard(dashboard dashboardRow, use usage) (bool, int) {
	provider := dashboardProviderFor(dashboard.Product)
	name := dashboardName(dashboard.GroupName, dashboard.Name)

	provenance := map[string]any{}
	putIf(provenance, "key", dashboard.Key)
	putIf(provenance, "cluster", dashboard.Cluster)
	putIf(provenance, "group", dashboard.GroupName)
	putIf(provenance, "group_url", dashboard.GroupURL)
	putIf(provenance, "product", dashboard.Product)
	putIf(provenance, "url", dashboard.URL)
	putIf(provenance, "query_names", dashboard.QueryNames)
	putIf(provenance, "tags", dashboard.Tags)
	putIf(provenance, "badges", dashboard.Badges)
	if c.config.IncludeDescriptions {
		putIf(provenance, "group_description", dashboard.GroupDescription)
	}
	if dashboard.LastRun > 0 {
		provenance["last_successful_run"] = time.Unix(dashboard.LastRun, 0).UTC().Format(time.RFC3339)
	}
	if link := c.dashboardURL(dashboard.Key); link != "" {
		provenance["amundsen_url"] = link
	}

	metadata := map[string]any{"amundsen": provenance}
	putIf(metadata, "chart_count", int64(len(dashboard.Charts)))

	asset := c.newAsset("Dashboard", provider, name, dashboard.Description, c.assetTags(dashboard.Tags), metadata)
	asset.ExternalLinks = append(c.dashboardLinks(dashboard, provider), asset.ExternalLinks...)

	if !c.add(dashboard.Key, asset) {
		return false, 0
	}

	c.stat(*asset.MRN, metricReadCount, use.Total)
	c.stat(*asset.MRN, metricUniqueReaders, use.Unique)

	charts := 0
	for _, chart := range dashboard.Charts {
		if c.addChart(dashboard, chart, provider, *asset.MRN) {
			charts++
		}
	}

	return true, charts
}

func (c *collector) addChart(dashboard dashboardRow, chart chartRow, provider, dashboardMRN string) bool {
	label := chart.Name
	if label == "" {
		// Amundsen stores an id for every chart but a name only when the
		// BI tool gave the chart one.
		label = chart.ID
	}
	if label == "" {
		log.Debug().Str("dashboard", dashboard.Key).Msg("Skipping a chart with neither a name nor an id")
		return false
	}

	name := dashboardName(dashboard.GroupName, dashboard.Name) + "/" + label

	provenance := map[string]any{}
	putIf(provenance, "dashboard_key", dashboard.Key)
	putIf(provenance, "group", dashboard.GroupName)
	putIf(provenance, "dashboard", dashboard.Name)
	putIf(provenance, "chart_id", chart.ID)
	putIf(provenance, "url", chart.URL)
	putIf(provenance, "product", dashboard.Product)

	metadata := map[string]any{"amundsen": provenance}
	putIf(metadata, "chart_type", chart.Type)

	asset := c.newAsset("Chart", provider, name, "", c.assetTags(dashboard.Tags), metadata)
	if chart.URL != "" {
		asset.ExternalLinks = append([]pluginsdk.AssetExternalLink{{
			Name: "Open in " + provider,
			Icon: "mdi:open-in-new",
			URL:  chart.URL,
		}}, asset.ExternalLinks...)
	}

	// A chart key would be the natural identifier, but the dashboard
	// query collects charts rather than returning them one per row, so
	// only the id is available and nothing resolves lineage by it.
	if !c.add("", asset) {
		return false
	}

	c.link(dashboardMRN, *asset.MRN, "CONTAINS")
	return true
}

func (c *collector) discoverDashboardLineage(ctx context.Context, r reader) error {
	edges := 0
	err := eachPage(ctx, r, "dashboard lineage", dashboardTableQuery, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			dashboardMRN, ok := c.mrnByKey[textOf(row, "dashboard_key")]
			if !ok {
				continue
			}
			tableMRN := c.tableMRN(textOf(row, "table_key"))
			if tableMRN == "" {
				continue
			}
			c.link(tableMRN, dashboardMRN, "FEEDS")
			edges++
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Int("edges", edges).Msg("Discovered Amundsen dashboard lineage")
	return nil
}

// dashboardLinks are the links on a dashboard asset: the dashboard in
// the tool that owns it, then its page in Amundsen.
func (c *collector) dashboardLinks(dashboard dashboardRow, provider string) []pluginsdk.AssetExternalLink {
	var links []pluginsdk.AssetExternalLink

	if dashboard.URL != "" {
		links = append(links, pluginsdk.AssetExternalLink{
			Name: "Open in " + provider,
			Icon: "mdi:open-in-new",
			URL:  dashboard.URL,
		})
	}
	if link := c.dashboardURL(dashboard.Key); link != "" {
		links = append(links, pluginsdk.AssetExternalLink{
			Name: "Open in Amundsen",
			Icon: "mdi:open-in-new",
			URL:  link,
		})
	}

	return links
}

// dashboardURL is the dashboard's page in the Amundsen web app, which
// routes a dashboard by its whole key.
func (c *collector) dashboardURL(key string) string {
	if c.config.AmundsenURL == "" || key == "" {
		return ""
	}
	return fmt.Sprintf("%s/dashboard/%s", c.config.AmundsenURL, url.PathEscape(key))
}

// dashboardName qualifies a dashboard by the group holding it, because
// two groups in one BI tool routinely hold a dashboard of the same name.
func dashboardName(group, name string) string {
	if group == "" {
		return name
	}
	return group + "/" + name
}

func decodeDashboardRow(row map[string]any) dashboardRow {
	dashboard := dashboardRow{
		GroupName:        textOf(row, "group_name"),
		Name:             textOf(row, "name"),
		Cluster:          textOf(row, "cluster"),
		Key:              textOf(row, "key"),
		Product:          dashboardProduct(row),
		Description:      textOf(row, "description"),
		GroupDescription: textOf(row, "group_description"),
		GroupURL:         textOf(row, "group_url"),
		URL:              textOf(row, "url"),
		QueryNames:       textsOf(row, "query_names"),
		Tags:             textsOf(row, "tags"),
		Badges:           textsOf(row, "badges"),
	}

	if lastRun, ok := numberOf(row, "last_successful_run_timestamp"); ok {
		dashboard.LastRun = int64(lastRun)
	}

	for _, chart := range mapsOf(row, "charts") {
		name, id := textOf(chart, "name"), textOf(chart, "id")
		if name == "" && id == "" {
			// A dashboard with no charts still collects one entry, made
			// entirely of the nulls the OPTIONAL MATCH did not fill.
			continue
		}
		dashboard.Charts = append(dashboard.Charts, chartRow{
			Name: name,
			ID:   id,
			URL:  textOf(chart, "url"),
			Type: textOf(chart, "type"),
		})
	}

	return dashboard
}

// dashboardProduct is the BI tool a dashboard came from. Amundsen writes
// it into the scheme of the key ("superset_dashboard://..."), which the
// query already splits out, but a graph loaded without it falls back to
// reading the key here.
func dashboardProduct(row map[string]any) string {
	if product := textOf(row, "product"); product != "" {
		return product
	}

	scheme, _, ok := strings.Cut(textOf(row, "key"), "://")
	if !ok {
		return ""
	}
	product, _, _ := strings.Cut(scheme, "_")
	return product
}
