package mssql

import (
	"context"
	"regexp"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/tsql"
	"github.com/rs/zerolog/log"
)

var (
	// Matches the CREATE VIEW header sys.sql_modules keeps in front of the
	// view's SELECT, up to and including AS: leading comments, an optional
	// OR ALTER, a one to three part name, an optional column list and any
	// WITH SCHEMABINDING / ENCRYPTION / VIEW_METADATA attributes. The column
	// list is captured so the SELECT's output can be renamed to match it.
	viewHeaderRE = regexp.MustCompile(`(?is)^(?:\s+|--[^\n]*\n|/\*.*?\*/)*create\s+(?:or\s+alter\s+)?view\s+` +
		identPattern + `(?:\s*\.\s*` + identPattern + `){0,2}\s*(?:\(([^()]*)\))?\s*(?:with\s+\w+(?:\s*,\s*\w+)*\s+)?as\b`)

	// Matches the clauses that can trail a view's SELECT but are not part of
	// a query, which the parser rejects.
	viewTrailerRE = regexp.MustCompile(`(?is)(?:\s+with\s+check\s+option)?\s*;?\s*$`)
)

// viewColumnLineage parses a view definition and returns its column edges
// keyed by source MRN. It returns nil when the definition cannot be parsed,
// so the caller keeps the table-level edges on their own.
func viewColumnLineage(ctx context.Context, extractor *sqllineage.Extractor, databaseName, schemaName, viewName, definition string, sources []sqllineage.ExtractSource) map[string][]pluginsdk.ColumnEdge {
	query, ok := viewQuery(definition)
	if !ok || len(sources) == 0 {
		return nil
	}
	result, err := extractor.Extract(ctx, sqllineage.ExtractRequest{
		SQL:             query,
		Dialect:         "mssql",
		DefaultDatabase: databaseName,
		DefaultSchema:   schemaName,
		Sources:         sources,
	})
	if err != nil {
		log.Info().Err(err).Str("view", qualifiedName(databaseName, schemaName, viewName)).Msg("view lineage extract failed; falling back to table-only")
		return nil
	}
	return result
}

// viewQuery turns a stored view definition into the plain SELECT the parser
// accepts. SQL Server keeps the whole CREATE VIEW statement, so the header
// and any WITH CHECK OPTION are cut away. A view that renames its columns in
// a column list has its SELECT wrapped in a derived table with those names,
// so the edges land on the view's columns rather than the SELECT's aliases.
func viewQuery(definition string) (string, bool) {
	header := viewHeaderRE.FindStringSubmatchIndex(definition)
	if header == nil {
		return "", false
	}
	body := viewTrailerRE.ReplaceAllString(definition[header[1]:], "")
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false
	}
	if header[2] < 0 {
		return body, true
	}

	var columns []string
	for _, name := range strings.Split(definition[header[2]:header[3]], ",") {
		name = unquoteIdent(strings.TrimSpace(name))
		if name == "" {
			return "", false
		}
		columns = append(columns, quoteIdent(name))
	}
	return "SELECT * FROM (" + body + ") AS [v](" + strings.Join(columns, ", ") + ")", true
}

// buildExtractSources describes every table and view discovered so far to
// the parser, including earlier databases so three-part names resolve.
func buildExtractSources(cfg *Config, objects map[string]objectRef) []sqllineage.ExtractSource {
	sources := make([]sqllineage.ExtractSource, 0, len(objects))
	for _, ref := range objects {
		sources = append(sources, sqllineage.ExtractSource{
			MRN:      ref.mrn(cfg),
			Database: ref.Database,
			Schema:   ref.Schema,
			Table:    ref.Name,
			Columns:  ref.Columns,
		})
	}
	return sources
}

// columnTypes maps each column to the type SQL Server would declare it with.
func columnTypes(columns []columnInfo) map[string]string {
	out := make(map[string]string, len(columns))
	for _, c := range columns {
		out[c.Name] = renderDataType(c.TypeName, c.MaxLength, c.Precision, c.Scale)
	}
	return out
}
