package starrocks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
)

const (
	connectTimeout = 15 * time.Second
	queryTimeout   = 30 * time.Second
)

// connect opens the pool and pins a single connection for the run. SET
// CATALOG is session state, so every query has to go through the same
// connection or an external catalog silently falls back to the default.
func (s *Source) connect(ctx context.Context) error {
	s.close()

	cfg := mysql.NewConfig()
	cfg.User = s.config.User
	cfg.Passwd = s.config.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	cfg.TLSConfig = s.config.TLS
	cfg.Timeout = connectTimeout
	cfg.ParseTime = true
	// Client-side interpolation keeps discovery off the binary prepared
	// statement path, which StarRocks only partly supports.
	cfg.InterpolateParams = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}
	db.SetMaxOpenConns(1)

	connCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	conn, err := db.Conn(connCtx)
	if err != nil {
		db.Close()
		return fmt.Errorf("connecting: %w", err)
	}
	if err := conn.PingContext(connCtx); err != nil {
		conn.Close()
		db.Close()
		return fmt.Errorf("pinging frontend: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Msg("Connected to StarRocks")

	s.db = db
	s.conn = conn
	return nil
}

func (s *Source) close() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// useCatalog switches the pinned connection to the configured catalog.
// The default catalog needs no switch.
func (s *Source) useCatalog(ctx context.Context) error {
	if s.config.Catalog == defaultCatalog {
		return nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	_, err := s.conn.ExecContext(queryCtx, "SET CATALOG "+quoteIdentifier(s.config.Catalog))
	return err
}

// showRows runs a SHOW statement and returns its column names and every
// row as strings. SHOW output differs between StarRocks versions, so
// callers pick columns by name instead of position.
func (s *Source) showRows(ctx context.Context, query string) ([]string, [][]sql.NullString, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("reading columns: %w", err)
	}

	var out [][]sql.NullString
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, fmt.Errorf("scanning row: %w", err)
		}
		out = append(out, values)
	}
	return columns, out, rows.Err()
}

// showValue returns the value at the named column of a SHOW row, or "" when
// the column is absent or NULL.
func showValue(columns []string, row []sql.NullString, name string) string {
	for i, c := range columns {
		if strings.EqualFold(c, name) && i < len(row) {
			return row[i].String
		}
	}
	return ""
}

func (s *Source) serverVersion(ctx context.Context) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var version string
	if err := s.conn.QueryRowContext(queryCtx, "SELECT current_version()").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

type catalogInfo struct {
	Name string
	Type string
}

func (s *Source) listCatalogs(ctx context.Context) ([]catalogInfo, error) {
	columns, rows, err := s.showRows(ctx, "SHOW CATALOGS")
	if err != nil {
		return nil, err
	}
	var catalogs []catalogInfo
	for _, row := range rows {
		catalogs = append(catalogs, catalogInfo{
			Name: showValue(columns, row, "Catalog"),
			Type: showValue(columns, row, "Type"),
		})
	}
	return catalogs, nil
}

func (s *Source) listDatabases(ctx context.Context) ([]string, error) {
	_, rows, err := s.showRows(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	var databases []string
	for _, row := range rows {
		if len(row) > 0 && row[0].Valid {
			databases = append(databases, row[0].String)
		}
	}
	return databases, nil
}

// tableInfo is one row of information_schema.tables.
type tableInfo struct {
	Name       string
	TableType  string
	Engine     string
	Comment    string
	Created    sql.NullTime
	Updated    sql.NullTime
	Rows       sql.NullInt64
	DataLength sql.NullInt64
}

func (s *Source) listTables(ctx context.Context, database string) ([]tableInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT TABLE_NAME, TABLE_TYPE, ENGINE, TABLE_COMMENT,
		       CREATE_TIME, UPDATE_TIME, TABLE_ROWS, DATA_LENGTH
		FROM information_schema.tables
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`
	rows, err := s.conn.QueryContext(queryCtx, query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []tableInfo
	for rows.Next() {
		var t tableInfo
		var tableType, engine, comment sql.NullString
		if err := rows.Scan(&t.Name, &tableType, &engine, &comment, &t.Created, &t.Updated, &t.Rows, &t.DataLength); err != nil {
			return nil, fmt.Errorf("scanning table row: %w", err)
		}
		t.TableType = tableType.String
		t.Engine = engine.String
		t.Comment = comment.String
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// materializedViewInfo is one row of information_schema.materialized_views,
// the authoritative listing of asynchronous materialized views.
type materializedViewInfo struct {
	Name             string
	RefreshType      string
	IsActive         string
	PartitionType    string
	TaskName         string
	LastRefreshState string
	LastRefreshStart sql.NullTime
	Definition       string
	// BaseTables are the catalog.db.table names StarRocks tracks refresh
	// versions for, which is the exact set the view reads from.
	BaseTables []string
}

func (s *Source) listMaterializedViews(ctx context.Context, database string) (map[string]materializedViewInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT TABLE_NAME, REFRESH_TYPE, IS_ACTIVE, PARTITION_TYPE, TASK_NAME,
		       LAST_REFRESH_STATE, LAST_REFRESH_START_TIME,
		       MATERIALIZED_VIEW_DEFINITION, BASE_TABLE_REFRESH_VERSION_TIMES
		FROM information_schema.materialized_views
		WHERE TABLE_SCHEMA = ?
	`
	rows, err := s.conn.QueryContext(queryCtx, query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make(map[string]materializedViewInfo)
	for rows.Next() {
		var mv materializedViewInfo
		var refreshType, isActive, partitionType, taskName, state, definition, baseTables sql.NullString
		if err := rows.Scan(&mv.Name, &refreshType, &isActive, &partitionType, &taskName, &state, &mv.LastRefreshStart, &definition, &baseTables); err != nil {
			return nil, fmt.Errorf("scanning materialized view row: %w", err)
		}
		mv.RefreshType = refreshType.String
		mv.IsActive = isActive.String
		mv.PartitionType = partitionType.String
		mv.TaskName = taskName.String
		mv.LastRefreshState = state.String
		mv.Definition = definition.String
		mv.BaseTables = parseBaseTables(baseTables.String)
		views[mv.Name] = mv
	}
	return views, rows.Err()
}

// parseBaseTables reads the keys of a BASE_TABLE_REFRESH_VERSION_TIMES
// value, a JSON object keyed by qualified table name.
func parseBaseTables(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var versions map[string]any
	if err := json.Unmarshal([]byte(value), &versions); err != nil {
		return nil
	}
	var names []string
	for name := range versions {
		names = append(names, name)
	}
	return names
}

// partitionCounts returns how many partitions each table in the database
// has, from information_schema.partitions_meta. One query covers the
// whole database, where SHOW PARTITIONS would take one per table.
func (s *Source) partitionCounts(ctx context.Context, database string) (map[string]int, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT TABLE_NAME, COUNT(*)
		FROM information_schema.partitions_meta
		WHERE DB_NAME = ?
		GROUP BY TABLE_NAME
	`
	rows, err := s.conn.QueryContext(queryCtx, query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("scanning partition row: %w", err)
		}
		counts[name] = count
	}
	return counts, rows.Err()
}

// showCreate returns the DDL printed by SHOW CREATE <kind> db.name. The
// result's column names differ per kind (Create Table, Create View,
// Create Materialized View), so the DDL is taken by position.
func (s *Source) showCreate(ctx context.Context, kind, database, name string) (string, error) {
	query := fmt.Sprintf("SHOW CREATE %s %s.%s", kind, quoteIdentifier(database), quoteIdentifier(name))
	_, rows, err := s.showRows(ctx, query)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 || len(rows[0]) < 2 {
		return "", fmt.Errorf("no DDL returned")
	}
	return rows[0][1].String, nil
}

// columnInfo is one row of SHOW FULL COLUMNS.
type columnInfo struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default sql.NullString
	Extra   string
	Comment string
}

func (s *Source) showColumns(ctx context.Context, database, name string) ([]columnInfo, error) {
	query := fmt.Sprintf("SHOW FULL COLUMNS FROM %s.%s", quoteIdentifier(database), quoteIdentifier(name))
	columns, rows, err := s.showRows(ctx, query)
	if err != nil {
		return nil, err
	}

	defaultIdx := -1
	for i, c := range columns {
		if strings.EqualFold(c, "Default") {
			defaultIdx = i
		}
	}

	var out []columnInfo
	for _, row := range rows {
		c := columnInfo{
			Field:   showValue(columns, row, "Field"),
			Type:    showValue(columns, row, "Type"),
			Null:    showValue(columns, row, "Null"),
			Key:     showValue(columns, row, "Key"),
			Extra:   showValue(columns, row, "Extra"),
			Comment: showValue(columns, row, "Comment"),
		}
		if defaultIdx >= 0 && defaultIdx < len(row) {
			c.Default = row[defaultIdx]
		}
		out = append(out, c)
	}
	return out, nil
}

// foreignKeyRow is one referencing column from
// information_schema.key_column_usage.
type foreignKeyRow struct {
	Table            string
	ReferencedSchema string
	ReferencedTable  string
}

func (s *Source) listForeignKeys(ctx context.Context, database string) ([]foreignKeyRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := `
		SELECT TABLE_NAME, REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME
		FROM information_schema.key_column_usage
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		LIMIT 1000
	`
	rows, err := s.conn.QueryContext(queryCtx, query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []foreignKeyRow
	for rows.Next() {
		var table string
		var refSchema, refTable sql.NullString
		if err := rows.Scan(&table, &refSchema, &refTable); err != nil {
			return nil, fmt.Errorf("scanning foreign key row: %w", err)
		}
		if !refTable.Valid {
			continue
		}
		keys = append(keys, foreignKeyRow{Table: table, ReferencedSchema: refSchema.String, ReferencedTable: refTable.String})
	}
	return keys, rows.Err()
}

// quoteIdentifier wraps an identifier in backticks for StarRocks SQL.
func quoteIdentifier(id string) string {
	id = strings.ReplaceAll(id, "\x00", "")
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}
