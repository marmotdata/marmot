package mariadb

import (
	"context"
	"encoding/json"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
	"github.com/rs/zerolog/log"
)

// attachViewColumnLineage parses each view's definition and sets the
// column lineage on the VIEW_OF edges attachViewDefinitions already found.
// Those edges stay authoritative for which objects a view reads: a view
// whose SQL cannot be parsed keeps its table-level edges, and a parsed
// reference with no matching edge is dropped rather than added.
func attachViewColumnLineage(ctx context.Context, database string, assets []pluginsdk.Asset, edges []pluginsdk.LineageEdge) {
	edgeIdx := map[[2]string]*pluginsdk.LineageEdge{}
	for i := range edges {
		if edges[i].Type == "VIEW_OF" {
			edgeIdx[[2]string{edges[i].Source, edges[i].Target}] = &edges[i]
		}
	}
	if len(edgeIdx) == 0 {
		return
	}

	sources := buildExtractSources(assets, database)
	if len(sources) == 0 {
		return
	}

	var extractor sqllineage.Extractor

	for i := range assets {
		viewAsset := &assets[i]
		if viewAsset.Type != "View" || viewAsset.MRN == nil || viewAsset.Query == nil || *viewAsset.Query == "" {
			continue
		}
		// MariaDB has no schema level: the database is the MariaDB schema.
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             *viewAsset.Query,
			Dialect:         "mariadb",
			DefaultDatabase: database,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", *viewAsset.Name).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		for sourceMRN, colEdges := range result {
			edge, ok := edgeIdx[[2]string{sourceMRN, *viewAsset.MRN}]
			if !ok {
				continue
			}
			edge.ColumnLineage = colEdges
		}
	}
}

// buildExtractSources describes every discovered table and view to the
// parser, with the columns attachColumns stored in its schema.
func buildExtractSources(assets []pluginsdk.Asset, database string) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assets))
	for i := range assets {
		a := &assets[i]
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil || a.Name == nil {
			continue
		}
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: database,
			Table:    *a.Name,
			Columns:  decodeColumns(a.Schema["columns"]),
		})
	}
	return sources
}

// decodeColumns pulls (name → type) out of the JSON blob attachColumns
// stores in Asset.Schema["columns"].
func decodeColumns(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return out
	}
	for _, c := range arr {
		name, _ := c["column_name"].(string)
		typ, _ := c["data_type"].(string)
		if name != "" {
			out[name] = typ
		}
	}
	return out
}
