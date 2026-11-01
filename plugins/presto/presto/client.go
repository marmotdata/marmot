package presto

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	prestodriver "github.com/prestodb/presto-go-client/presto"
	"github.com/rs/zerolog/log"
)

// buildDSN renders the driver connection string. The password rides in
// the URL user info, which the driver turns into HTTP basic auth over
// HTTPS only; Validate refuses a password on plain HTTP for that reason.
func buildDSN(config *Config) string {
	u := url.URL{
		Scheme: "http",
		User:   url.User(config.User),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(config.Port)),
	}
	if config.Secure {
		u.Scheme = "https"
	}
	if config.Password != "" {
		u.User = url.UserPassword(config.User, config.Password)
	}

	// source shows up in the Presto query log, so admins can trace
	// discovery traffic back to Marmot.
	query := url.Values{}
	query.Set("source", "marmot")
	if config.SSLCertPath != "" {
		query.Set("SSLCertPath", config.SSLCertPath)
	}
	u.RawQuery = query.Encode()

	return u.String()
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	db, err := sql.Open("presto", buildDSN(s.config))
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return fmt.Errorf("pinging Presto: %w", err)
	}

	log.Debug().Str("host", s.config.Host).Int("port", s.config.Port).Msg("Connected to Presto")

	s.db = db
	return nil
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// rowsErr reports a real iteration error. The driver ends a result set
// with its own *presto.EOF instead of io.EOF, which database/sql then
// surfaces through rows.Err(), so that one is not an error here.
func rowsErr(rows *sql.Rows) error {
	return ignoreEOF(rows.Err())
}

func ignoreEOF(err error) error {
	var eof *prestodriver.EOF
	if errors.As(err, &eof) {
		return nil
	}
	return err
}

// quoteIdentifier double-quotes an identifier for Presto SQL.
func quoteIdentifier(id string) string {
	id = strings.ReplaceAll(id, "\x00", "")
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// qualifiedName renders catalog.schema.table with every part quoted.
func qualifiedName(catalog, schema, table string) string {
	return quoteIdentifier(catalog) + "." + quoteIdentifier(schema) + "." + quoteIdentifier(table)
}

// escapeString escapes a value for use inside a single-quoted SQL literal.
func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.ReplaceAll(s, "'", "''")
}
