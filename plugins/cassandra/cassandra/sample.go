package cassandra

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"gopkg.in/inf.v0"
)

// FetchSampleData implements pluginsdk.DataFetcher: the first 20 rows of a
// table or materialized view, with driver types rendered as JSON-friendly
// values.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	keyspace, table := sampleTarget(a)
	if keyspace == "" || table == "" {
		return nil, nil, fmt.Errorf("could not determine keyspace and table from asset")
	}

	session, err := connect(s.config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to Cassandra: %w", err)
	}
	defer session.Close()

	fetchCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 20", quoteIdent(keyspace), quoteIdent(table))

	log.Debug().Str("keyspace", keyspace).Str("table", table).Msg("Fetching sample data")

	iter := session.Query(query).WithContext(fetchCtx).Iter()
	names, rows, err := scanRows(iter)
	if err != nil {
		return nil, nil, fmt.Errorf("querying %s.%s: %w", keyspace, table, err)
	}

	log.Debug().Int("columns", len(names)).Int("rows", len(rows)).Msg("Successfully fetched sample data")

	return names, rows, nil
}

// sampleTarget finds the keyspace and table a sample belongs to. Metadata
// is authoritative; the qualified asset name is the fallback for an asset
// that reached the host through another route.
func sampleTarget(a *pluginsdk.Asset) (keyspace, table string) {
	if a.Metadata != nil {
		keyspace, _ = a.Metadata["keyspace"].(string)
		table, _ = a.Metadata["table_name"].(string)
	}
	if (keyspace == "" || table == "") && a.Name != nil {
		if ks, t, ok := strings.Cut(*a.Name, "."); ok {
			keyspace, table = ks, t
		}
	}
	return keyspace, table
}

// scanRows reads every row of the iterator into column-ordered values.
// Each destination is a pointer to a pointer of the column's Go type: gocql
// leaves the inner pointer nil for a CQL null, where a plain destination
// would silently read as a zero value and show an empty uuid or "" in the
// preview. Tuple columns are what gocql splits into one entry per element,
// so they are reassembled into a list.
func scanRows(iter *gocql.Iter) ([]string, [][]interface{}, error) {
	columns := iter.Columns()
	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = c.Name
	}

	var rows [][]interface{}
	for {
		row := make(map[string]interface{}, len(columns))
		for _, c := range columns {
			if err := addNullableDest(row, c); err != nil {
				return nil, nil, err
			}
		}

		if !iter.MapScan(row) {
			break
		}

		values := make([]interface{}, len(columns))
		for i, c := range columns {
			if tuple, ok := c.TypeInfo.(gocql.TupleTypeInfo); ok {
				elems := make([]interface{}, len(tuple.Elems))
				for j := range tuple.Elems {
					elems[j] = convertValue(row[gocql.TupleColumnName(c.Name, j)])
				}
				values[i] = elems
				continue
			}
			values[i] = convertValue(row[c.Name])
		}
		rows = append(rows, values)
	}

	if err := iter.Close(); err != nil {
		return nil, nil, err
	}
	return names, rows, nil
}

func addNullableDest(row map[string]interface{}, c gocql.ColumnInfo) error {
	if tuple, ok := c.TypeInfo.(gocql.TupleTypeInfo); ok {
		for j, elem := range tuple.Elems {
			dest, err := nullableDest(elem)
			if err != nil {
				return fmt.Errorf("column %s: %w", c.Name, err)
			}
			row[gocql.TupleColumnName(c.Name, j)] = dest
		}
		return nil
	}

	dest, err := nullableDest(c.TypeInfo)
	if err != nil {
		return fmt.Errorf("column %s: %w", c.Name, err)
	}
	row[c.Name] = dest
	return nil
}

// nullableDest returns a **T for the Go type gocql maps the CQL type to.
func nullableDest(t gocql.TypeInfo) (interface{}, error) {
	value, err := t.NewWithError()
	if err != nil {
		return nil, err
	}
	return reflect.New(reflect.TypeOf(value)).Interface(), nil
}

// convertValue renders a gocql value as something JSON can carry: ids,
// times, addresses and big numbers become strings, collections and user
// types become lists and maps of converted values.
func convertValue(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case gocql.UUID:
		return x.String()
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	case time.Duration:
		// The CQL time type: nanoseconds since midnight.
		return formatClock(x)
	case gocql.Duration:
		return formatDuration(x)
	case net.IP:
		return x.String()
	case *big.Int:
		if x == nil {
			return nil
		}
		return x.String()
	case *inf.Dec:
		if x == nil {
			return nil
		}
		return x.String()
	case []byte:
		return "0x" + hex.EncodeToString(x)
	case string, bool, int, int8, int16, int32, int64, float32, float64:
		return x
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			return nil
		}
		return convertValue(rv.Elem().Interface())
	case reflect.Slice, reflect.Array:
		out := make([]interface{}, rv.Len())
		for i := range out {
			out[i] = convertValue(rv.Index(i).Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]interface{}, rv.Len())
		for it := rv.MapRange(); it.Next(); {
			out[fmt.Sprint(convertValue(it.Key().Interface()))] = convertValue(it.Value().Interface())
		}
		return out
	}

	return fmt.Sprint(v)
}

func formatClock(d time.Duration) string {
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	nanos := d - seconds*time.Second
	return fmt.Sprintf("%02d:%02d:%02d.%09d", hours, minutes, seconds, nanos)
}

func formatDuration(d gocql.Duration) string {
	return fmt.Sprintf("%dmo%dd%s", d.Months, d.Days, time.Duration(d.Nanoseconds))
}
