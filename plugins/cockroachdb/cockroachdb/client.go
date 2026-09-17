package cockroachdb

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// connect opens one connection to the given database. CockroachDB cannot
// switch databases on an open connection, so discovery connects once per
// database it visits.
func (s *Source) connect(ctx context.Context, database string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(s.connString(database))
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	config.ConnectTimeout = 15 * time.Second
	config.RuntimeParams["application_name"] = "marmot"

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connecting: %w", err)
	}

	// Bound every query so one slow catalog read cannot eat the whole
	// discovery budget.
	if _, err := conn.Exec(ctx, "SET statement_timeout = '30s'"); err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("setting statement timeout: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", database).
		Msg("Connected to CockroachDB")

	return conn, nil
}

// connString builds a libpq-style URL. Going through url.URL keeps a
// password with reserved characters from breaking the parse.
func (s *Source) connString(database string) string {
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)),
		Path:   "/" + database,
	}
	if s.config.Password != "" {
		u.User = url.UserPassword(s.config.User, s.config.Password)
	} else {
		u.User = url.User(s.config.User)
	}

	query := url.Values{}
	query.Set("sslmode", s.config.SSLMode)
	if s.config.SSLRootCert != "" {
		query.Set("sslrootcert", s.config.SSLRootCert)
	}
	if s.config.SSLCert != "" {
		query.Set("sslcert", s.config.SSLCert)
	}
	if s.config.SSLKey != "" {
		query.Set("sslkey", s.config.SSLKey)
	}
	u.RawQuery = query.Encode()

	return u.String()
}

// tableIdent quotes database.schema.table for interpolation into SQL that
// cannot bind an identifier as a parameter.
func tableIdent(database, schema, table string) string {
	return pgx.Identifier{database, schema, table}.Sanitize()
}

// convertValue turns the Go values pgx hands back into ones that survive
// JSON encoding on the way to the Marmot UI.
func convertValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case []byte:
		return fmt.Sprintf("\\x%x", v)
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16])
	case time.Time:
		return v.Format(time.RFC3339)
	case map[string]interface{}:
		return v
	case []interface{}:
		converted := make([]interface{}, len(v))
		for i, item := range v {
			converted[i] = convertValue(item)
		}
		return converted
	case driver.Valuer:
		// DECIMAL, INTERVAL and the other pgtype structs render as their
		// text form rather than as a struct dump.
		out, err := v.Value()
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return convertValue(out)
	case fmt.Stringer:
		return v.String()
	default:
		return val
	}
}
