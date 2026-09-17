package grafana

import (
	"fmt"
	"strings"
)

// flattenPanels returns a dashboard's panels in display order with the
// row layout removed. An expanded row's panels are already its
// siblings; a collapsed row carries its panels inside it, so those are
// lifted out. The row panel itself is dropped: it is layout, not a
// chart.
func flattenPanels(panels []panel) []panel {
	var out []panel
	for _, p := range panels {
		if p.Type == "row" {
			out = append(out, flattenPanels(p.Panels)...)
			continue
		}
		out = append(out, p)
	}
	return out
}

// isChartPanel reports whether a panel shows data. Text panels hold
// prose and a panel without a type is a library reference that could
// not be resolved.
func isChartPanel(p panel) bool {
	return p.Type != "" && p.Type != "text" && p.Type != "row"
}

// chartType normalises a Grafana panel type to the chart vocabulary the
// other dashboard plugins use.
func chartType(panelType string) string {
	switch panelType {
	case "graph", "timeseries":
		return "Line"
	case "table", "alertlist", "logs":
		return "Table"
	case "stat", "text", "news":
		return "Text"
	case "gauge":
		return "Gauge"
	case "bargauge", "barchart", "bar":
		return "Bar"
	case "piechart":
		return "Pie"
	case "heatmap":
		return "Heatmap"
	case "histogram":
		return "Histogram"
	case "geomap":
		return "Map"
	case "nodeGraph":
		return "Graph"
	case "state-timeline", "status-history":
		return "Timeline"
	default:
		return "Other"
	}
}

// panelName is the part of a chart's name that identifies the panel
// within its dashboard. Grafana allows untitled panels, so those fall
// back to the panel id, which is unique within a dashboard.
func panelName(p panel) string {
	if title := strings.TrimSpace(p.Title); title != "" {
		return title
	}
	return fmt.Sprintf("panel-%d", p.ID)
}

// datasourceIndex resolves the references panels make to data sources.
type datasourceIndex struct {
	byUID  map[string]*datasource
	byName map[string]*datasource
	def    *datasource
}

func indexDatasources(list []datasource) *datasourceIndex {
	ix := &datasourceIndex{
		byUID:  make(map[string]*datasource, len(list)),
		byName: make(map[string]*datasource, len(list)),
	}
	for i := range list {
		ds := &list[i]
		ix.byUID[ds.UID] = ds
		ix.byName[ds.Name] = ds
		if ds.IsDefault {
			ix.def = ds
		}
	}
	return ix
}

// resolve returns the data source a reference points at, or nil when
// it points at nothing catalogued: Grafana's built-in pseudo sources,
// a template variable, or a uid that no longer exists. A missing
// reference means the default data source.
func (ix *datasourceIndex) resolve(ref *datasourceRef) *datasource {
	if ref == nil || (ref.UID == "" && ref.Type == "") {
		return ix.def
	}
	if isPseudoDatasource(ref) {
		return nil
	}
	if ds, ok := ix.byUID[ref.UID]; ok {
		return ds
	}
	if ds, ok := ix.byName[ref.UID]; ok {
		return ds
	}
	return nil
}

// isPseudoDatasource reports whether a reference names one of the
// sources Grafana provides itself rather than a configured one: the
// built-in Grafana source, the mixed source that defers to each
// target, the dashboard source that reuses another panel's result, and
// server-side expressions.
func isPseudoDatasource(ref *datasourceRef) bool {
	switch ref.UID {
	case "grafana", "-- Grafana --", "-- Mixed --", "-- Dashboard --", "__expr__":
		return true
	}
	return ref.Type == "__expr__"
}

// panelQuery is one target of a panel together with the data source it
// runs against. ref is what the dashboard wrote; ds is what it resolves
// to, nil when it resolves to nothing.
type panelQuery struct {
	target target
	ref    *datasourceRef
	ds     *datasource
}

// queriesOf pairs every target of a panel with its data source. A
// target's own reference wins over the panel's, which is how mixed
// panels work.
func queriesOf(p panel, ix *datasourceIndex) []panelQuery {
	queries := make([]panelQuery, 0, len(p.Targets))
	for _, t := range p.Targets {
		ref := p.Datasource
		if t.Datasource != nil && t.Datasource.UID != "" {
			ref = t.Datasource
		}
		queries = append(queries, panelQuery{target: t, ref: ref, ds: ix.resolve(ref)})
	}
	return queries
}

// datasourceType is the type of the data source a query runs against.
// The reference carries the type from Grafana 8.3 on, so a query can
// be classified even when the data source could not be listed.
func (q panelQuery) datasourceType() string {
	if q.ds != nil {
		return q.ds.Type
	}
	if q.ref != nil {
		return q.ref.Type
	}
	return ""
}

// sql returns the SQL a query runs, or "" when it is not a SQL query.
// rawSql is only ever written by SQL data source plugins. query is
// used by several non-SQL plugins too, so it only counts for the data
// source types known to put SQL there.
func (q panelQuery) sql() string {
	if s := strings.TrimSpace(string(q.target.RawSQL)); s != "" {
		return s
	}
	if _, ok := datasourceMap[q.datasourceType()]; ok {
		return strings.TrimSpace(string(q.target.Query))
	}
	return ""
}

// queryOf returns the query text and language to record on a chart.
// Only a panel whose targets all speak one language gets one: the
// first SQL query when every target is SQL, the first PromQL or LogQL
// expression when every target is an expression.
func queryOf(queries []panelQuery) (query, language string) {
	if len(queries) == 0 {
		return "", ""
	}

	allSQL, allExpr := true, true
	for _, q := range queries {
		if q.sql() == "" {
			allSQL = false
		}
		if strings.TrimSpace(string(q.target.Expr)) == "" {
			allExpr = false
		}
	}

	switch {
	case allSQL:
		return queries[0].sql(), "SQL"
	case allExpr:
		return strings.TrimSpace(string(queries[0].target.Expr)), exprLanguage(queries[0].datasourceType())
	default:
		return "", ""
	}
}

// exprLanguage names the language of an expr target from its data
// source type. Loki and Prometheus both use expr; the type is the only
// thing that tells them apart.
func exprLanguage(datasourceType string) string {
	lower := strings.ToLower(datasourceType)
	switch {
	case strings.Contains(lower, "loki"):
		return "LogQL"
	case strings.Contains(lower, "prometheus"):
		return "PromQL"
	default:
		return ""
	}
}
