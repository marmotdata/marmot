package cockroachdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Schemas CockroachDB creates in every database for its own catalogs.
const systemSchemas = `('pg_catalog', 'information_schema', 'crdb_internal', 'pg_extension')`

// pg_class relkind values, as CockroachDB reports them.
const (
	relkindTable            = "r"
	relkindPartitionedTable = "p"
	relkindForeignTable     = "f"
	relkindView             = "v"
	relkindMaterializedView = "m"
)

type database struct {
	Name  string
	Owner string
}

// relation is one pg_class row worth of a table or view.
type relation struct {
	Schema  string
	Name    string
	Kind    string
	Comment string
	Owner   string
}

// key is how the per-database maps address a relation.
func (r relation) key() string {
	return r.Schema + "." + r.Name
}

func (r relation) assetType() string {
	if r.isView() {
		return "View"
	}
	return "Table"
}

func (r relation) isView() bool {
	return r.Kind == relkindView || r.Kind == relkindMaterializedView
}

func (r relation) objectType() string {
	switch r.Kind {
	case relkindView:
		return "view"
	case relkindMaterializedView:
		return "materialized_view"
	case relkindForeignTable:
		return "foreign_table"
	default:
		return "table"
	}
}

// column is the per-column shape serialised into an asset's schema. It adds
// the computed column expression to the SDK's canonical column.
type column struct {
	pluginsdk.Column
	GenerationExpression string `json:"generation_expression,omitempty"`
}

// columnRow is one information_schema.columns row, before hidden columns
// are dropped and primary keys are marked.
type columnRow struct {
	Schema     string
	Table      string
	Name       string
	DataType   string
	Nullable   bool
	Default    *string
	Generation string
	Hidden     bool
	Comment    string
}

type partition struct {
	Columns []string
}

// databaseResult is what one database contributes to the discovery result.
type databaseResult struct {
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic
}

// listDatabases connects once to read the cluster's databases and version,
// then narrows them to the ones discovery should visit.
func (s *Source) listDatabases(ctx context.Context) ([]database, string, error) {
	// The bootstrap database only has to be reachable: pg_database and
	// version() do not depend on which one the session is in.
	bootstrap := s.config.Database
	if bootstrap == "" {
		bootstrap = "defaultdb"
	}

	conn, err := s.connect(ctx, bootstrap)
	if err != nil {
		return nil, "", err
	}
	defer conn.Close(ctx)

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var serverVersion string
	if err := conn.QueryRow(queryCtx, "SELECT version()").Scan(&serverVersion); err != nil {
		log.Warn().Err(err).Msg("Failed to read server version")
	}

	rows, err := conn.Query(queryCtx, `
		SELECT d.datname, pg_catalog.pg_get_userbyid(d.datdba)
		FROM pg_catalog.pg_database d
		ORDER BY d.datname`)
	if err != nil {
		return nil, "", fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	var all []database
	for rows.Next() {
		var db database
		var owner sql.NullString
		if err := rows.Scan(&db.Name, &owner); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}
		db.Owner = owner.String
		all = append(all, db)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterating database rows: %w", err)
	}

	selected, err := selectDatabases(all, s.config.Database, s.config.ExcludeDatabases)
	if err != nil {
		return nil, "", err
	}

	log.Debug().Int("count", len(selected)).Msg("Selected databases")
	return selected, serverVersion, nil
}

// discoverDatabase reads everything about one database over one connection.
// A failure in an optional pass (columns, statistics, lineage) is logged and
// the pass skipped; only failing to list the relations fails the database.
func (s *Source) discoverDatabase(ctx context.Context, dbName string) (*databaseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	conn, err := s.connect(ctx, dbName)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	relations, err := listRelations(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("listing relations: %w", err)
	}
	if !s.config.IncludeViews {
		relations = withoutViews(relations)
	}
	log.Debug().Str("database", dbName).Int("count", len(relations)).Msg("Found relations")

	columns := make(map[string][]column)
	if s.config.IncludeColumns {
		columnRows, err := listColumns(ctx, conn, dbName)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to list columns")
		} else {
			primaryKeys, err := listPrimaryKeys(ctx, conn, dbName)
			if err != nil {
				log.Warn().Err(err).Str("database", dbName).Msg("Failed to list primary keys")
			}
			columns = buildColumns(columnRows, primaryKeys)
		}
	}

	partitions, err := listPartitions(ctx, conn, dbName)
	if err != nil {
		log.Warn().Err(err).Str("database", dbName).Msg("Failed to list partitions")
	}

	definitions := make(map[string]string)
	if s.config.IncludeViews {
		definitions, err = listViewDefinitions(ctx, conn)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to list view definitions")
		}
	}

	rowCounts := make(map[string]int64)
	sizes := make(map[string]int64)
	columnCounts := make(map[string]int64)
	if s.config.IncludeStatistics {
		if rowCounts, err = listRowCounts(ctx, conn, dbName); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to read row statistics")
		}
		if sizes, err = listTableSizes(ctx, conn, dbName); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to read table sizes")
		}
		if columnCounts, err = listColumnCounts(ctx, conn, dbName); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to count columns")
		}
	}

	result := &databaseResult{}
	byKey := make(map[string]relation, len(relations))

	for _, r := range relations {
		byKey[r.key()] = r

		details := relationDetails{
			definition: definitions[r.key()],
			partition:  partitions[r.key()],
		}
		if s.config.IncludeColumns {
			cols := columns[r.key()]
			if cols == nil {
				cols = []column{}
			}
			details.columns = cols
		}
		if count, ok := rowCounts[r.key()]; ok && r.Kind != relkindView {
			details.rowCount = &count
		}

		asset := s.relationAsset(dbName, r, details)
		result.assets = append(result.assets, asset)

		if !s.config.IncludeStatistics {
			continue
		}
		if count, ok := columnCounts[r.key()]; ok {
			result.statistics = append(result.statistics, pluginsdk.Statistic{
				AssetMRN: *asset.MRN, MetricName: "asset.column_count", Value: float64(count),
			})
		}
		if details.rowCount != nil {
			result.statistics = append(result.statistics, pluginsdk.Statistic{
				AssetMRN: *asset.MRN, MetricName: "asset.row_count", Value: float64(*details.rowCount),
			})
		}
		if size, ok := sizes[r.key()]; ok && r.Kind != relkindView {
			result.statistics = append(result.statistics, pluginsdk.Statistic{
				AssetMRN: *asset.MRN, MetricName: "asset.size_bytes", Value: float64(size),
			})
		}
	}

	if s.config.DiscoverForeignKeys {
		edges, err := listForeignKeys(ctx, conn, dbName, byKey)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover foreign keys")
		} else {
			result.lineage = append(result.lineage, edges...)
		}
	}

	if s.config.IncludeViews {
		result.lineage = append(result.lineage, viewLineage(dbName, relations, definitions, byKey)...)
	}

	return result, nil
}

func withoutViews(relations []relation) []relation {
	kept := relations[:0]
	for _, r := range relations {
		if !r.isView() {
			kept = append(kept, r)
		}
	}
	return kept
}

func listRelations(ctx context.Context, conn *pgx.Conn) ([]relation, error) {
	rows, err := conn.Query(ctx, `
		SELECT n.nspname,
		       c.relname,
		       c.relkind::text,
		       pg_catalog.obj_description(c.oid, 'pg_class'),
		       pg_catalog.pg_get_userbyid(c.relowner)
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p', 'f', 'v', 'm')
		  AND n.nspname NOT IN `+systemSchemas+`
		ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, fmt.Errorf("querying pg_class: %w", err)
	}
	defer rows.Close()

	var relations []relation
	for rows.Next() {
		var r relation
		var comment, owner sql.NullString
		if err := rows.Scan(&r.Schema, &r.Name, &r.Kind, &comment, &owner); err != nil {
			log.Warn().Err(err).Msg("Failed to scan relation row")
			continue
		}
		r.Comment = comment.String
		r.Owner = owner.String
		relations = append(relations, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating relation rows: %w", err)
	}
	return relations, nil
}

// listColumns reads every column in the database in one pass.
// information_schema.columns carries CockroachDB's own extensions: the
// native type name, the computed column expression and the hidden flag.
func listColumns(ctx context.Context, conn *pgx.Conn, dbName string) ([]columnRow, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_schema,
		       table_name,
		       column_name,
		       crdb_sql_type,
		       is_nullable = 'YES',
		       column_default,
		       generation_expression,
		       is_hidden = 'YES',
		       column_comment
		FROM information_schema.columns
		WHERE table_catalog = $1
		  AND table_schema NOT IN `+systemSchemas+`
		ORDER BY table_schema, table_name, ordinal_position`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	var columns []columnRow
	for rows.Next() {
		var c columnRow
		var columnDefault, generation, comment sql.NullString
		if err := rows.Scan(&c.Schema, &c.Table, &c.Name, &c.DataType, &c.Nullable,
			&columnDefault, &generation, &c.Hidden, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}
		if columnDefault.Valid {
			value := columnDefault.String
			c.Default = &value
		}
		c.Generation = generation.String
		c.Comment = comment.String
		columns = append(columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}
	return columns, nil
}

// listPrimaryKeys returns the set of "schema.table.column" that belong to a
// primary key.
func listPrimaryKeys(ctx context.Context, conn *pgx.Conn, dbName string) (map[string]struct{}, error) {
	rows, err := conn.Query(ctx, `
		SELECT kcu.table_schema, kcu.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_name = tc.constraint_name
		 AND kcu.table_schema = tc.table_schema
		 AND kcu.table_name = tc.table_name
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_catalog = $1`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying primary keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var schema, table, col string
		if err := rows.Scan(&schema, &table, &col); err != nil {
			log.Warn().Err(err).Msg("Failed to scan primary key row")
			continue
		}
		keys[schema+"."+table+"."+col] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating primary key rows: %w", err)
	}
	return keys, nil
}

// buildColumns groups column rows by relation, drops hidden columns and
// marks primary keys. Hidden columns are CockroachDB's own bookkeeping: the
// rowid it adds to a table without a primary key and the shard column of a
// hash-sharded index. Neither is something a user wrote or can select
// without naming it, so they are left out of the catalog.
func buildColumns(rows []columnRow, primaryKeys map[string]struct{}) map[string][]column {
	columns := make(map[string][]column)
	for _, row := range rows {
		if row.Hidden {
			continue
		}
		key := row.Schema + "." + row.Table
		_, isPrimaryKey := primaryKeys[key+"."+row.Name]

		c := column{
			Column: pluginsdk.Column{
				Name:        row.Name,
				DataType:    row.DataType,
				Nullable:    row.Nullable,
				PrimaryKey:  isPrimaryKey,
				Description: row.Comment,
			},
			GenerationExpression: row.Generation,
		}
		if row.Default != nil {
			c.Default = *row.Default
		}
		columns[key] = append(columns[key], c)
	}
	return columns
}

// listPartitions returns, per partitioned table, the columns its primary
// index is partitioned on. Sub-partitions and secondary index partitions are
// not reported.
func listPartitions(ctx context.Context, conn *pgx.Conn, dbName string) (map[string]*partition, error) {
	rows, err := conn.Query(ctx, `
		SELECT t.schema_name, t.name, p.column_names
		FROM crdb_internal.partitions p
		JOIN crdb_internal.tables t ON t.table_id = p.table_id
		JOIN crdb_internal.table_indexes i
		  ON i.descriptor_id = p.table_id
		 AND i.index_id = p.index_id
		 AND i.index_type = 'primary'
		WHERE t.database_name = $1
		  AND t.state = 'PUBLIC'
		  AND p.parent_name IS NULL`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying partitions: %w", err)
	}
	defer rows.Close()

	partitions := make(map[string]*partition)
	for rows.Next() {
		var schema, table, columnNames string
		if err := rows.Scan(&schema, &table, &columnNames); err != nil {
			log.Warn().Err(err).Msg("Failed to scan partition row")
			continue
		}
		key := schema + "." + table
		if _, seen := partitions[key]; !seen {
			partitions[key] = &partition{Columns: splitColumnNames(columnNames)}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating partition rows: %w", err)
	}
	return partitions, nil
}

// splitColumnNames splits the comma separated column list
// crdb_internal.partitions reports.
func splitColumnNames(columnNames string) []string {
	var columns []string
	for _, name := range strings.Split(columnNames, ",") {
		if name = strings.TrimSpace(name); name != "" {
			columns = append(columns, name)
		}
	}
	return columns
}

func listViewDefinitions(ctx context.Context, conn *pgx.Conn) (map[string]string, error) {
	rows, err := conn.Query(ctx, `
		SELECT schemaname, viewname, definition
		FROM pg_catalog.pg_views
		WHERE schemaname NOT IN `+systemSchemas+`
		UNION ALL
		SELECT schemaname, matviewname, definition
		FROM pg_catalog.pg_matviews
		WHERE schemaname NOT IN `+systemSchemas)
	if err != nil {
		return nil, fmt.Errorf("querying view definitions: %w", err)
	}
	defer rows.Close()

	definitions := make(map[string]string)
	for rows.Next() {
		var schema, name string
		var definition sql.NullString
		if err := rows.Scan(&schema, &name, &definition); err != nil {
			log.Warn().Err(err).Msg("Failed to scan view row")
			continue
		}
		if definition.Valid {
			definitions[schema+"."+name] = definition.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view rows: %w", err)
	}
	return definitions, nil
}

// listRowCounts reads the optimizer's row estimate for every table in the
// database. It comes from the statistics CockroachDB collects automatically
// (or from ANALYZE), so a brand new table reports zero until the first
// collection runs.
func listRowCounts(ctx context.Context, conn *pgx.Conn, dbName string) (map[string]int64, error) {
	rows, err := conn.Query(ctx, `
		SELECT t.schema_name, t.name, s.estimated_row_count
		FROM crdb_internal.table_row_statistics s
		JOIN crdb_internal.tables t ON t.table_id = s.table_id
		WHERE t.database_name = $1
		  AND t.state = 'PUBLIC'
		  AND s.estimated_row_count IS NOT NULL`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying row statistics: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var schema, table string
		var count int64
		if err := rows.Scan(&schema, &table, &count); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row statistics row")
			continue
		}
		counts[schema+"."+table] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating row statistics rows: %w", err)
	}
	return counts, nil
}

// listTableSizes reads the live data bytes of every table in the database
// in one span statistics call, the same call the DB Console's Databases
// page makes.
func listTableSizes(ctx context.Context, conn *pgx.Conn, dbName string) (map[string]int64, error) {
	rows, err := conn.Query(ctx, `
		SELECT t.schema_name, t.name, s.live_bytes
		FROM crdb_internal.tenant_span_stats(crdb_internal.get_database_id($1)) s
		JOIN crdb_internal.tables t ON t.table_id = s.table_id
		WHERE t.database_name = $1
		  AND t.state = 'PUBLIC'`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying span statistics: %w", err)
	}
	defer rows.Close()

	sizes := make(map[string]int64)
	for rows.Next() {
		var schema, table string
		var liveBytes sql.NullInt64
		if err := rows.Scan(&schema, &table, &liveBytes); err != nil {
			log.Warn().Err(err).Msg("Failed to scan span statistics row")
			continue
		}
		if liveBytes.Valid {
			sizes[schema+"."+table] = liveBytes.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating span statistics rows: %w", err)
	}
	return sizes, nil
}

// listColumnCounts counts the visible columns of every relation, so the
// column count statistic agrees with the columns shown in the schema.
func listColumnCounts(ctx context.Context, conn *pgx.Conn, dbName string) (map[string]int64, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_schema, table_name, count(*)
		FROM information_schema.columns
		WHERE table_catalog = $1
		  AND is_hidden = 'NO'
		  AND table_schema NOT IN `+systemSchemas+`
		GROUP BY table_schema, table_name`, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying column counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var schema, table string
		var count int64
		if err := rows.Scan(&schema, &table, &count); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column count row")
			continue
		}
		counts[schema+"."+table] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column count rows: %w", err)
	}
	return counts, nil
}

// listForeignKeys turns every foreign key into an edge from the referencing
// table to the referenced one. CockroachDB cannot reference a table in
// another database, so both ends are always in the relations just listed.
func listForeignKeys(ctx context.Context, conn *pgx.Conn, dbName string, known map[string]relation) ([]pluginsdk.LineageEdge, error) {
	rows, err := conn.Query(ctx, `
		SELECT ns.nspname, s.relname, nt.nspname, t.relname
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class s ON s.oid = con.conrelid
		JOIN pg_catalog.pg_namespace ns ON ns.oid = s.relnamespace
		JOIN pg_catalog.pg_class t ON t.oid = con.confrelid
		JOIN pg_catalog.pg_namespace nt ON nt.oid = t.relnamespace
		WHERE con.contype = 'f'
		LIMIT 1000`)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var edges []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for rows.Next() {
		var sourceSchema, sourceTable, targetSchema, targetTable string
		if err := rows.Scan(&sourceSchema, &sourceTable, &targetSchema, &targetTable); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		source, ok := known[sourceSchema+"."+sourceTable]
		if !ok {
			continue
		}
		target, ok := known[targetSchema+"."+targetTable]
		if !ok {
			continue
		}

		edge := pluginsdk.LineageEdge{
			Source: assetMRN(source.assetType(), qualifiedName(dbName, source.Schema, source.Name)),
			Target: assetMRN(target.assetType(), qualifiedName(dbName, target.Schema, target.Name)),
			Type:   "FOREIGN_KEY",
		}
		if edge.Source == edge.Target {
			continue
		}
		if _, dup := seen[edge.Source+":"+edge.Target]; dup {
			continue
		}
		seen[edge.Source+":"+edge.Target] = struct{}{}

		log.Debug().Str("source", edge.Source).Str("target", edge.Target).Msg("Found foreign key relationship")
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foreign key rows: %w", err)
	}
	return edges, nil
}

// viewLineage links every view to the relations its definition reads from.
// Edges run in the direction the data flows, base relation to view, the
// same way the MongoDB plugin draws its VIEW_OF edges.
func viewLineage(dbName string, relations []relation, definitions map[string]string, known map[string]relation) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, view := range relations {
		if !view.isView() {
			continue
		}
		definition, ok := definitions[view.key()]
		if !ok {
			continue
		}

		viewMRN := assetMRN(view.assetType(), qualifiedName(dbName, view.Schema, view.Name))

		for _, ref := range viewReferences(definition) {
			base, ok := resolveReference(ref, dbName, view.Schema, known)
			if !ok {
				log.Debug().Str("view", view.key()).Strs("reference", ref).Msg("Skipping VIEW_OF edge, reference not discovered")
				continue
			}

			edge := pluginsdk.LineageEdge{
				Source: assetMRN(base.assetType(), qualifiedName(dbName, base.Schema, base.Name)),
				Target: viewMRN,
				Type:   "VIEW_OF",
			}
			if edge.Source == edge.Target {
				continue
			}
			if _, dup := seen[edge.Source+":"+edge.Target]; dup {
				continue
			}
			seen[edge.Source+":"+edge.Target] = struct{}{}
			edges = append(edges, edge)
		}
	}
	return edges
}
