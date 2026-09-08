package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	go_ora "github.com/sijms/go-ora/v2"
)

const queryTimeout = 30 * time.Second

// defaultBlockSize is what Oracle ships with; it is only used when
// v$parameter cannot be read.
const defaultBlockSize = 8192

// buildConnectionURL renders the go-ora connection string. BuildUrl escapes
// the user and password, so a password with @ or / cannot break the URL.
func buildConnectionURL(c *Config) string {
	options := map[string]string{
		"CONNECT TIMEOUT": "15",
	}
	if c.SID != "" {
		options["SID"] = c.SID
	}
	if c.SSL {
		options["SSL"] = "true"
		options["SSL VERIFY"] = strconv.FormatBool(c.SSLVerify)
	}
	if c.WalletPath != "" {
		options["WALLET"] = c.WalletPath
	}
	return go_ora.BuildUrl(c.Host, c.Port, c.ServiceName, c.User, c.Password, options)
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	db, err := sql.Open("oracle", buildConnectionURL(s.config))
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("service_name", s.config.ServiceName).
		Str("sid", s.config.SID).
		Msg("Successfully connected to Oracle")

	s.db = db
	return nil
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// dict names a data dictionary view. ALL_* views list what the connected
// user can access; DBA_* views list everything but need SELECT ANY
// DICTIONARY.
func (s *Source) dict(view string) string {
	if s.config.UseDBAViews {
		return "DBA_" + view
	}
	return "ALL_" + view
}

type serverInfo struct {
	Version     string
	DBName      string
	Container   string
	ServiceName string
	BlockSize   int64
}

// serverInfo reads what the database says about itself. None of it is
// required for discovery, so every part that cannot be read is logged and
// left empty.
func (s *Source) serverInfo(ctx context.Context) serverInfo {
	info := serverInfo{BlockSize: defaultBlockSize}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// banner_full arrived in 18c, banner exists everywhere but v$ views need
	// a grant, and product_component_version is readable by any user.
	versionQueries := []string{
		"SELECT banner_full FROM v$version",
		"SELECT banner FROM v$version",
		"SELECT version FROM product_component_version WHERE product LIKE 'Oracle%'",
	}
	for _, query := range versionQueries {
		var version string
		if err := s.db.QueryRowContext(queryCtx, query).Scan(&version); err != nil {
			log.Debug().Err(err).Str("query", query).Msg("Version query not available")
			continue
		}
		info.Version = strings.TrimSpace(version)
		break
	}
	if info.Version == "" {
		log.Warn().Msg("Could not determine the Oracle version")
	}

	var dbName, container, serviceName sql.NullString
	err := s.db.QueryRowContext(queryCtx,
		"SELECT sys_context('USERENV','DB_NAME'), sys_context('USERENV','CON_NAME'), sys_context('USERENV','SERVICE_NAME') FROM dual",
	).Scan(&dbName, &container, &serviceName)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read database name")
	}
	info.DBName = dbName.String
	info.Container = container.String
	info.ServiceName = serviceName.String

	var blockSize sql.NullInt64
	err = s.db.QueryRowContext(queryCtx, "SELECT value FROM v$parameter WHERE name = 'db_block_size'").Scan(&blockSize)
	if err != nil || !blockSize.Valid || blockSize.Int64 <= 0 {
		log.Debug().Err(err).Msg("Block size not readable, assuming the default")
	} else {
		info.BlockSize = blockSize.Int64
	}

	return info
}

type schemaRow struct {
	Name    string
	Created sql.NullTime
}

// listSchemas returns the schemas to discover: the configured list, or every
// user that Oracle does not maintain itself, minus the exclude list.
func (s *Source) listSchemas(ctx context.Context) ([]schemaRow, error) {
	users, err := s.listUsers(ctx)
	if err != nil {
		return nil, err
	}

	excluded := make(map[string]struct{}, len(s.config.ExcludeSchemas))
	for _, name := range s.config.ExcludeSchemas {
		excluded[name] = struct{}{}
	}

	if len(s.config.Schemas) > 0 {
		byName := make(map[string]schemaRow, len(users))
		for _, u := range users {
			byName[u.Name] = u.schemaRow
		}
		var schemas []schemaRow
		for _, name := range s.config.Schemas {
			if _, skip := excluded[name]; skip {
				log.Debug().Str("schema", name).Msg("Configured schema is on the exclude list, skipping")
				continue
			}
			if u, ok := byName[name]; ok {
				schemas = append(schemas, u)
			} else {
				schemas = append(schemas, schemaRow{Name: name})
			}
		}
		return schemas, nil
	}

	var schemas []schemaRow
	for _, u := range users {
		if u.oracleMaintained {
			continue
		}
		if _, skip := excluded[u.Name]; skip {
			continue
		}
		schemas = append(schemas, u.schemaRow)
	}
	return schemas, nil
}

type userRow struct {
	schemaRow
	oracleMaintained bool
}

// listUsers reads ALL_USERS. ORACLE_MAINTAINED arrived in 12c; on older
// releases the column is missing and the exclude list alone decides.
func (s *Source) listUsers(ctx context.Context) ([]userRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	users, err := s.scanUsers(queryCtx, fmt.Sprintf("SELECT username, oracle_maintained, created FROM %s ORDER BY username", s.dict("USERS")), true)
	if err == nil {
		return users, nil
	}
	if !isInvalidIdentifier(err) {
		return nil, err
	}
	log.Debug().Msg("ORACLE_MAINTAINED is not available, relying on the exclude list")
	return s.scanUsers(queryCtx, fmt.Sprintf("SELECT username, created FROM %s ORDER BY username", s.dict("USERS")), false)
}

func (s *Source) scanUsers(ctx context.Context, query string, withMaintained bool) ([]userRow, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying users: %w", err)
	}
	defer rows.Close()

	var users []userRow
	for rows.Next() {
		var u userRow
		var maintained sql.NullString
		if withMaintained {
			err = rows.Scan(&u.Name, &maintained, &u.Created)
		} else {
			err = rows.Scan(&u.Name, &u.Created)
		}
		if err != nil {
			return nil, fmt.Errorf("scanning user row: %w", err)
		}
		u.oracleMaintained = maintained.String == "Y"
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user rows: %w", err)
	}
	return users, nil
}

type tableRow struct {
	Owner        string
	Name         string
	Tablespace   sql.NullString
	NumRows      sql.NullInt64
	Blocks       sql.NullInt64
	LastAnalyzed sql.NullTime
	Partitioned  sql.NullString
	Temporary    sql.NullString
	IOTType      sql.NullString
	Compression  sql.NullString
}

// listTables returns the schema's heap and index-organized tables. Overflow
// segments of index-organized tables, nested table storage and tables in
// the recycle bin are implementation details, not objects users query.
// Tables backing materialized views are returned too; the caller separates
// them using the materialized view list.
func (s *Source) listTables(ctx context.Context, owner string) ([]tableRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT owner, table_name, tablespace_name, num_rows, blocks, last_analyzed,
		       partitioned, temporary, iot_type, compression
		FROM %s
		WHERE owner = :1
		  AND (iot_type IS NULL OR iot_type = 'IOT')
		  AND nested = 'NO'
		  AND table_name NOT LIKE 'BIN$%%'
		ORDER BY table_name`, s.dict("TABLES"))

	rows, err := s.db.QueryContext(queryCtx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var tables []tableRow
	for rows.Next() {
		var t tableRow
		if err := rows.Scan(&t.Owner, &t.Name, &t.Tablespace, &t.NumRows, &t.Blocks, &t.LastAnalyzed,
			&t.Partitioned, &t.Temporary, &t.IOTType, &t.Compression); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table row")
			continue
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}
	return tables, nil
}

type viewRow struct {
	Owner      string
	Name       string
	TextLength int64
	Text       string
}

// listViews returns the schema's views with their SQL. TEXT is a LONG
// column, which some drivers cannot read; when it comes back empty the
// VARCHAR2 copy TEXT_VC (12c+) is used, and failing that DBMS_METADATA.
func (s *Source) listViews(ctx context.Context, owner string) ([]viewRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	views, err := s.scanViews(queryCtx, fmt.Sprintf(
		"SELECT owner, view_name, text_length, text_vc, text FROM %s WHERE owner = :1 ORDER BY view_name", s.dict("VIEWS")), owner)
	if err != nil && isInvalidIdentifier(err) {
		log.Debug().Msg("TEXT_VC is not available, reading view text from the LONG column only")
		views, err = s.scanViews(queryCtx, fmt.Sprintf(
			"SELECT owner, view_name, text_length, NULL, text FROM %s WHERE owner = :1 ORDER BY view_name", s.dict("VIEWS")), owner)
	}
	if err != nil {
		return nil, err
	}

	for i := range views {
		if views[i].Text != "" {
			continue
		}
		text, err := s.objectDDL(ctx, "VIEW", views[i].Owner, views[i].Name)
		if err != nil {
			log.Warn().Err(err).Str("view", objectName(views[i].Owner, views[i].Name)).Msg("Failed to read view text")
			continue
		}
		views[i].Text = text
	}
	return views, nil
}

func (s *Source) scanViews(ctx context.Context, query, owner string) ([]viewRow, error) {
	rows, err := s.db.QueryContext(ctx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying views: %w", err)
	}
	defer rows.Close()

	var views []viewRow
	for rows.Next() {
		var v viewRow
		var textLength sql.NullInt64
		var textVC, text sql.NullString
		if err := rows.Scan(&v.Owner, &v.Name, &textLength, &textVC, &text); err != nil {
			log.Warn().Err(err).Msg("Failed to scan view row")
			continue
		}
		v.TextLength = textLength.Int64
		// The LONG column holds the complete text; TEXT_VC is cut at 4000
		// characters, so it only stands in when the LONG read gave nothing.
		v.Text = text.String
		if v.Text == "" {
			v.Text = textVC.String
		}
		views = append(views, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view rows: %w", err)
	}
	return views, nil
}

// objectDDL asks DBMS_METADATA for an object's definition. It needs
// SELECT_CATALOG_ROLE for objects in other schemas, so it is only a fallback.
func (s *Source) objectDDL(ctx context.Context, objectType, owner, name string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var ddl sql.NullString
	err := s.db.QueryRowContext(queryCtx, "SELECT DBMS_METADATA.GET_DDL(:1, :2, :3) FROM dual", objectType, name, owner).Scan(&ddl)
	if err != nil {
		return "", fmt.Errorf("reading DDL: %w", err)
	}
	return strings.TrimSpace(ddl.String), nil
}

type mviewRow struct {
	Owner         string
	Name          string
	Query         string
	RefreshMode   sql.NullString
	RefreshMethod sql.NullString
	BuildMode     sql.NullString
	LastRefresh   sql.NullTime
	Staleness     sql.NullString
}

func (s *Source) listMaterializedViews(ctx context.Context, owner string) ([]mviewRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT owner, mview_name, query, refresh_mode, refresh_method, build_mode, last_refresh_date, staleness
		FROM %s
		WHERE owner = :1
		ORDER BY mview_name`, s.dict("MVIEWS"))

	rows, err := s.db.QueryContext(queryCtx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying materialized views: %w", err)
	}
	defer rows.Close()

	var mviews []mviewRow
	for rows.Next() {
		var mv mviewRow
		var text sql.NullString
		if err := rows.Scan(&mv.Owner, &mv.Name, &text, &mv.RefreshMode, &mv.RefreshMethod, &mv.BuildMode, &mv.LastRefresh, &mv.Staleness); err != nil {
			log.Warn().Err(err).Msg("Failed to scan materialized view row")
			continue
		}
		mv.Query = text.String
		mviews = append(mviews, mv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating materialized view rows: %w", err)
	}

	for i := range mviews {
		if mviews[i].Query != "" {
			continue
		}
		text, err := s.objectDDL(ctx, "MATERIALIZED_VIEW", mviews[i].Owner, mviews[i].Name)
		if err != nil {
			log.Warn().Err(err).Str("materialized_view", objectName(mviews[i].Owner, mviews[i].Name)).Msg("Failed to read materialized view query")
			continue
		}
		mviews[i].Query = text
	}
	return mviews, nil
}

// listComments returns table comments keyed by table and column comments
// keyed by table then column.
func (s *Source) listComments(ctx context.Context, owner string) (map[string]string, map[string]map[string]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	tableComments := make(map[string]string)
	rows, err := s.db.QueryContext(queryCtx,
		fmt.Sprintf("SELECT table_name, comments FROM %s WHERE owner = :1 AND comments IS NOT NULL", s.dict("TAB_COMMENTS")), owner)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table comments: %w", err)
	}
	for rows.Next() {
		var table string
		var comment sql.NullString
		if err := rows.Scan(&table, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table comment row")
			continue
		}
		if comment.Valid && comment.String != "" {
			tableComments[table] = comment.String
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating table comment rows: %w", err)
	}

	columnComments := make(map[string]map[string]string)
	rows, err = s.db.QueryContext(queryCtx,
		fmt.Sprintf("SELECT table_name, column_name, comments FROM %s WHERE owner = :1 AND comments IS NOT NULL", s.dict("COL_COMMENTS")), owner)
	if err != nil {
		return tableComments, nil, fmt.Errorf("querying column comments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, col string
		var comment sql.NullString
		if err := rows.Scan(&table, &col, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column comment row")
			continue
		}
		if !comment.Valid || comment.String == "" {
			continue
		}
		if columnComments[table] == nil {
			columnComments[table] = make(map[string]string)
		}
		columnComments[table][col] = comment.String
	}
	if err := rows.Err(); err != nil {
		return tableComments, columnComments, fmt.Errorf("iterating column comment rows: %w", err)
	}
	return tableComments, columnComments, nil
}

type columnRow struct {
	Table      string
	Name       string
	DataType   string
	DataLength sql.NullInt64
	CharLength sql.NullInt64
	CharUsed   sql.NullString
	Precision  sql.NullInt64
	Scale      sql.NullInt64
	Nullable   string
	Default    sql.NullString
	Virtual    string
	Identity   string
}

// listColumns returns the schema's columns keyed by table, in column order.
// DATA_DEFAULT is a LONG column; if the driver cannot read it the columns
// are read again without defaults rather than lost.
func (s *Source) listColumns(ctx context.Context, owner string) (map[string][]columnRow, error) {
	columns, err := s.scanColumns(ctx, owner, true)
	if err == nil {
		return columns, nil
	}
	log.Warn().Err(err).Str("schema", owner).Msg("Failed to read column defaults, retrying without them")
	return s.scanColumns(ctx, owner, false)
}

func (s *Source) scanColumns(ctx context.Context, owner string, withDefaults bool) (map[string][]columnRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	defaultColumn := "data_default"
	if !withDefaults {
		defaultColumn = "NULL"
	}
	query := fmt.Sprintf(`
		SELECT table_name, column_name, data_type, data_length, char_length, char_used,
		       data_precision, data_scale, nullable, %s, virtual_column, identity_column
		FROM %s
		WHERE owner = :1 AND hidden_column = 'NO'
		ORDER BY table_name, column_id`, defaultColumn, s.dict("TAB_COLS"))

	rows, err := s.db.QueryContext(queryCtx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	columns := make(map[string][]columnRow)
	for rows.Next() {
		var c columnRow
		var nullable, virtual, identity sql.NullString
		if err := rows.Scan(&c.Table, &c.Name, &c.DataType, &c.DataLength, &c.CharLength, &c.CharUsed,
			&c.Precision, &c.Scale, &nullable, &c.Default, &virtual, &identity); err != nil {
			return nil, fmt.Errorf("scanning column row: %w", err)
		}
		c.Nullable = nullable.String
		c.Virtual = virtual.String
		c.Identity = identity.String
		columns[c.Table] = append(columns[c.Table], c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}
	return columns, nil
}

type constraintRow struct {
	Owner       string
	Table       string
	Name        string
	Type        string
	Column      string
	Position    sql.NullInt64
	ROwner      sql.NullString
	RConstraint sql.NullString
	DeleteRule  sql.NullString
	Status      sql.NullString
}

// listConstraints returns one row per column of every primary key, unique
// and foreign key constraint in the schema.
func (s *Source) listConstraints(ctx context.Context, owner string) ([]constraintRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT c.owner, c.table_name, c.constraint_name, c.constraint_type, cc.column_name, cc.position,
		       c.r_owner, c.r_constraint_name, c.delete_rule, c.status
		FROM %s c
		JOIN %s cc
		  ON cc.owner = c.owner AND cc.constraint_name = c.constraint_name AND cc.table_name = c.table_name
		WHERE c.owner = :1 AND c.constraint_type IN ('P', 'R', 'U')
		ORDER BY c.constraint_name, cc.position`, s.dict("CONSTRAINTS"), s.dict("CONS_COLUMNS"))

	rows, err := s.db.QueryContext(queryCtx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying constraints: %w", err)
	}
	defer rows.Close()

	var constraints []constraintRow
	for rows.Next() {
		var c constraintRow
		if err := rows.Scan(&c.Owner, &c.Table, &c.Name, &c.Type, &c.Column, &c.Position,
			&c.ROwner, &c.RConstraint, &c.DeleteRule, &c.Status); err != nil {
			log.Warn().Err(err).Msg("Failed to scan constraint row")
			continue
		}
		constraints = append(constraints, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating constraint rows: %w", err)
	}
	return constraints, nil
}

type procedureRow struct {
	Owner   string
	Name    string
	Type    string
	Status  string
	Created sql.NullTime
	LastDDL sql.NullTime
}

// listProcedures returns the schema's standalone procedures, functions and
// packages. ALL_OBJECTS only lists ones the user may execute.
func (s *Source) listProcedures(ctx context.Context, owner string) ([]procedureRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT owner, object_name, object_type, status, created, last_ddl_time
		FROM %s
		WHERE owner = :1 AND object_type IN ('PROCEDURE', 'FUNCTION', 'PACKAGE')
		ORDER BY object_name`, s.dict("OBJECTS"))

	rows, err := s.db.QueryContext(queryCtx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("querying procedures: %w", err)
	}
	defer rows.Close()

	var procedures []procedureRow
	for rows.Next() {
		var p procedureRow
		var status sql.NullString
		if err := rows.Scan(&p.Owner, &p.Name, &p.Type, &status, &p.Created, &p.LastDDL); err != nil {
			log.Warn().Err(err).Msg("Failed to scan procedure row")
			continue
		}
		p.Status = status.String
		procedures = append(procedures, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating procedure rows: %w", err)
	}
	return procedures, nil
}

// isInvalidIdentifier reports ORA-00904, which is what Oracle answers when a
// query names a dictionary column this release does not have.
func isInvalidIdentifier(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ORA-00904")
}

// isSyntaxError reports the errors an Oracle release raises for SQL it does
// not understand (ORA-00933 before 23c, ORA-03048/03049 from 23c on).
func isSyntaxError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "ORA-00933") || strings.Contains(msg, "ORA-03048") || strings.Contains(msg, "ORA-03049")
}
