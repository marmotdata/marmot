package hive

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/beltran/gohive"
)

// client wraps one HiveServer2 session. A gohive connection is not safe for
// concurrent use, and discovery runs its statements one after another.
type client struct {
	conn *gohive.Connection
}

func connect(cfg *Config) (*client, error) {
	conf := gohive.NewConnectConfiguration()
	conf.Username = cfg.Username
	conf.Password = cfg.Password
	conf.Service = cfg.KerberosServiceName
	conf.TransportMode = cfg.Transport
	conf.HTTPPath = cfg.HTTPPath
	conf.ConnectTimeout = 15 * time.Second
	conf.SocketTimeout = 30 * time.Second
	conf.HttpTimeout = 30 * time.Second
	// SELECT * would otherwise name every column table.column.
	conf.HiveConfiguration = map[string]string{
		"hive.resultset.use.unique.column.names": "false",
	}
	if cfg.SSL {
		conf.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.SSLSkipVerify, //nolint:gosec // opt-in via ssl_skip_verify
		}
	}

	conn, err := dial(cfg.Host, cfg.Port, driverAuth(cfg), conf)
	if err != nil {
		return nil, err
	}
	return &client{conn: conn}, nil
}

// driverAuth translates the configured mechanism into the one gohive
// expects for the transport. Over HTTP the driver only knows NONE, which
// sends the credentials as basic auth, and KERBEROS, so LDAP and NOSASL
// fold into NONE there.
func driverAuth(cfg *Config) string {
	if cfg.Transport == "http" && cfg.Auth != "KERBEROS" {
		return "NONE"
	}
	return cfg.Auth
}

// dial turns the panics gohive raises for a mechanism it cannot serve, such
// as KERBEROS in a binary built without the kerberos tag, into errors.
func dial(host string, port int, auth string, conf *gohive.ConnectConfiguration) (conn *gohive.Connection, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s authentication is not available in this build: %v", auth, r)
		}
	}()
	return gohive.Connect(host, port, auth, conf)
}

func (c *client) close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// query runs one statement and returns the column names in result order
// with every row, NULL as nil. Results are catalog metadata or a 20-row
// sample, so they are read whole.
func (c *client) query(ctx context.Context, statement string) ([]string, [][]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cursor := c.conn.Cursor()
	defer cursor.Close()

	cursor.Exec(ctx, statement)
	if cursor.Err != nil {
		return nil, nil, cursor.Err
	}

	var columns []string
	for _, d := range cursor.Description() {
		columns = append(columns, d[0])
	}
	if cursor.Err != nil {
		return nil, nil, fmt.Errorf("reading result schema: %w", cursor.Err)
	}

	var rows [][]any
	for cursor.HasMore(ctx) {
		row := cursor.RowMap(ctx)
		if cursor.Err != nil {
			return nil, nil, cursor.Err
		}
		values := make([]any, len(columns))
		for i, name := range columns {
			values[i] = row[name]
		}
		rows = append(rows, values)
	}
	if cursor.Err != nil {
		return nil, nil, cursor.Err
	}

	return columns, rows, nil
}

// text runs a statement whose result is read as text: DESCRIBE, SHOW and
// friends. NULL cells become "".
func (c *client) text(ctx context.Context, statement string) ([]string, [][]string, error) {
	columns, rows, err := c.query(ctx, statement)
	if err != nil {
		return nil, nil, err
	}
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		for j, v := range row {
			out[i][j] = cellString(v)
		}
	}
	return columns, out, nil
}

// firstColumn runs a statement and returns its first column, for the SHOW
// commands that list names.
func (c *client) firstColumn(ctx context.Context, statement string) ([]string, error) {
	_, rows, err := c.text(ctx, statement)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) > 0 && row[0] != "" {
			names = append(names, row[0])
		}
	}
	return names, nil
}

func (c *client) version(ctx context.Context) (string, error) {
	_, rows, err := c.text(ctx, "SELECT VERSION()")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		return "", nil
	}
	return hiveVersion(rows[0][0]), nil
}

func (c *client) databases(ctx context.Context) ([]string, error) {
	return c.firstColumn(ctx, "SHOW DATABASES")
}

// databaseInfo is what DESCRIBE DATABASE EXTENDED reports, keyed by the
// result column names so older servers that return fewer columns still parse.
type databaseInfo struct {
	Comment    string
	Location   string
	Owner      string
	OwnerType  string
	Parameters map[string]string
}

func (c *client) describeDatabase(ctx context.Context, database string) (*databaseInfo, error) {
	columns, rows, err := c.text(ctx, "DESCRIBE DATABASE EXTENDED "+quoteIdent(database))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no rows returned")
	}
	byName := make(map[string]string, len(columns))
	for i, name := range columns {
		if i < len(rows[0]) {
			byName[strings.ToLower(name)] = rows[0][i]
		}
	}
	return &databaseInfo{
		Comment:    byName["comment"],
		Location:   byName["location"],
		Owner:      byName["owner_name"],
		OwnerType:  byName["owner_type"],
		Parameters: parseDatabaseParameters(byName["parameters"]),
	}, nil
}

func (c *client) tables(ctx context.Context, database string) ([]string, error) {
	return c.firstColumn(ctx, "SHOW TABLES IN "+quoteIdent(database))
}

func (c *client) views(ctx context.Context, database string) ([]string, error) {
	return c.firstColumn(ctx, "SHOW VIEWS IN "+quoteIdent(database))
}

func (c *client) materializedViews(ctx context.Context, database string) ([]string, error) {
	return c.firstColumn(ctx, "SHOW MATERIALIZED VIEWS IN "+quoteIdent(database))
}

func (c *client) describeFormatted(ctx context.Context, database, table string) ([][]string, error) {
	_, rows, err := c.text(ctx, "DESCRIBE FORMATTED "+qualifiedIdent(database, table))
	return rows, err
}

func (c *client) partitions(ctx context.Context, database, table string) ([]string, error) {
	return c.firstColumn(ctx, "SHOW PARTITIONS "+qualifiedIdent(database, table))
}

// showCreate returns the DDL as one string. Servers differ on whether the
// statement comes back as one multi-line row or one row per line.
func (c *client) showCreate(ctx context.Context, database, table string) (string, error) {
	lines, err := c.firstColumn(ctx, "SHOW CREATE TABLE "+qualifiedIdent(database, table))
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func cellString(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprint(v)
	}
}

// quoteIdent backtick-quotes a HiveQL identifier so names that are keywords
// or carry unusual characters interpolate safely. HiveServer2 has no
// parameter binding for identifiers.
func quoteIdent(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func qualifiedIdent(database, table string) string {
	return quoteIdent(database) + "." + quoteIdent(table)
}

// hiveVersion keeps the version number out of SELECT VERSION(), which also
// carries the build's git revision ("4.0.1 r3af4517...").
func hiveVersion(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// parseDatabaseParameters reads the "{team=data, owner=bi}" form DESCRIBE
// DATABASE EXTENDED uses, which is a Java map's toString. A value holding a
// comma splits wrongly; Hive gives no better shape over HiveServer2.
func parseDatabaseParameters(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if raw == "" {
		return nil
	}
	params := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		params[key] = strings.TrimSpace(value)
	}
	if len(params) == 0 {
		return nil
	}
	return params
}
