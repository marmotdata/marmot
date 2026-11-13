package starrocks

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
	"github.com/rs/zerolog/log"
)

// attachViewColumnLineage parses the query of each view and materialized
// view and sets the column lineage on the VIEW_OF edges resolveLineage
// already found. StarRocks has no column-level parser of its own; its
// view SQL is MySQL syntax, so the MySQL dialect reads it. The VIEW_OF
// edges stay authoritative for which objects a view reads: a view whose SQL cannot be
// parsed keeps its table-level edges, and a parsed reference with no
// matching edge is dropped rather than added.
func attachViewColumnLineage(ctx context.Context, catalog string, assets []pluginsdk.Asset, edges []pluginsdk.LineageEdge) {
	edgeIdx := map[[2]string]*pluginsdk.LineageEdge{}
	for i := range edges {
		if edges[i].Type == "VIEW_OF" {
			edgeIdx[[2]string{edges[i].Source, edges[i].Target}] = &edges[i]
		}
	}
	if len(edgeIdx) == 0 {
		return
	}

	sources := buildExtractSources(assets)
	if len(sources) == 0 {
		return
	}

	var extractor sqllineage.Extractor

	for i := range assets {
		viewAsset := &assets[i]
		if viewAsset.Type != "View" || viewAsset.MRN == nil || viewAsset.Query == nil || *viewAsset.Query == "" {
			continue
		}
		database, _ := viewAsset.Metadata["database"].(string)
		_, viewColumns := decodeColumns(viewAsset.Schema["columns"])
		// MySQL has no catalog level: StarRocks's database is the MySQL schema.
		result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
			SQL:             withViewColumns(stripCatalog(*viewAsset.Query, catalog), viewColumns),
			Dialect:         "mysql",
			DefaultDatabase: database,
			Sources:         sources,
		})
		if err != nil {
			log.Info().Err(err).Str("view", database+"."+*viewAsset.Name).Msg("view lineage extract failed; falling back to table-only")
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

// buildExtractSources describes every discovered table and view, across
// all databases of the run, with the columns stored in its schema.
func buildExtractSources(assets []pluginsdk.Asset) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(assets))
	for i := range assets {
		a := &assets[i]
		if (a.Type != "Table" && a.Type != "View") || a.MRN == nil {
			continue
		}
		database, _ := a.Metadata["database"].(string)
		table, _ := a.Metadata["table_name"].(string)
		if database == "" || table == "" {
			continue
		}
		columns, _ := decodeColumns(a.Schema["columns"])
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      *a.MRN,
			Database: database,
			Table:    table,
			Columns:  columns,
		})
	}
	return sources
}

// decodeColumns pulls (name → type) out of the JSON blob objectAsset
// stores in Asset.Schema["columns"], plus the names in their ordinal
// order.
func decodeColumns(raw string) (map[string]string, []string) {
	out := map[string]string{}
	if raw == "" {
		return out, nil
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return out, nil
	}
	var names []string
	for _, c := range arr {
		name, _ := c["column_name"].(string)
		typ, _ := c["data_type"].(string)
		if name != "" {
			out[name] = typ
			names = append(names, name)
		}
	}
	return out, names
}

// viewColumnsCTE names the CTE withViewColumns wraps a view's query in.
const viewColumnsCTE = "__marmot_view"

// withViewColumns wraps a view's query in a CTE that declares the view's
// own column names, in ordinal order. CREATE VIEW v (a, b) AS SELECT x, y
// names the view's columns a and b, not x and y, and the stored query the
// asset carries has lost that list; the CTE column list makes the parser
// rename the SELECT's outputs positionally the same way. When the counts
// differ the parser rejects the query, so the view falls back to its
// table-level edges rather than getting column lineage on the wrong
// columns.
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

const sqlIdent = "(?:`(?:[^`]|``)+`|[A-Za-z_][A-Za-z0-9_$]*)"

// stripCatalog drops the catalog qualifier from names such as
// `default_catalog`.`shop`.`orders` (and the column form
// `default_catalog`.`shop`.`orders`.`id`) that StarRocks writes into
// stored view SQL. MySQL has no catalog level, so its parser rejects them. Only a catalog
// followed by at least two more name parts is removed, so a table or alias
// that happens to share the catalog's name is left alone.
func stripCatalog(query, catalog string) string {
	if catalog == "" {
		return query
	}
	quoted := regexp.QuoteMeta(catalog)
	re, err := regexp.Compile("(?i)(?:`" + quoted + "`|\\b" + quoted + "\\b)\\s*\\.\\s*(" + sqlIdent + "\\s*\\.\\s*" + sqlIdent + ")")
	if err != nil {
		return query
	}
	return re.ReplaceAllString(query, "$1")
}
