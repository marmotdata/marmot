package trino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	"github.com/rs/zerolog/log"
)

// viewLineage parses each view's SQL and emits a VIEW_OF edge, carrying
// column lineage, from every discovered table or view it reads. Views
// read across catalogs, so it runs once every catalog is in. A view the
// parser cannot handle gets no edges.
func viewLineage(ctx context.Context, assets []pluginsdk.Asset, viewSQL map[string]string) []pluginsdk.LineageEdge {
	if len(viewSQL) == 0 {
		return nil
	}

	sources := buildExtractSources(assets)
	if len(sources) == 0 {
		return nil
	}

	var extractor sqllineage.Extractor
	var edges []pluginsdk.LineageEdge

	for _, a := range assets {
		if a.Type != "View" || a.MRN == nil {
			continue
		}
		query, ok := viewSQL[*a.MRN]
		if !ok {
			continue
		}
		view := keyOf(a)
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             query,
			Dialect:         "trino",
			DefaultDatabase: view.catalog,
			DefaultSchema:   view.schema,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", view.String()).Msg("view lineage extract failed; skipping view lineage")
			continue
		}
		for sourceMRN, colEdges := range result {
			if sourceMRN == *a.MRN {
				continue
			}
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

type tableKey struct{ catalog, schema, table string }

func (k tableKey) String() string { return k.catalog + "." + k.schema + "." + k.table }

func keyOf(a pluginsdk.Asset) tableKey {
	catalog, _ := a.Metadata["catalog"].(string)
	schema, _ := a.Metadata["schema"].(string)
	table, _ := a.Metadata["table_name"].(string)
	return tableKey{catalog, schema, table}
}

// buildExtractSources describes every discovered table and view to the
// parser, with the catalog as its database.
func buildExtractSources(assets []pluginsdk.Asset) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assets))
	for _, a := range assets {
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil {
			continue
		}
		key := keyOf(a)
		if key.table == "" {
			continue
		}
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: key.catalog,
			Schema:   key.schema,
			Table:    key.table,
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

// fetchViewDefinitions reads the SQL of a schema's views, keyed by view
// name. SHOW CREATE TABLE refuses views, so this is the only place
// discovery sees their SQL.
func (s *Source) fetchViewDefinitions(ctx context.Context, catalog, schema string) (map[string]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitized via quoteIdentifier/escapeString
		"SELECT table_name, view_definition FROM %s.information_schema.views WHERE table_schema = '%s'",
		quoteIdentifier(catalog),
		escapeString(schema),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying view definitions in %s.%s: %w", catalog, schema, err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name string
		var definition sql.NullString
		if err := rows.Scan(&name, &definition); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Str("schema", schema).Msg("Failed to scan view row")
			continue
		}
		if definition.Valid && definition.String != "" {
			out[name] = definition.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view rows: %w", err)
	}
	return out, nil
}

// collectViewDefinitions records the SQL of a schema's views by MRN and
// shows it as the view's query where SHOW CREATE TABLE gave none.
func (s *Source) collectViewDefinitions(ctx context.Context, catalog, schema string, assets []pluginsdk.Asset, viewSQL map[string]string) {
	hasView := false
	for i := range assets {
		if assets[i].Type == "View" {
			hasView = true
			break
		}
	}
	if !hasView {
		return
	}

	defs, err := s.fetchViewDefinitions(ctx, catalog, schema)
	if err != nil {
		log.Warn().Err(err).Str("catalog", catalog).Str("schema", schema).Msg("Failed to query view definitions")
		return
	}

	for i := range assets {
		if assets[i].Type != "View" {
			continue
		}
		def, ok := defs[keyOf(assets[i]).table]
		if !ok {
			continue
		}
		viewSQL[*assets[i].MRN] = def
		if assets[i].Query == nil {
			lang := "sql"
			assets[i].Query = &def
			assets[i].QueryLanguage = &lang
		}
	}
}
