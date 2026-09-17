package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	mssqldb "github.com/microsoft/go-mssqldb"
	"github.com/rs/zerolog/log"
)

// masterDatabase is where discovery starts when the config names no database.
// sys.databases is readable from any database, but a connection has to land
// somewhere, and every SQL Server has master.
const masterDatabase = "master"

// dsn builds the connection string for one database. The password is placed
// through url.URL so a password containing "@", "/" or "?" cannot break the
// URL apart, and it is never logged.
//
// The encrypt values are the driver's own: "true" requires TLS, "disable"
// turns TLS off entirely. The config's encrypt: false maps to "disable" rather
// than the driver's "false", because "false" still runs a TLS handshake for
// the login packet, which fails against the self-signed certificate a default
// SQL Server install presents.
func (s *Source) dsn(database string) string {
	encrypt := "disable"
	if s.config.Encrypt {
		encrypt = "true"
	}

	query := url.Values{}
	query.Set("database", database)
	query.Set("encrypt", encrypt)
	query.Set("trustservercertificate", strconv.FormatBool(s.config.TrustServerCertificate))
	query.Set("connection timeout", strconv.Itoa(s.config.ConnectTimeoutSeconds))
	if s.config.ApplicationIntent != "" {
		query.Set("applicationintent", s.config.ApplicationIntent)
	}

	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(s.config.User, s.config.Password),
		Host:     hostPort(s.config.Host, s.config.Port),
		RawQuery: query.Encode(),
	}
	return u.String()
}

// hostPort joins a host and port, bracketing the host when it is an IPv6 literal.
func hostPort(host string, port int) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port)
}

// connect opens a connection bound to one database. SQL Server binds a session
// to a single database context, so each database is discovered over its own
// connection.
func (s *Source) connect(ctx context.Context, database string) error {
	s.close()

	db, err := sql.Open("sqlserver", s.dsn(database))
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	// Discovery is a sequence of read-only catalog queries, so one connection
	// is enough and keeps the load on the server predictable.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(10 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.ConnectTimeoutSeconds)*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", database).
		Msg("Successfully connected to SQL Server")

	s.db = db
	return nil
}

func (s *Source) close() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// convertValue turns a value the driver returned into something that survives
// JSON encoding. dbType is the column's SQL Server type name, needed because
// both a uniqueidentifier (16 raw bytes) and a decimal (ASCII digits) arrive
// as []byte and cannot be told apart from the bytes alone.
func convertValue(value any, dbType string) any {
	switch v := value.(type) {
	case nil:
		return nil
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []byte:
		switch strings.ToUpper(dbType) {
		case "UNIQUEIDENTIFIER":
			var id mssqldb.UniqueIdentifier
			if err := id.Scan(v); err == nil {
				return id.String()
			}
			return fmt.Sprintf("0x%x", v)
		case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
			// The driver hands these back as their decimal text, which keeps
			// the exact value that a float64 would round away.
			return string(v)
		case "BINARY", "VARBINARY", "IMAGE", "TIMESTAMP", "ROWVERSION":
			return fmt.Sprintf("0x%x", v)
		}
		return string(v)
	default:
		return value
	}
}

// sqlVariantString reads an extended property value. Extended properties are
// stored as sql_variant, so the driver returns them as an untyped value that
// may be a string or the raw bytes behind one.
func sqlVariantString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

// sqlVariantInt reads an identity seed or increment, which are also stored as
// sql_variant and come back as whichever integer width the column uses.
func sqlVariantInt(value any) (int64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case int8:
		return int64(v), true
	case int:
		return int64(v), true
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
