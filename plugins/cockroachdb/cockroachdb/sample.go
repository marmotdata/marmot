package cockroachdb

import (
	"context"
	"fmt"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// FetchSampleData implements pluginsdk.DataFetcher: it reads the first 20
// rows of a table or view for the asset preview.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, err
	}

	database, _ := a.Metadata["database"].(string)
	schema, _ := a.Metadata["schema"].(string)
	table, _ := a.Metadata["table_name"].(string)

	if database == "" {
		return nil, nil, fmt.Errorf("could not determine database from asset metadata")
	}
	if schema == "" {
		return nil, nil, fmt.Errorf("could not determine schema from asset metadata")
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := s.connect(fetchCtx, database)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to database %s: %w", database, err)
	}
	defer conn.Close(fetchCtx)

	query := fmt.Sprintf("SELECT * FROM %s LIMIT 20", tableIdent(database, schema, table))

	log.Debug().
		Str("database", database).
		Str("schema", schema).
		Str("table", table).
		Msg("Fetching sample data")

	rows, err := conn.Query(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columnNames[i] = fd.Name
	}

	var dataRows [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to read row, skipping")
			continue
		}

		converted := make([]interface{}, len(values))
		for i, val := range values {
			converted[i] = convertValue(val)
		}
		dataRows = append(dataRows, converted)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	log.Debug().
		Int("columns", len(columnNames)).
		Int("rows", len(dataRows)).
		Msg("Fetched sample data")

	return columnNames, dataRows, nil
}
