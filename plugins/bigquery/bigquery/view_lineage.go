package bigquery

import (
	"context"
	"encoding/json"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/bigquery"
	"github.com/rs/zerolog/log"
)

// discoverViewLineage parses the query behind each view and materialized
// view and returns a DEPENDS_ON edge, with column lineage, from every
// discovered table it reads. It runs over the whole project's assets so a
// view reading another dataset still resolves; tables outside the project
// are dropped by the extractor.
func (s *Source) discoverViewLineage(ctx context.Context, assets []pluginsdk.Asset) []pluginsdk.LineageEdge {
	assetIdx := map[datasetTable]*pluginsdk.Asset{}
	for i := range assets {
		a := &assets[i]
		if a.Type != "Table" && a.Type != "View" && a.Type != "ExternalTable" {
			continue
		}
		dt, ok := datasetTableOf(a)
		if !ok || a.MRN == nil {
			continue
		}
		assetIdx[dt] = a
	}

	sources := buildExtractSources(assetIdx, s.config.ProjectID)
	if len(sources) == 0 {
		return nil
	}

	var extractor sqllineage.Extractor
	var edges []pluginsdk.LineageEdge

	for dt, viewAsset := range assetIdx {
		sql := viewQueryOf(viewAsset)
		if sql == "" {
			continue
		}
		// Legacy SQL ([project:dataset.table]) is not GoogleSQL and the
		// parser cannot read it.
		if legacy, _ := viewAsset.Metadata["use_legacy_sql"].(bool); legacy {
			continue
		}
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             sql,
			Dialect:         "bigquery",
			DefaultDatabase: s.config.ProjectID,
			DefaultSchema:   dt.dataset,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", dt.String()).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		viewCols := columnNameIndex(viewAsset)
		for sourceMRN, colEdges := range result {
			if sourceMRN == *viewAsset.MRN {
				continue
			}
			for i := range colEdges {
				if name, ok := viewCols[strings.ToLower(colEdges[i].ToColumn)]; ok {
					colEdges[i].ToColumn = name
				}
			}
			edges = append(edges, pluginsdk.LineageEdge{
				Source:        sourceMRN,
				Target:        *viewAsset.MRN,
				Type:          "DEPENDS_ON",
				ColumnLineage: colEdges,
			})
		}
	}

	return edges
}

// columnNameIndex maps a view's lower-cased column names to their spelling
// in its schema: the GoogleSQL parser upper-cases result names, which would
// otherwise not match the view's own columns.
func columnNameIndex(a *pluginsdk.Asset) map[string]string {
	cols := decodeColumns(a.Schema["json_schema"])
	out := make(map[string]string, len(cols))
	for name := range cols {
		out[strings.ToLower(name)] = name
	}
	return out
}

type datasetTable struct{ dataset, table string }

func (d datasetTable) String() string { return d.dataset + "." + d.table }

func datasetTableOf(a *pluginsdk.Asset) (datasetTable, bool) {
	dataset, ok := a.Metadata["dataset_id"].(string)
	if !ok {
		return datasetTable{}, false
	}
	table, ok := a.Metadata["table_id"].(string)
	if !ok {
		return datasetTable{}, false
	}
	return datasetTable{dataset: dataset, table: table}, true
}

// viewQueryOf returns the SQL a view or materialized view is defined by;
// materialized views are discovered as tables, so their query sits under
// its own key.
func viewQueryOf(a *pluginsdk.Asset) string {
	if q, ok := a.Metadata["view_query"].(string); ok && q != "" {
		return q
	}
	q, _ := a.Metadata["materialized_view_query"].(string)
	return q
}

func buildExtractSources(assetIdx map[datasetTable]*pluginsdk.Asset, projectID string) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assetIdx))
	for dt, a := range assetIdx {
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: projectID,
			Schema:   dt.dataset,
			Table:    dt.table,
			Columns:  decodeColumns(a.Schema["json_schema"]),
		})
	}
	return sources
}

// decodeColumns pulls (name → type) for the top-level fields out of the JSON
// Schema the plugin stores in Asset.Schema["json_schema"] (see
// generateJSONSchema). Nested RECORD fields are not addressed separately.
func decodeColumns(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return out
	}
	for name, p := range schema.Properties {
		out[name] = p.Type
	}
	return out
}
