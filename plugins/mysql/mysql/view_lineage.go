package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
	"github.com/rs/zerolog/log"
)

// discoverViewLineage parses each view's definition and returns a
// DEPENDS_ON edge, carrying column lineage, from every discovered table or
// view it reads into the view. A view whose SQL cannot be parsed is logged
// and skipped.
func (s *Source) discoverViewLineage(ctx context.Context, dbName string, dbAssets []pluginsdk.Asset) ([]pluginsdk.LineageEdge, error) {
	if len(dbAssets) == 0 {
		return nil, nil
	}

	viewDefs, err := s.fetchViewDefinitions(ctx, dbName)
	if err != nil {
		return nil, fmt.Errorf("fetching view definitions: %w", err)
	}
	if len(viewDefs) == 0 {
		return nil, nil
	}

	// The parser needs each table's columns to expand SELECT * and to
	// attribute unqualified column names in joins.
	columns, err := s.fetchColumns(ctx, dbName)
	if err != nil {
		return nil, fmt.Errorf("fetching columns: %w", err)
	}

	return viewColumnLineage(ctx, dbName, dbAssets, viewDefs, columns), nil
}

// viewColumnLineage is the database-free half of discoverViewLineage:
// given the discovered assets, the view definitions and each table's
// columns, it derives the view edges.
func viewColumnLineage(ctx context.Context, dbName string, dbAssets []pluginsdk.Asset, viewDefs map[string]string, columns map[string][]tableColumn) []pluginsdk.LineageEdge {
	sources := buildExtractSources(dbAssets, dbName, columns)
	if len(sources) == 0 {
		return nil
	}

	var extractor sqllineage.Extractor
	var edges []pluginsdk.LineageEdge

	for i := range dbAssets {
		viewAsset := &dbAssets[i]
		if viewAsset.Type != "View" || viewAsset.MRN == nil {
			continue
		}
		name, _ := viewAsset.Metadata["table_name"].(string)
		def, ok := viewDefs[name]
		if !ok || def == "" {
			continue
		}
		// MySQL has no schema level: the database is the MySQL schema.
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             withViewColumns(def, columnNames(columns[name])),
			Dialect:         "mysql",
			DefaultDatabase: dbName,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", dbName+"."+name).Msg("view lineage extract failed; skipping view")
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

	return edges
}

// buildExtractSources describes every discovered table and view to the
// parser, keyed by the same table_name the assets were built from.
func buildExtractSources(dbAssets []pluginsdk.Asset, dbName string, columns map[string][]tableColumn) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(dbAssets))
	for i := range dbAssets {
		a := &dbAssets[i]
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil {
			continue
		}
		table, ok := a.Metadata["table_name"].(string)
		if !ok {
			continue
		}
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: dbName,
			Table:    table,
			Columns:  columnTypes(columns[table]),
		})
	}
	return sources
}

// viewColumnsCTE names the CTE withViewColumns wraps a view's query in.
const viewColumnsCTE = "__marmot_view"

// withViewColumns wraps a view's query in a CTE that declares the view's
// own column names, in ordinal order. CREATE VIEW v (a, b) AS SELECT x, y
// names the view's columns a and b, not x and y, and the CTE column list
// makes the parser rename the SELECT's outputs positionally the same way.
// When the counts differ the parser rejects the query, so the view falls
// back rather than getting edges on the wrong columns.
func withViewColumns(query string, columns []string) string {
	if len(columns) == 0 {
		return query
	}
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = quoteIdentifier(c)
	}
	// The newlines keep a trailing line comment in the query from
	// swallowing the closing parenthesis.
	query = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(query), ";"))
	cte := quoteIdentifier(viewColumnsCTE)
	return "WITH " + cte + " (" + strings.Join(quoted, ", ") + ") AS (\n" + query + "\n) SELECT * FROM " + cte
}

// tableColumn is one information_schema.COLUMNS row.
type tableColumn struct {
	name, dataType string
}

func columnNames(cols []tableColumn) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	return names
}

func columnTypes(cols []tableColumn) map[string]string {
	types := make(map[string]string, len(cols))
	for _, c := range cols {
		types[c.name] = c.dataType
	}
	return types
}

// fetchViewDefinitions reads the SELECT behind every view in the database.
// VIEW_DEFINITION is empty when the user lacks SHOW VIEW on a view; those
// views are left out.
func (s *Source) fetchViewDefinitions(ctx context.Context, dbName string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, VIEW_DEFINITION
		FROM information_schema.VIEWS
		WHERE TABLE_SCHEMA = ?`

	rows, err := s.db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying view definitions: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name string
		var def sql.NullString
		if err := rows.Scan(&name, &def); err != nil {
			log.Warn().Err(err).Msg("scanning view definition")
			continue
		}
		if !def.Valid || def.String == "" {
			log.Debug().Str("view", name).Msg("view definition is empty, grant SHOW VIEW to read it")
			continue
		}
		out[name] = def.String
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view definitions: %w", err)
	}
	return out, nil
}

// fetchColumns reads the columns of every table and view in the database
// in one query, each table's in ordinal order.
func (s *Source) fetchColumns(ctx context.Context, dbName string) (map[string][]tableColumn, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, COLUMN_NAME, DATA_TYPE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`

	rows, err := s.db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	out := map[string][]tableColumn{}
	for rows.Next() {
		var table string
		var c tableColumn
		if err := rows.Scan(&table, &c.name, &c.dataType); err != nil {
			log.Warn().Err(err).Msg("scanning column")
			continue
		}
		out[table] = append(out[table], c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating columns: %w", err)
	}
	return out, nil
}
