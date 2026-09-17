package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// queryTimeout caps a single catalog query, so one slow view or a blocked
// system table cannot stall the whole run.
const queryTimeout = 30 * time.Second

// serverInfo is what the instance reports about itself. It is copied onto
// every database asset so a reader can tell which server an asset came from.
type serverInfo struct {
	ProductVersion string
	Edition        string
	// EngineEdition is the only reliable way to tell an Azure SQL Database
	// apart from a Managed Instance or a box install: 5 is Azure SQL
	// Database, 8 is Managed Instance, 9 is Azure SQL Edge.
	EngineEdition int
}

// databaseInfo is one row of sys.databases.
type databaseInfo struct {
	Name          string
	DatabaseID    int
	CreateDate    time.Time
	Collation     string
	State         string
	RecoveryModel string
	Owner         string
}

// schemaInfo is one schema, with its owner and MS_Description if it has one.
type schemaInfo struct {
	Name    string
	Owner   string
	Comment string
}

// objectInfo is one table or view.
type objectInfo struct {
	Schema     string
	Name       string
	TypeDesc   string
	CreateDate time.Time
	ModifyDate time.Time
	Comment    string
}

// columnInfo is one column, with the SQL Server extras that do not fit
// pluginsdk.Column.
type columnInfo struct {
	Schema             string
	Object             string
	Name               string
	TypeName           string
	MaxLength          int64
	Precision          int
	Scale              int
	Nullable           bool
	ColumnID           int
	ComputedDefinition string
	IsPersisted        bool
	IsIdentity         bool
	IdentitySeed       int64
	IdentityIncrement  int64
	Comment            string
	DefaultDefinition  string
	Collation          string
}

// foreignKeyInfo is one column pair of one foreign key constraint.
type foreignKeyInfo struct {
	Name         string
	Schema       string
	Table        string
	Column       string
	TargetSchema string
	TargetTable  string
	TargetColumn string
	DeleteAction string
	UpdateAction string
}

// routineInfo is one stored procedure or function.
type routineInfo struct {
	Schema      string
	Name        string
	TypeDesc    string
	CreateDate  time.Time
	ModifyDate  time.Time
	Definition  string
	IsEncrypted bool
}

// tableStats is the row count and on-disk size of one table.
type tableStats struct {
	Schema    string
	Table     string
	RowCount  int64
	SizeBytes int64
}

func (s *Source) queryServerInfo(ctx context.Context) (serverInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var info serverInfo
	row := s.db.QueryRowContext(queryCtx, `
		SELECT CAST(SERVERPROPERTY('ProductVersion') AS NVARCHAR(128)) AS product_version,
		       CAST(SERVERPROPERTY('Edition') AS NVARCHAR(128)) AS edition,
		       CAST(SERVERPROPERTY('EngineEdition') AS INT) AS engine_edition`)

	var productVersion, edition sql.NullString
	var engineEdition sql.NullInt64
	if err := row.Scan(&productVersion, &edition, &engineEdition); err != nil {
		return info, fmt.Errorf("querying server properties: %w", err)
	}

	info.ProductVersion = productVersion.String
	info.Edition = edition.String
	info.EngineEdition = int(engineEdition.Int64)
	return info, nil
}

// queryDatabases lists the databases this login can actually open. Offline and
// restoring databases are excluded by state, and HAS_DBACCESS drops the ones
// the login has no rights to, which would otherwise fail on connect.
func (s *Source) queryDatabases(ctx context.Context) ([]databaseInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT d.name, d.database_id, d.create_date, d.collation_name,
		       d.state_desc, d.recovery_model_desc, SUSER_SNAME(d.owner_sid) AS owner
		FROM sys.databases d
		WHERE d.state = 0 AND HAS_DBACCESS(d.name) = 1
		ORDER BY d.name`)
	if err != nil {
		return nil, fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	var databases []databaseInfo
	for rows.Next() {
		var (
			info                              databaseInfo
			collation, state, recovery, owner sql.NullString
		)
		if err := rows.Scan(&info.Name, &info.DatabaseID, &info.CreateDate,
			&collation, &state, &recovery, &owner); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}
		info.Collation = collation.String
		info.State = state.String
		info.RecoveryModel = recovery.String
		info.Owner = owner.String
		databases = append(databases, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating database rows: %w", err)
	}
	return databases, nil
}

func (s *Source) querySchemas(ctx context.Context) ([]schemaInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, p.name AS owner, ep.value AS comment
		FROM sys.schemas s
		LEFT JOIN sys.database_principals p ON p.principal_id = s.principal_id
		LEFT JOIN sys.extended_properties ep
		       ON ep.major_id = s.schema_id AND ep.minor_id = 0
		      AND ep.class = 3 AND ep.name = 'MS_Description'
		ORDER BY s.name`)
	if err != nil {
		return nil, fmt.Errorf("querying schemas: %w", err)
	}
	defer rows.Close()

	var schemas []schemaInfo
	for rows.Next() {
		var (
			info    schemaInfo
			owner   sql.NullString
			comment any
		)
		if err := rows.Scan(&info.Name, &owner, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan schema row")
			continue
		}
		info.Owner = owner.String
		info.Comment = sqlVariantString(comment)
		schemas = append(schemas, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schema rows: %w", err)
	}
	return schemas, nil
}

// queryObjects lists the user tables and views of the connected database.
// is_ms_shipped keeps out the objects SQL Server installs itself.
func (s *Source) queryObjects(ctx context.Context) ([]objectInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, o.name AS object_name, o.type_desc,
		       o.create_date, o.modify_date, ep.value AS comment
		FROM sys.objects o
		JOIN sys.schemas s ON s.schema_id = o.schema_id
		LEFT JOIN sys.extended_properties ep
		       ON ep.major_id = o.object_id AND ep.minor_id = 0
		      AND ep.class = 1 AND ep.name = 'MS_Description'
		WHERE o.type IN ('U', 'V') AND o.is_ms_shipped = 0
		ORDER BY s.name, o.name`)
	if err != nil {
		return nil, fmt.Errorf("querying objects: %w", err)
	}
	defer rows.Close()

	var objects []objectInfo
	for rows.Next() {
		var (
			info    objectInfo
			comment any
		)
		if err := rows.Scan(&info.Schema, &info.Name, &info.TypeDesc,
			&info.CreateDate, &info.ModifyDate, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan object row")
			continue
		}
		info.Comment = sqlVariantString(comment)
		objects = append(objects, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating object rows: %w", err)
	}
	return objects, nil
}

// queryViewDefinitions returns each view's SQL text, keyed by "schema.view".
func (s *Source) queryViewDefinitions(ctx context.Context) (map[string]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, v.name AS view_name, m.definition
		FROM sys.sql_modules m
		JOIN sys.views v ON v.object_id = m.object_id
		JOIN sys.schemas s ON s.schema_id = v.schema_id
		WHERE v.is_ms_shipped = 0`)
	if err != nil {
		return nil, fmt.Errorf("querying view definitions: %w", err)
	}
	defer rows.Close()

	definitions := make(map[string]string)
	for rows.Next() {
		var schemaName, viewName string
		var definition sql.NullString
		if err := rows.Scan(&schemaName, &viewName, &definition); err != nil {
			log.Warn().Err(err).Msg("Failed to scan view definition row")
			continue
		}
		if definition.Valid {
			definitions[schemaName+"."+viewName] = definition.String
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view definition rows: %w", err)
	}
	return definitions, nil
}

// queryColumns reads every column of every user table and view in one pass.
func (s *Source) queryColumns(ctx context.Context) ([]columnInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, o.name AS object_name, c.name AS column_name,
		       t.name AS type_name, c.max_length, c.precision, c.scale,
		       c.is_nullable, c.column_id,
		       cc.definition AS computed_definition, cc.is_persisted,
		       ic.seed_value, ic.increment_value,
		       ep.value AS comment, dc.definition AS default_definition,
		       c.collation_name
		FROM sys.columns c
		JOIN sys.objects o ON o.object_id = c.object_id
		JOIN sys.schemas s ON s.schema_id = o.schema_id
		JOIN sys.types t ON t.user_type_id = c.user_type_id
		LEFT JOIN sys.computed_columns cc
		       ON cc.object_id = c.object_id AND cc.column_id = c.column_id
		LEFT JOIN sys.identity_columns ic
		       ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		LEFT JOIN sys.extended_properties ep
		       ON ep.major_id = c.object_id AND ep.minor_id = c.column_id
		      AND ep.class = 1 AND ep.name = 'MS_Description'
		LEFT JOIN sys.default_constraints dc ON dc.object_id = c.default_object_id
		WHERE o.type IN ('U', 'V') AND o.is_ms_shipped = 0
		ORDER BY s.name, o.name, c.column_id`)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var (
			info                            columnInfo
			computed, defaultDef, collation sql.NullString
			persisted                       sql.NullBool
			seed, increment, comment        any
			maxLength                       int64
			precision, scale                int
		)
		if err := rows.Scan(&info.Schema, &info.Object, &info.Name, &info.TypeName,
			&maxLength, &precision, &scale, &info.Nullable, &info.ColumnID,
			&computed, &persisted, &seed, &increment, &comment, &defaultDef,
			&collation); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		info.MaxLength = maxLength
		info.Precision = precision
		info.Scale = scale
		info.ComputedDefinition = computed.String
		info.IsPersisted = persisted.Bool
		info.Comment = sqlVariantString(comment)
		info.DefaultDefinition = defaultDef.String
		info.Collation = collation.String

		// A row in sys.identity_columns is what makes a column an identity;
		// the seed is the useful part of it.
		if seedValue, ok := sqlVariantInt(seed); ok {
			info.IsIdentity = true
			info.IdentitySeed = seedValue
		}
		if incrementValue, ok := sqlVariantInt(increment); ok {
			info.IsIdentity = true
			info.IdentityIncrement = incrementValue
		}

		columns = append(columns, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}
	return columns, nil
}

// queryPrimaryKeyColumns returns the columns covered by a primary key,
// keyed by "schema.object.column".
func (s *Source) queryPrimaryKeyColumns(ctx context.Context) (map[string]bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, o.name AS object_name, c.name AS column_name
		FROM sys.key_constraints kc
		JOIN sys.objects o ON o.object_id = kc.parent_object_id
		JOIN sys.schemas s ON s.schema_id = o.schema_id
		JOIN sys.indexes i
		  ON i.object_id = kc.parent_object_id AND i.index_id = kc.unique_index_id
		JOIN sys.index_columns ixc
		  ON ixc.object_id = i.object_id AND ixc.index_id = i.index_id
		JOIN sys.columns c
		  ON c.object_id = ixc.object_id AND c.column_id = ixc.column_id
		WHERE kc.type = 'PK' AND o.is_ms_shipped = 0`)
	if err != nil {
		return nil, fmt.Errorf("querying primary keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]bool)
	for rows.Next() {
		var schemaName, objectName, columnName string
		if err := rows.Scan(&schemaName, &objectName, &columnName); err != nil {
			log.Warn().Err(err).Msg("Failed to scan primary key row")
			continue
		}
		keys[schemaName+"."+objectName+"."+columnName] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating primary key rows: %w", err)
	}
	return keys, nil
}

func (s *Source) queryForeignKeys(ctx context.Context) ([]foreignKeyInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT fk.name AS constraint_name,
		       OBJECT_SCHEMA_NAME(fk.parent_object_id) AS source_schema,
		       OBJECT_NAME(fk.parent_object_id) AS source_table,
		       pc.name AS source_column,
		       OBJECT_SCHEMA_NAME(fk.referenced_object_id) AS target_schema,
		       OBJECT_NAME(fk.referenced_object_id) AS target_table,
		       rc.name AS target_column,
		       fk.delete_referential_action_desc,
		       fk.update_referential_action_desc
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fkc.constraint_object_id = fk.object_id
		JOIN sys.columns pc
		  ON pc.object_id = fkc.parent_object_id AND pc.column_id = fkc.parent_column_id
		JOIN sys.columns rc
		  ON rc.object_id = fkc.referenced_object_id AND rc.column_id = fkc.referenced_column_id
		WHERE fk.is_ms_shipped = 0
		ORDER BY fk.name, fkc.constraint_column_id`)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var keys []foreignKeyInfo
	for rows.Next() {
		var (
			info                       foreignKeyInfo
			deleteAction, updateAction sql.NullString
		)
		if err := rows.Scan(&info.Name, &info.Schema, &info.Table, &info.Column,
			&info.TargetSchema, &info.TargetTable, &info.TargetColumn,
			&deleteAction, &updateAction); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}
		info.DeleteAction = deleteAction.String
		info.UpdateAction = updateAction.String
		keys = append(keys, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foreign key rows: %w", err)
	}
	return keys, nil
}

func (s *Source) queryRoutines(ctx context.Context) ([]routineInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, o.name AS object_name, o.type_desc,
		       o.create_date, o.modify_date, m.definition,
		       OBJECTPROPERTY(o.object_id, 'IsEncrypted') AS is_encrypted
		FROM sys.objects o
		JOIN sys.schemas s ON s.schema_id = o.schema_id
		LEFT JOIN sys.sql_modules m ON m.object_id = o.object_id
		WHERE o.type IN ('P', 'FN', 'IF', 'TF') AND o.is_ms_shipped = 0
		ORDER BY s.name, o.name`)
	if err != nil {
		return nil, fmt.Errorf("querying routines: %w", err)
	}
	defer rows.Close()

	var routines []routineInfo
	for rows.Next() {
		var (
			info       routineInfo
			definition sql.NullString
			encrypted  sql.NullInt64
		)
		if err := rows.Scan(&info.Schema, &info.Name, &info.TypeDesc,
			&info.CreateDate, &info.ModifyDate, &definition, &encrypted); err != nil {
			log.Warn().Err(err).Msg("Failed to scan routine row")
			continue
		}
		info.Definition = definition.String
		info.IsEncrypted = encrypted.Int64 == 1
		routines = append(routines, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating routine rows: %w", err)
	}
	return routines, nil
}

// queryTableStats reads row counts and on-disk sizes from the allocation
// metadata, which costs nothing compared to counting rows.
//
// The two totals are computed in separate subqueries on purpose. A table with
// a MAX or large column has several allocation units per partition, so joining
// partitions to allocation units in one query would count its rows once per
// unit.
func (s *Source) queryTableStats(ctx context.Context) ([]tableStats, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT s.name AS schema_name, o.name AS object_name,
		       rc.row_count, sz.size_bytes
		FROM sys.objects o
		JOIN sys.schemas s ON s.schema_id = o.schema_id
		CROSS APPLY (
		    SELECT SUM(p.rows) AS row_count
		    FROM sys.partitions p
		    WHERE p.object_id = o.object_id AND p.index_id IN (0, 1)
		) rc
		CROSS APPLY (
		    SELECT SUM(a.total_pages) * 8 * 1024 AS size_bytes
		    FROM sys.partitions p
		    JOIN sys.allocation_units a ON a.container_id = p.partition_id
		    WHERE p.object_id = o.object_id AND p.index_id IN (0, 1)
		) sz
		WHERE o.type = 'U' AND o.is_ms_shipped = 0`)
	if err != nil {
		return nil, fmt.Errorf("querying table statistics: %w", err)
	}
	defer rows.Close()

	var stats []tableStats
	for rows.Next() {
		var (
			info                tableStats
			rowCount, sizeBytes sql.NullInt64
		)
		if err := rows.Scan(&info.Schema, &info.Table, &rowCount, &sizeBytes); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table statistics row")
			continue
		}
		info.RowCount = rowCount.Int64
		info.SizeBytes = sizeBytes.Int64
		stats = append(stats, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table statistics rows: %w", err)
	}
	return stats, nil
}
