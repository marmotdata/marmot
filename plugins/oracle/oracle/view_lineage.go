package oracle

import (
	"context"
	"regexp"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/oracle"
	"github.com/rs/zerolog/log"
)

const oracleIdent = `(?:"[^"]*"|[A-Za-z_][\w$#]*)`

var (
	// Matches the CREATE VIEW header DBMS_METADATA.GET_DDL puts in front of
	// the view's query, up to and including AS: OR REPLACE, FORCE and the
	// editioning keywords, the optionally owner-qualified name, an optional
	// column list and clauses such as DEFAULT COLLATION or BEQUEATH. Text
	// read from ALL_VIEWS or ALL_MVIEWS is the bare query and does not match.
	viewHeaderRE = regexp.MustCompile(`(?is)^\s*create\s+(?:or\s+replace\s+)?(?:(?:no\s+)?force\s+)?(?:(?:editioning|editionable|noneditionable)\s+)*view\s+` +
		oracleIdent + `(?:\s*\.\s*` + oracleIdent + `)?\s*(?:\([^()]*\))?.*?\bas\b`)

	// Matches the restriction clauses that can trail a view's query but are
	// not part of it, which the parser rejects.
	viewTrailerRE = regexp.MustCompile(`(?is)(?:\s+with\s+(?:read\s+only|check\s+option)(?:\s+constraint\s+` + oracleIdent + `)?)?\s*;?\s*$`)
)

// viewColumnLineage parses a view's SQL and returns its column edges keyed
// by source MRN. It returns nil when the SQL cannot be parsed, so the caller
// keeps the table-level edges on their own. viewColumns is the view's own
// catalog columns; when known, an edge is kept only if it lands on one of
// them, since a view declared with a column list renames its query's output
// and those edges would otherwise point at columns the view does not have.
func viewColumnLineage(ctx context.Context, extractor *sqllineage.Extractor, view objectRef, text string, viewColumns map[string]string, sources []sqllineage.ExtractSource) map[string][]pluginsdk.ColumnEdge {
	query := viewQuery(text)
	if query == "" || len(sources) == 0 {
		return nil
	}
	result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
		SQL:             query,
		Dialect:         "oracle",
		DefaultDatabase: view.Owner,
		Sources:         sources,
	})
	if err != nil {
		log.Info().Err(err).Str("view", objectName(view.Owner, view.Name)).Msg("view lineage extract failed; falling back to table-only")
		return nil
	}
	if len(viewColumns) == 0 {
		return result
	}

	// Oracle reports unquoted names in upper case; match the parser's
	// output names to the catalog's spelling.
	catalogName := make(map[string]string, len(viewColumns))
	for name := range viewColumns {
		catalogName[strings.ToUpper(name)] = name
	}
	out := make(map[string][]pluginsdk.ColumnEdge, len(result))
	for mrn, edges := range result {
		var kept []pluginsdk.ColumnEdge
		for _, e := range edges {
			if _, exact := viewColumns[e.ToColumn]; !exact {
				name, ok := catalogName[strings.ToUpper(e.ToColumn)]
				if !ok {
					continue
				}
				e.ToColumn = name
			}
			kept = append(kept, e)
		}
		if len(kept) > 0 {
			out[mrn] = kept
		}
	}
	return out
}

// viewQuery reduces a view's stored SQL to the query the parser accepts:
// the CREATE VIEW header DBMS_METADATA adds and any WITH READ ONLY or WITH
// CHECK OPTION are cut away.
func viewQuery(text string) string {
	if header := viewHeaderRE.FindStringIndex(text); header != nil {
		text = text[header[1]:]
	}
	return strings.TrimSpace(viewTrailerRE.ReplaceAllString(text, ""))
}

// buildExtractSources describes every table and view discovered in this run
// to the parser. Oracle's schema is its user, so the owner goes in Database
// and Schema stays empty; names are the catalog's, upper case for unquoted
// objects, which is how the parser resolves unquoted references.
func (d *discovery) buildExtractSources() []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(d.objects))
	for ref, mrn := range d.objects {
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      mrn,
			Database: ref.Owner,
			Table:    ref.Name,
			Columns:  d.columns[ref],
		})
	}
	return sources
}

// addColumnTypes records an object's column names and rendered types for
// view column lineage.
func (d *discovery) addColumnTypes(ref objectRef, rows []columnRow) {
	if len(rows) == 0 {
		return
	}
	if d.columns == nil {
		d.columns = make(map[objectRef]map[string]string)
	}
	types := make(map[string]string, len(rows))
	for _, r := range rows {
		types[r.Name] = renderDataType(r)
	}
	d.columns[ref] = types
}
