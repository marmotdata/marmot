package presto

import (
	"context"
	"encoding/json"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	"github.com/rs/zerolog/log"
)

// attachColumnLineage parses each view's SQL and hangs the column edges
// it derives on the matching VIEW_OF edge. A base table the parser finds
// but the reference scan missed gets an edge of its own. A view the
// parser cannot handle keeps its table-level edges unchanged.
func attachColumnLineage(ctx context.Context, assets []pluginsdk.Asset, edges []pluginsdk.LineageEdge) []pluginsdk.LineageEdge {
	sources := buildExtractSources(assets)
	if len(sources) == 0 {
		return edges
	}

	edgeIdx := make(map[string]int, len(edges))
	for i, e := range edges {
		edgeIdx[e.Source+">"+e.Target] = i
	}

	var extractor sqllineage.Extractor

	for _, a := range assets {
		if a.Type != "View" || a.Query == nil || a.MRN == nil {
			continue
		}
		view := assetKey(a)
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             *a.Query,
			Dialect:         "presto",
			DefaultDatabase: view.Catalog,
			DefaultSchema:   view.Schema,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", view.String()).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		for sourceMRN, colEdges := range result {
			if sourceMRN == *a.MRN {
				continue
			}
			key := sourceMRN + ">" + *a.MRN
			if i, ok := edgeIdx[key]; ok {
				edges[i].ColumnLineage = colEdges
				continue
			}
			edgeIdx[key] = len(edges)
			edges = append(edges, pluginsdk.LineageEdge{
				Source:        sourceMRN,
				Target:        *a.MRN,
				Type:          "VIEW_OF",
				ColumnLineage: colEdges,
			})
		}
	}

	return edges
}

// buildExtractSources describes every discovered table and view to the
// parser, with the catalog as its database.
func buildExtractSources(assets []pluginsdk.Asset) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assets))
	for _, a := range assets {
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil {
			continue
		}
		key := assetKey(a)
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: key.Catalog,
			Schema:   key.Schema,
			Table:    key.Table,
			Columns:  decodeColumns(a.Schema["columns"]),
		})
	}
	return sources
}

// decodeColumns pulls (name → type) out of the JSON attachColumns stores
// in Asset.Schema["columns"].
func decodeColumns(raw string) map[string]string {
	out := map[string]string{}
	if raw == "" {
		return out
	}
	var cols []struct {
		Name     string `json:"column_name"`
		DataType string `json:"data_type"`
	}
	if err := json.Unmarshal([]byte(raw), &cols); err != nil {
		return out
	}
	for _, c := range cols {
		if c.Name != "" {
			out[c.Name] = c.DataType
		}
	}
	return out
}
