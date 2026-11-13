package dbt

import (
	"cmp"
	"context"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	sqllineage "github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/bigquery"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/oracle"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/postgres"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/redshift"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/snowflake"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/tsql"
	"github.com/rs/zerolog/log"
)

// sqlDialect is how a dbt adapter's compiled SQL is parsed. schemaOnly marks
// engines with a single level above tables, where dbt's schema is what the
// parser calls the database.
type sqlDialect struct {
	name       string
	schemaOnly bool
}

// sqlDialects maps manifest adapter_type to the parser for its SQL. Adapters
// left out (spark, databricks, duckdb, clickhouse and so on) keep
// table-level lineage only.
var sqlDialects = map[string]sqlDialect{
	"postgres":   {name: "postgres"},
	"postgresql": {name: "postgres"},
	"alloydb":    {name: "postgres"},
	"redshift":   {name: "redshift"},
	"snowflake":  {name: "snowflake"},
	"bigquery":   {name: "bigquery"},
	"trino":      {name: "trino"},
	"starburst":  {name: "trino"},
	"athena":     {name: "trino"},
	"mysql":      {name: "mysql", schemaOnly: true},
	"mariadb":    {name: "mariadb", schemaOnly: true},
	"sqlserver":  {name: "mssql"},
	"mssql":      {name: "mssql"},
	"synapse":    {name: "mssql"},
	"fabric":     {name: "mssql"},
	"oracle":     {name: "oracle", schemaOnly: true},
}

// sqlDialect returns the parser for the manifest's adapter, if there is one.
func (s *Source) sqlDialect() (sqlDialect, bool) {
	if s.manifest == nil {
		return sqlDialect{}, false
	}
	d, ok := sqlDialects[strings.ToLower(s.manifest.Metadata.AdapterType)]
	return d, ok
}

// attachColumnLineage parses a model's compiled SQL and sets ColumnLineage
// on its DEPENDS_ON edges. upstream maps each dependency's node ID to the
// MRN its edge starts from. Any failure leaves the edges table-level.
func (s *Source) attachColumnLineage(ctx context.Context, node ManifestNode, upstream map[string]string, edges []pluginsdk.LineageEdge) {
	d, ok := s.sqlDialect()
	if !ok || len(upstream) == 0 {
		return
	}
	sql := cmp.Or(node.CompiledCode, node.CompiledSQL)
	if sql == "" {
		return
	}

	sources := make([]sqllineage.ExtractSource, 0, len(upstream))
	for depID, depMRN := range upstream {
		dep, ok := s.lookupNode(depID)
		if !ok {
			continue
		}
		sources = append(sources, d.extractSource(depMRN, dep, s.nodeColumns(depID, dep)))
	}

	req := sqllineage.ExtractRequest{SQL: sql, Dialect: d.name, DefaultDatabase: node.Database, DefaultSchema: node.Schema}
	if d.schemaOnly {
		req.DefaultDatabase, req.DefaultSchema = node.Schema, ""
	}
	req.Sources = sources

	var extractor sqllineage.Extractor
	result, err := extractor.Extract(ctx, req)
	if err != nil {
		log.Debug().Err(err).Str("model", node.UniqueID).Msg("model column lineage extract failed; keeping table-level lineage")
		return
	}

	modelCols := columnNameIndex(s.nodeColumns(node.UniqueID, node))
	for i := range edges {
		e := &edges[i]
		if e.Type != "DEPENDS_ON" {
			continue
		}
		colEdges := result[e.Source]
		for j := range colEdges {
			if name, ok := modelCols[strings.ToLower(colEdges[j].ToColumn)]; ok {
				colEdges[j].ToColumn = name
			}
		}
		e.ColumnLineage = colEdges
	}
}

// extractSource describes one upstream relation by the name its SQL uses:
// a source's identifier, else its alias, else its name.
func (d sqlDialect) extractSource(mrn string, dep ManifestNode, columns map[string]string) sqllineage.ExtractSource {
	src := sqllineage.ExtractSource{
		MRN:      mrn,
		Database: dep.Database,
		Schema:   dep.Schema,
		Table:    cmp.Or(dep.Identifier, dep.Alias, dep.Name),
		Columns:  columns,
	}
	if d.schemaOnly {
		src.Database, src.Schema = dep.Schema, ""
	}
	return src
}

func (s *Source) lookupNode(id string) (ManifestNode, bool) {
	if n, ok := s.manifest.Nodes[id]; ok {
		return n, true
	}
	n, ok := s.manifest.Sources[id]
	return n, ok
}

// nodeColumns returns (name → type) for a node, preferring catalog.json, which
// reflects the built relation, over the columns documented in the manifest.
func (s *Source) nodeColumns(id string, node ManifestNode) map[string]string {
	out := map[string]string{}
	if s.catalog != nil {
		cn, ok := s.catalog.Nodes[id]
		if !ok {
			cn, ok = s.catalog.Sources[id]
		}
		if ok {
			for key, c := range cn.Columns {
				out[cmp.Or(c.Name, key)] = c.Type
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	for key, c := range node.Columns {
		out[cmp.Or(c.Name, key)] = c.DataType
	}
	return out
}

// columnNameIndex maps lower-cased column names to their spelling in the
// model, since some parsers change the case of result names.
func columnNameIndex(cols map[string]string) map[string]string {
	out := make(map[string]string, len(cols))
	for name := range cols {
		out[strings.ToLower(name)] = name
	}
	return out
}
