package questdb

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// queryTimeout bounds each query so one slow table function cannot eat the
// whole discovery budget.
const queryTimeout = 30 * time.Second

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Built as a url.URL rather than by string formatting so a password
	// containing "@", "/" or "%" is escaped instead of breaking the DSN.
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(s.config.User, s.config.Password),
		Host:     net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)),
		Path:     "/" + s.config.Database,
		RawQuery: "sslmode=" + s.config.SSLMode,
	}

	config, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return fmt.Errorf("parsing connection string: %w", err)
	}

	config.MaxConns = 2
	config.MinConns = 1
	config.MaxConnLifetime = 2 * time.Minute
	config.MaxConnIdleTime = 30 * time.Second

	// QuestDB does not announce standard_conforming_strings, which pgx's
	// simple protocol insists on, so queries go over the extended protocol
	// without a prepared statement cache. Nothing is bound as a parameter:
	// QuestDB cannot infer the type of a parameter passed to its table
	// functions, so table names are inlined as quoted literals instead.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(timeoutCtx, config)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
		pool.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", s.config.Database).
		Msg("Successfully connected to QuestDB")

	s.pool = pool
	return nil
}

func (s *Source) closeConnection() {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
}

// row is one result row keyed by column name. QuestDB's table functions gain
// and lose columns between releases, so callers read the columns they know
// by name instead of scanning by position.
type row map[string]any

// queryRows runs sql and returns every row keyed by column name.
func (s *Source) queryRows(ctx context.Context, sql string) ([]row, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()

	var result []row
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}
		r := make(row, len(fields))
		for i, field := range fields {
			r[field.Name] = values[i]
		}
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r row) has(key string) bool {
	_, ok := r[key]
	return ok
}

func (r row) str(key string) string {
	switch v := r[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func (r row) boolean(key string) bool {
	v, _ := r[key].(bool)
	return v
}

// integer reads any of the integer widths pgx decodes QuestDB's INT and
// LONG columns into. Missing, null and non-numeric values read as 0.
func (r row) integer(key string) int64 {
	switch v := r[key].(type) {
	case int:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func (r row) timestamp(key string) (time.Time, bool) {
	v, ok := r[key].(time.Time)
	return v, ok
}

// quoteIdent double-quotes a table name so names containing spaces, dots or
// keywords interpolate safely into a FROM clause.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteLiteral single-quotes a string for use as a table function argument.
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// isUnsupportedFunction reports whether err is QuestDB saying a table
// function does not exist, which is how an older build answers a call to
// views() or materialized_views().
func isUnsupportedFunction(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown function") ||
		strings.Contains(msg, "function does not exist")
}
