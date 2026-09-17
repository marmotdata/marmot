package timescale

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// queryTimeout bounds a single catalog query so one slow read cannot use up
// the whole discovery budget.
const queryTimeout = 30 * time.Second

// internalSchemas are the schemas TimescaleDB keeps its own bookkeeping in.
// Chunks, materialization hypertables and the catalog views live here: they
// are implementation detail, and a single busy hypertable puts thousands of
// chunk tables in _timescaledb_internal.
var internalSchemas = []string{
	"_timescaledb_cache",
	"_timescaledb_catalog",
	"_timescaledb_config",
	"_timescaledb_internal",
	"timescaledb_experimental",
	"timescaledb_information",
}

// isInternalSchema reports whether a schema holds PostgreSQL or TimescaleDB
// internals rather than user objects.
func isInternalSchema(schema string) bool {
	if strings.HasPrefix(schema, "pg_") || schema == "information_schema" {
		return true
	}
	for _, s := range internalSchemas {
		if schema == s {
			return true
		}
	}
	return false
}

// schemaFilterSQL builds the WHERE fragment that keeps internal schemas out of
// a catalog query, for the schema name in the column alias. The names are a
// fixed list in this file, never user input, so interpolating them is safe.
func schemaFilterSQL(column string) string {
	quoted := make([]string, 0, len(internalSchemas))
	for _, s := range internalSchemas {
		quoted = append(quoted, "'"+s+"'")
	}
	return fmt.Sprintf("%s !~ '^pg_' AND %s NOT IN (%s)", column, column, strings.Join(quoted, ", "))
}

// objectKind is how PostgreSQL classifies a relation, kept as the string the
// asset metadata reports.
type objectKind string

const (
	kindTable            objectKind = "table"
	kindView             objectKind = "view"
	kindMaterializedView objectKind = "materialized_view"
)

// column is a discovered column. It embeds the SDK's canonical shape so the
// schema view renders it, and adds the two PostgreSQL attributes that change
// how a column is written to.
type column struct {
	pluginsdk.Column
	Identity  string `json:"identity,omitempty"`
	Generated bool   `json:"is_generated,omitempty"`
}

// object is one discovered table or view before the TimescaleDB catalog is
// layered on top of it.
type object struct {
	Database    string
	Schema      string
	Name        string
	Kind        objectKind
	Owner       string
	Comment     string
	Definition  string
	RowEstimate int64
	SizeBytes   int64
	Columns     []column
}

// key addresses an object within one database, matching the
// hypertable_schema/hypertable_name pairs the TimescaleDB catalog reports.
func (o object) key() string {
	return o.Schema + "." + o.Name
}

func (o object) metadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"database":    o.Database,
		"schema":      o.Schema,
		"table_name":  o.Name,
		"object_type": string(o.Kind),
		"owner":       o.Owner,
	}
	if o.Comment != "" {
		metadata["comment"] = o.Comment
	}
	return metadata
}

func (s *Source) connect(ctx context.Context, database string) error {
	s.disconnect()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		s.config.User,
		s.config.Password,
		s.config.Host,
		s.config.Port,
		database,
		s.config.SSLMode,
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parsing connection string: %w", err)
	}

	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = 2 * time.Minute
	config.MaxConnIdleTime = 30 * time.Second

	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.ConnConfig.RuntimeParams["statement_timeout"] = "30000"

	p, err := pgxpool.NewWithConfig(timeoutCtx, config)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	if err := p.Ping(timeoutCtx); err != nil {
		p.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", database).
		Msg("Connected to TimescaleDB")

	s.pool = p
	return nil
}

func (s *Source) disconnect() {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
}

// discoverDatabases lists the databases this run should read, one asset each.
func (s *Source) discoverDatabases(ctx context.Context) ([]pluginsdk.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// pg_database_size raises an error for a database the user cannot
	// connect to, so it is only called where the privilege check passes.
	query := `
		SELECT
			d.datname,
			pg_catalog.pg_get_userbyid(d.datdba) AS owner,
			CASE WHEN pg_catalog.has_database_privilege(d.datname, 'CONNECT')
			     THEN pg_catalog.pg_database_size(d.oid) END AS size,
			pg_catalog.shobj_description(d.oid, 'pg_database') AS comment,
			pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
			d.datcollate,
			d.datctype,
			d.datconnlimit
		FROM pg_catalog.pg_database d
		WHERE d.datistemplate = false
		  AND d.datallowconn = true
		ORDER BY d.datname
	`

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	excluded := make(map[string]struct{}, len(s.config.ExcludeDatabases))
	for _, name := range s.config.ExcludeDatabases {
		excluded[name] = struct{}{}
	}

	var assets []pluginsdk.Asset

	for rows.Next() {
		var (
			name, owner, encoding, collate, ctype string
			size                                  *int64
			comment                               *string
			connectionLimit                       int
		)

		if err := rows.Scan(&name, &owner, &size, &comment, &encoding, &collate, &ctype, &connectionLimit); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}

		if s.config.Database != "" && name != s.config.Database {
			continue
		}
		if _, skip := excluded[name]; skip {
			log.Debug().Str("database", name).Msg("Skipping excluded database")
			continue
		}

		metadata := map[string]interface{}{
			"host":             s.config.Host,
			"port":             s.config.Port,
			"database":         name,
			"owner":            owner,
			"encoding":         encoding,
			"collate":          collate,
			"ctype":            ctype,
			"connection_limit": connectionLimit,
		}
		if size != nil {
			metadata["size"] = *size
		}
		if comment != nil && *comment != "" {
			metadata["comment"] = *comment
		}

		mrnValue := assetMRN("Database", name)
		dbName := name

		assets = append(assets, pluginsdk.Asset{
			Name:      &dbName,
			MRN:       &mrnValue,
			Type:      "Database",
			Providers: []string{provider},
			Metadata:  metadata,
			Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating database rows: %w", err)
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered databases")
	return assets, nil
}

// discoverObjects reads the tables, views and materialized views of the
// connected database, with their columns when configured.
func (s *Source) discoverObjects(ctx context.Context, dbName string) ([]object, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// relispartition drops the child partitions of a declaratively
	// partitioned table: the parent already represents them.
	query := `
		SELECT
			n.nspname,
			c.relname,
			CASE c.relkind
				WHEN 'r' THEN 'table'
				WHEN 'p' THEN 'table'
				WHEN 'v' THEN 'view'
				WHEN 'm' THEN 'materialized_view'
			END AS object_type,
			pg_catalog.pg_get_userbyid(c.relowner) AS owner,
			c.reltuples::bigint AS row_estimate,
			COALESCE(pg_catalog.obj_description(c.oid, 'pg_class'), '') AS comment,
			pg_catalog.pg_total_relation_size(c.oid) AS size_bytes,
			CASE WHEN c.relkind IN ('v', 'm')
			     THEN COALESCE(pg_catalog.pg_get_viewdef(c.oid, true), '')
			     ELSE '' END AS definition
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p', 'v', 'm')
		  AND NOT c.relispartition
		  AND n.nspname <> 'information_schema'
	`
	if s.config.ExcludeSystemSchemas {
		query += " AND " + schemaFilterSQL("n.nspname")
	}
	query += " ORDER BY n.nspname, c.relname"

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables and views: %w", err)
	}
	defer rows.Close()

	var objects []object

	for rows.Next() {
		obj := object{Database: dbName}
		var kind string

		if err := rows.Scan(
			&obj.Schema, &obj.Name, &kind, &obj.Owner,
			&obj.RowEstimate, &obj.Comment, &obj.SizeBytes, &obj.Definition,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan object row")
			continue
		}

		obj.Kind = objectKind(kind)
		obj.Definition = strings.TrimSpace(obj.Definition)
		objects = append(objects, obj)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating object rows: %w", err)
	}

	log.Debug().Int("count", len(objects)).Str("database", dbName).Msg("Discovered tables and views")

	if s.config.IncludeColumns && len(objects) > 0 {
		columns, err := s.discoverColumns(ctx)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover columns")
		} else {
			for i := range objects {
				objects[i].Columns = columns[objects[i].key()]
			}
		}
	}

	return objects, nil
}

// discoverColumns reads every column of every user object in one pass, keyed
// by schema-qualified object name.
func (s *Source) discoverColumns(ctx context.Context) (map[string][]column, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// attidentity is '' for a plain column, 'a' for GENERATED ALWAYS AS
	// IDENTITY and 'd' for BY DEFAULT; attgenerated is 's' for a stored
	// generated column. Both are "char", cast to text so they scan as
	// strings.
	query := `
		SELECT
			n.nspname,
			c.relname,
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
			NOT a.attnotnull AS is_nullable,
			COALESCE(pg_catalog.pg_get_expr(ad.adbin, ad.adrelid), '') AS column_default,
			COALESCE(pk.is_primary_key, false) AS is_primary_key,
			COALESCE(pg_catalog.col_description(a.attrelid, a.attnum), '') AS comment,
			a.attidentity::text AS identity,
			a.attgenerated::text AS generated
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		LEFT JOIN LATERAL (
			SELECT true AS is_primary_key
			FROM pg_catalog.pg_constraint con
			WHERE con.conrelid = a.attrelid
			  AND con.contype = 'p'
			  AND a.attnum = ANY (con.conkey)
		) pk ON true
		WHERE a.attnum > 0
		  AND NOT a.attisdropped
		  AND c.relkind IN ('r', 'p', 'v', 'm')
		  AND NOT c.relispartition
		  AND n.nspname <> 'information_schema'
	`
	if s.config.ExcludeSystemSchemas {
		query += " AND " + schemaFilterSQL("n.nspname")
	}
	query += " ORDER BY n.nspname, c.relname, a.attnum"

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]column)

	for rows.Next() {
		var (
			schema, table            string
			col                      column
			columnDefault, generated string
			comment                  string
		)

		if err := rows.Scan(
			&schema, &table, &col.Name, &col.DataType, &col.Nullable,
			&columnDefault, &col.PrimaryKey, &comment, &col.Identity, &generated,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		if columnDefault != "" {
			col.Default = columnDefault
		}
		if comment != "" {
			col.Description = comment
		}
		col.Generated = generated != ""

		key := schema + "." + table
		result[key] = append(result[key], col)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return result, nil
}

// discoverForeignKeys turns every foreign key constraint into an edge from
// the referencing table to the referenced one. It reads pg_constraint rather
// than information_schema because TimescaleDB copies a hypertable's
// constraints onto every chunk, and the chunks are filtered out here by
// schema.
func (s *Source) discoverForeignKeys(ctx context.Context) ([]pluginsdk.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT
			sn.nspname AS source_schema,
			sc.relname AS source_table,
			tn.nspname AS target_schema,
			tc.relname AS target_table
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class sc ON sc.oid = con.conrelid
		JOIN pg_catalog.pg_namespace sn ON sn.oid = sc.relnamespace
		JOIN pg_catalog.pg_class tc ON tc.oid = con.confrelid
		JOIN pg_catalog.pg_namespace tn ON tn.oid = tc.relnamespace
		WHERE con.contype = 'f'
	`
	if s.config.ExcludeSystemSchemas {
		query += " AND " + schemaFilterSQL("sn.nspname")
		query += " AND " + schemaFilterSQL("tn.nspname")
	}
	query += " LIMIT 1000"

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var lineages []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for rows.Next() {
		var sourceSchema, sourceTable, targetSchema, targetTable string
		if err := rows.Scan(&sourceSchema, &sourceTable, &targetSchema, &targetTable); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		sourceMRN := assetMRN("Table", sourceTable)
		targetMRN := assetMRN("Table", targetTable)
		if sourceMRN == targetMRN {
			continue
		}

		key := sourceMRN + ":" + targetMRN
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		log.Debug().
			Str("source", sourceSchema+"."+sourceTable).
			Str("target", targetSchema+"."+targetTable).
			Msg("Found foreign key relationship")

		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: sourceMRN,
			Target: targetMRN,
			Type:   "FOREIGN_KEY",
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foreign key rows: %w", err)
	}

	return lineages, nil
}

// columnCounts counts the columns of every user object, so statistics do not
// depend on include_columns being on.
func (s *Source) columnCounts(ctx context.Context) map[string]int64 {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT n.nspname, c.relname, count(*)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE a.attnum > 0
		  AND NOT a.attisdropped
		  AND c.relkind IN ('r', 'p', 'v', 'm')
		  AND NOT c.relispartition
		  AND n.nspname <> 'information_schema'
	`
	if s.config.ExcludeSystemSchemas {
		query += " AND " + schemaFilterSQL("n.nspname")
	}
	query += " GROUP BY 1, 2"

	counts := make(map[string]int64)

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to count columns")
		return counts
	}
	defer rows.Close()

	for rows.Next() {
		var schema, table string
		var count int64
		if err := rows.Scan(&schema, &table, &count); err != nil {
			continue
		}
		counts[schema+"."+table] = count
	}

	return counts
}

// collectStatistics emits row, column and size metrics for every object. A
// hypertable's parent table holds no rows and no data of its own, so
// reltuples and pg_total_relation_size both report near zero for it; where
// the TimescaleDB catalog has better numbers they replace the PostgreSQL
// ones, and the chunk count is added.
func (s *Source) collectStatistics(ctx context.Context, objects []object, ts *timescaleInfo) []pluginsdk.Statistic {
	return buildStatistics(objects, ts, s.columnCounts(ctx))
}

// buildStatistics assembles the metrics for one database from the numbers
// already read out of the two catalogs.
func buildStatistics(objects []object, ts *timescaleInfo, counts map[string]int64) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	for _, obj := range objects {
		assetType := "Table"
		if obj.Kind != kindTable {
			assetType = "View"
		}
		assetMRNValue := assetMRN(assetType, obj.Name)

		rowCount := float64(obj.RowEstimate)
		sizeBytes := float64(obj.SizeBytes)

		if ts != nil {
			if size, ok := ts.sizes[obj.key()]; ok {
				if size.Rows != nil {
					rowCount = float64(*size.Rows)
				}
				if size.Bytes != nil {
					sizeBytes = float64(*size.Bytes)
				}
			}
			if ht, ok := ts.hypertables[obj.key()]; ok {
				statistics = append(statistics, pluginsdk.Statistic{
					AssetMRN:   assetMRNValue,
					MetricName: "asset.chunk_count",
					Value:      float64(ht.NumChunks),
				})
			}
		}

		statistics = append(statistics,
			pluginsdk.Statistic{AssetMRN: assetMRNValue, MetricName: "asset.row_count", Value: rowCount},
			pluginsdk.Statistic{AssetMRN: assetMRNValue, MetricName: "asset.size_bytes", Value: sizeBytes},
		)

		if count, ok := counts[obj.key()]; ok {
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   assetMRNValue,
				MetricName: "asset.column_count",
				Value:      float64(count),
			})
		}
	}

	return statistics
}

// FetchSampleData implements pluginsdk.DataFetcher, reading a preview of the
// rows behind one asset.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}

	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)
	s.config = config

	database, _ := a.Metadata["database"].(string)
	schema, _ := a.Metadata["schema"].(string)
	table, _ := a.Metadata["table_name"].(string)
	if table == "" && a.Name != nil {
		table = *a.Name
	}

	if database == "" {
		return nil, nil, fmt.Errorf("could not determine database from asset metadata")
	}
	if schema == "" {
		return nil, nil, fmt.Errorf("could not determine schema from asset metadata")
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	if err := s.connect(fetchCtx, database); err != nil {
		return nil, nil, fmt.Errorf("connecting to database %s: %w", database, err)
	}
	defer s.disconnect()

	query := fmt.Sprintf(
		"SELECT * FROM %s.%s LIMIT 20",
		pgx.Identifier{schema}.Sanitize(),
		pgx.Identifier{table}.Sanitize(),
	)

	log.Debug().Str("database", database).Str("table", schema+"."+table).Msg("Fetching sample data")

	rows, err := s.pool.Query(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columnNames := make([]string, len(fields))
	for i, f := range fields {
		columnNames[i] = f.Name
	}

	var dataRows [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to read row, skipping")
			continue
		}

		converted := make([]interface{}, len(values))
		for i, v := range values {
			converted[i] = convertValue(v)
		}
		dataRows = append(dataRows, converted)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	return columnNames, dataRows, nil
}

// convertValue turns the PostgreSQL-specific values pgx returns into shapes
// that survive JSON encoding to the Marmot host.
func convertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16])
	case []byte:
		return fmt.Sprintf("\\x%x", v)
	case time.Time:
		return v.Format(time.RFC3339)
	case []interface{}:
		converted := make([]interface{}, len(v))
		for i, item := range v {
			converted[i] = convertValue(item)
		}
		return converted
	default:
		return value
	}
}
