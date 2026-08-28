package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	"github.com/rs/zerolog/log"
)

func (s *Source) discoverViewLineage(ctx context.Context, dbName string, dbAssets []pluginsdk.Asset) ([]pluginsdk.LineageEdge, error) {
	if len(dbAssets) == 0 {
		return nil, nil
	}

	viewDefs, err := s.fetchViewDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching view definitions: %w", err)
	}
	if len(viewDefs) == 0 {
		return nil, nil
	}

	assetIdx := map[schemaTable]*pluginsdk.Asset{}
	for i := range dbAssets {
		a := &dbAssets[i]
		if a.Type != "Table" && a.Type != "View" {
			continue
		}
		st, ok := schemaTableOf(a)
		if !ok {
			continue
		}
		assetIdx[st] = a
	}

	sources := buildExtractSources(assetIdx, dbName)
	if len(sources) == 0 {
		return nil, nil
	}

	extractor := sqllineage.New()
	var edges []pluginsdk.LineageEdge

	for st, sql := range viewDefs {
		viewAsset, ok := assetIdx[st]
		if !ok {
			continue
		}
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             sql,
			Dialect:         "postgres",
			DefaultDatabase: dbName,
			DefaultSchema:   st.schema,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", st.String()).Msg("view lineage extract failed; falling back to table-only")
			continue
		}
		for sourceMRN, colEdges := range result {
			if sourceMRN == *viewAsset.MRN {
				continue
			}
			edges = append(edges, pluginsdk.LineageEdge{
				Source:        sourceMRN,
				Target:        *viewAsset.MRN,
				Type:          "DEPENDS_ON",
				ColumnLineage: colEdges,
			})
		}
	}

	return edges, nil
}

type schemaTable struct{ schema, table string }

func (s schemaTable) String() string { return s.schema + "." + s.table }

func schemaTableOf(a *pluginsdk.Asset) (schemaTable, bool) {
	schema, ok := a.Metadata["schema"].(string)
	if !ok {
		return schemaTable{}, false
	}
	table, ok := a.Metadata["table_name"].(string)
	if !ok {
		return schemaTable{}, false
	}
	return schemaTable{schema: schema, table: table}, true
}

func buildExtractSources(assetIdx map[schemaTable]*pluginsdk.Asset, dbName string) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assetIdx))
	for st, a := range assetIdx {
		if a.MRN == nil {
			continue
		}
		cols := decodeColumns(a.Schema["columns"])
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: dbName,
			Schema:   st.schema,
			Table:    st.table,
			Columns:  cols,
		})
	}
	return sources
}

// decodeColumns pulls (name → type) out of the JSON blob the postgres
// plugin stashes into Asset.Schema["columns"] (see getBulkColumnInfo).
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

func (s *Source) fetchViewDefinitions(ctx context.Context) (map[schemaTable]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
        SELECT schemaname, viewname, definition FROM pg_catalog.pg_views
        WHERE (schemaname !~ '^pg_' OR NOT $1) AND schemaname != 'information_schema'
        UNION ALL
        SELECT schemaname, matviewname, definition FROM pg_catalog.pg_matviews
        WHERE (schemaname !~ '^pg_' OR NOT $1) AND schemaname != 'information_schema'`

	rows, err := s.pool.Query(ctx, query, s.config.ExcludeSystemSchemas)
	if err != nil {
		return nil, fmt.Errorf("querying view definitions: %w", err)
	}
	defer rows.Close()

	out := map[schemaTable]string{}
	for rows.Next() {
		var schema, name, def string
		if err := rows.Scan(&schema, &name, &def); err != nil {
			log.Warn().Err(err).Msg("scanning view definition")
			continue
		}
		out[schemaTable{schema: schema, table: name}] = def
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view definitions: %w", err)
	}
	return out, nil
}
