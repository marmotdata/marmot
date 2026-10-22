package dataquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SourceTable is a catalogued table that a queryable source can read, with a
// statement that previews it in the source's own dialect.
type SourceTable struct {
	MRN             string   `json:"mrn"`
	AssetID         string   `json:"asset_id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Schema          string   `json:"schema,omitempty"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Queries         int64    `json:"queries"`
	SampleStatement string   `json:"sample_statement,omitempty"`
} // @name QuerySourceTable

// SourceTablesFilter narrows the tables listed for a source. Metadata pins
// asset metadata keys to values, so a source only lists tables it can reach.
type SourceTablesFilter struct {
	SourceID string
	// SourceLabel names the plugin's entry in an asset's sources, whose
	// properties win over the merged metadata when several plugins describe
	// the same table.
	SourceLabel string
	Search      string
	Metadata    map[string]string
	Required    []string
	Limit       int
}

// sourceTableLocator says which tables a plugin's queries can reach and how
// to write a preview statement for one.
type sourceTableLocator struct {
	label     string
	required  []string
	metadata  func(config map[string]any) map[string]string
	statement func(meta map[string]any, name string) string
}

var sourceTableLocators = map[string]sourceTableLocator{
	"postgresql": {
		label:    "PostgreSQL",
		required: []string{"schema"},
		metadata: func(config map[string]any) map[string]string {
			database, _ := config["query_database"].(string)
			if database == "" {
				database = "postgres"
			}
			return map[string]string{"database": database}
		},
		statement: func(meta map[string]any, name string) string {
			schema := metaString(meta, "schema")
			table := metaString(meta, "table_name")
			if table == "" {
				table = name
			}
			if schema == "" || schema == "public" {
				return previewStatement(quoteIdent(table))
			}
			return previewStatement(quoteIdent(schema) + "." + quoteIdent(table))
		},
	},
	"trino": {
		label:    "Trino",
		required: []string{"catalog", "schema", "table_name"},
		statement: func(meta map[string]any, _ string) string {
			return previewStatement(quoteIdent(metaString(meta, "catalog")) + "." +
				quoteIdent(metaString(meta, "schema")) + "." + quoteIdent(metaString(meta, "table_name")))
		},
	},
}

// systemSchemas are left out of source table lists: nobody wants to preview
// the engine's own catalog.
var systemSchemas = []string{"pg_catalog", "information_schema", "pg_toast"}

func previewStatement(table string) string {
	return "SELECT *\nFROM " + table + "\nLIMIT 10"
}

// likeEscaper makes user input literal inside an ILIKE pattern.
var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

var plainIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// quoteIdent leaves plain lower-case identifiers bare and double-quotes
// anything else, which is valid in both PostgreSQL and Trino.
func quoteIdent(id string) string {
	if plainIdent.MatchString(id) {
		return id
	}
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

func metaString(meta map[string]any, key string) string {
	v, _ := meta[key].(string)
	return v
}

func (r *PostgresRepository) ListSourceTables(ctx context.Context, f SourceTablesFilter) ([]SourceTable, map[string]map[string]any, error) {
	args := []any{f.SourceID, "%" + likeEscaper.Replace(f.Search) + "%", systemSchemas, time.Now().Add(-30 * 24 * time.Hour), f.SourceLabel}
	prop := func(keyArg int) string {
		return fmt.Sprintf("COALESCE(p.props->>$%d, a.metadata->>$%d)", keyArg, keyArg)
	}
	where := ""
	for key, value := range f.Metadata {
		args = append(args, key, value)
		where += fmt.Sprintf(" AND %s = $%d", prop(len(args)-1), len(args))
	}
	for _, key := range f.Required {
		args = append(args, key)
		where += fmt.Sprintf(" AND COALESCE(%s, '') <> ''", prop(len(args)))
	}
	args = append(args, f.Limit)

	rows, err := r.db.Query(ctx, `
		SELECT a.mrn, a.id, COALESCE(a.name, ''), a.type, COALESCE(a.metadata->>'schema', ''),
		       COALESCE(NULLIF(a.user_description, ''), a.description, ''), COALESCE(a.tags, '{}'),
		       COALESCE(u.queries, 0), COALESCE(a.metadata, '{}'::jsonb) || COALESCE(p.props, '{}'::jsonb)
		FROM asset_schedules link
		JOIN assets a ON a.id = link.asset_id
		LEFT JOIN LATERAL (
			SELECT s->'properties' AS props
			FROM jsonb_array_elements(COALESCE(a.sources, '[]'::jsonb)) s
			WHERE s->>'name' = $5
			LIMIT 1
		) p ON true
		LEFT JOIN LATERAL (
			SELECT sum(t.queries) AS queries FROM data_query_table_stats_hourly t
			WHERE t.mrn = a.mrn AND t.bucket >= $4
		) u ON true
		WHERE link.schedule_id = $1
		  AND a.type IN ('Table', 'View')
		  AND a.name ILIKE $2
		  AND COALESCE(a.metadata->>'schema', '') <> ALL($3)`+where+fmt.Sprintf(`
		ORDER BY COALESCE(u.queries, 0) DESC, a.name
		LIMIT $%d`, len(args)), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("listing source tables: %w", err)
	}
	defer rows.Close()

	tables := []SourceTable{}
	meta := map[string]map[string]any{}
	for rows.Next() {
		var t SourceTable
		var m map[string]any
		if err := rows.Scan(&t.MRN, &t.AssetID, &t.Name, &t.Type, &t.Schema, &t.Description, &t.Tags, &t.Queries, &m); err != nil {
			return nil, nil, fmt.Errorf("scanning source table: %w", err)
		}
		tables = append(tables, t)
		meta[t.MRN] = m
	}
	return tables, meta, rows.Err()
}

// SourceActivity fills in each source's catalogued table count and recent
// query activity.
func (r *PostgresRepository) SourceActivity(ctx context.Context, sources []*Source) error {
	if len(sources) == 0 {
		return nil
	}
	ids := make([]string, len(sources))
	names := make([]string, len(sources))
	byID := map[string]*Source{}
	byName := map[string]*Source{}
	for i, s := range sources {
		ids[i], names[i] = s.ID, s.Name
		byID[s.ID], byName[s.Name] = s, s
	}

	rows, err := r.db.Query(ctx, `
		SELECT link.schedule_id::text, count(*)
		FROM asset_schedules link
		JOIN assets a ON a.id = link.asset_id
		WHERE link.schedule_id::text = ANY($1) AND a.type IN ('Table', 'View')
		  AND COALESCE(a.metadata->>'schema', '') <> ALL($2)
		GROUP BY link.schedule_id`, ids, systemSchemas)
	if err != nil {
		return fmt.Errorf("counting source tables: %w", err)
	}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return err
		}
		if source, ok := byID[id]; ok {
			source.TableCount = n
		}
	}
	rows.Close()

	// The week's count comes from the rollup; the last query time comes from
	// the log, one indexed lookup per source, so it is exact to the second.
	rows, err = r.db.Query(ctx, `
		SELECT n.name, COALESCE(w.queries, 0), l.started_at
		FROM unnest($1::text[]) AS n(name)
		LEFT JOIN LATERAL (
			SELECT sum(queries) AS queries FROM data_query_stats_hourly
			WHERE source_name = n.name AND bucket >= $2
		) w ON true
		LEFT JOIN LATERAL (
			SELECT started_at FROM data_queries
			WHERE source_name = n.name
			ORDER BY started_at DESC LIMIT 1
		) l ON true`, names, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return fmt.Errorf("loading source activity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var week int64
		var last *time.Time
		if err := rows.Scan(&name, &week, &last); err != nil {
			return err
		}
		if source, ok := byName[name]; ok {
			source.QueriesLastWeek = week
			source.LastQueriedAt = last
		}
	}
	return rows.Err()
}

// ListSourceTables lists the catalogued tables a source can reach, busiest
// first, each with a statement that previews it.
func (s *service) ListSourceTables(ctx context.Context, sourceName, search string, limit int) ([]SourceTable, error) {
	source, err := s.sources.GetQueryableSource(ctx, sourceName)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	filter := SourceTablesFilter{SourceID: source.ID, Search: strings.TrimSpace(search), Limit: limit}
	locator, known := sourceTableLocators[source.PluginID]
	if known {
		filter.SourceLabel = locator.label
		filter.Required = locator.required
		if locator.metadata != nil {
			filter.Metadata = locator.metadata(source.Config)
		}
	}

	tables, meta, err := s.repo.ListSourceTables(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		if known {
			tables[i].SampleStatement = locator.statement(meta[tables[i].MRN], tables[i].Name)
		} else {
			tables[i].SampleStatement = previewStatement(quoteIdent(tables[i].Name))
		}
	}
	return tables, nil
}
