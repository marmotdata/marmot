package presto

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// scalarTypes lists the Presto types the driver converts on its own.
// Anything else is cast in the query so a preview never fails on one
// exotic column.
var scalarTypes = map[string]bool{
	"boolean":                  true,
	"tinyint":                  true,
	"smallint":                 true,
	"integer":                  true,
	"bigint":                   true,
	"real":                     true,
	"double":                   true,
	"decimal":                  true,
	"varchar":                  true,
	"char":                     true,
	"varbinary":                true,
	"json":                     true,
	"date":                     true,
	"time":                     true,
	"time with time zone":      true,
	"timestamp":                true,
	"timestamp with time zone": true,
	"interval year to month":   true,
	"interval day to second":   true,
	"ipaddress":                true,
}

// typeFamily strips the parameters from a Presto type: "row(...)" is
// "row", "varchar(25)" is "varchar".
func typeFamily(dataType string) string {
	family, _, _ := strings.Cut(dataType, "(")
	return strings.ToLower(strings.TrimSpace(family))
}

// sampleExpression renders one select-list entry. Presto sends row,
// array and map values in a text form the driver rejects, so those are
// cast to JSON; unknown types are cast to varchar.
func sampleExpression(name, dataType string) string {
	quoted := quoteIdentifier(name)
	switch family := typeFamily(dataType); {
	case family == "row" || family == "array" || family == "map":
		return "CAST(" + quoted + " AS JSON) AS " + quoted
	case scalarTypes[family]:
		return quoted
	default:
		return "CAST(" + quoted + " AS varchar) AS " + quoted
	}
}

// sampleQuery builds the preview query. With no column info it selects
// everything and leaves conversion to the driver.
func sampleQuery(key tableKey, columns []column) string {
	selectList := "*"
	if len(columns) > 0 {
		expressions := make([]string, len(columns))
		for i, col := range columns {
			expressions[i] = sampleExpression(col.Name, col.DataType)
		}
		selectList = strings.Join(expressions, ", ")
	}
	return "SELECT " + selectList + " FROM " + qualifiedName(key.Catalog, key.Schema, key.Table) + " LIMIT 20"
}

// FetchSampleData returns up to 20 rows of a table or view for preview.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	key := assetKey(*a)
	if key.Catalog == "" || key.Schema == "" || key.Table == "" {
		return nil, nil, fmt.Errorf("asset metadata is missing catalog, schema or table_name")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.initConnection(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("initialising connection: %w", err)
	}
	defer s.closeConnection()

	where := fmt.Sprintf("table_schema = '%s' AND table_name = '%s'", escapeString(key.Schema), escapeString(key.Table))
	columns, err := s.fetchColumns(fetchCtx, key.Catalog, where)
	if err != nil {
		log.Warn().Err(err).Str("table", key.String()).Msg("Failed to read column types, selecting all columns as is")
	}

	query := sampleQuery(key, columns[key])
	log.Debug().Str("table", key.String()).Msg("Fetching sample data")

	rows, err := s.db.QueryContext(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("getting column names: %w", err)
	}

	var dataRows [][]any
	for rows.Next() {
		values := make([]any, len(columnNames))
		pointers := make([]any, len(columnNames))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row, skipping")
			continue
		}
		for i, v := range values {
			values[i] = sampleValue(v)
		}
		dataRows = append(dataRows, values)
	}

	if err := rowsErr(rows); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	return columnNames, dataRows, nil
}

// sampleValue turns driver values into JSON-friendly ones.
func sampleValue(v any) any {
	switch value := v.(type) {
	case time.Time:
		return value.Format(time.RFC3339)
	case []byte:
		return string(value)
	default:
		return v
	}
}
