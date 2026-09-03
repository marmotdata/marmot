package trino

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// The query gateway keeps this plugin's process long-running, so the
// connection and catalog map are cached on the Source and rebuilt only when
// the target config changes or the connection dies.

const executeChunkRows = 500

// ensureConnection reuses the cached connection when the config is
// unchanged and the connection still answers, otherwise reconnects.
func (s *Source) ensureConnection(ctx context.Context) error {
	hash := configHash(s.config)
	if s.db != nil && s.dbConfigHash == hash {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.db.PingContext(pingCtx); err == nil {
			return nil
		}
		log.Warn().Msg("Cached Trino connection failed ping, reconnecting")
	}

	if err := s.initConnection(ctx); err != nil {
		return err
	}
	s.dbConfigHash = hash
	return nil
}

func configHash(c *Config) string {
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// loadCatalogConnectors fills the catalog to connector map for MRN mapping.
// Unlike discoverCatalogs it applies no exclusions: planning must map any
// catalog a statement can reference.
func (s *Source) loadCatalogConnectors(ctx context.Context) error {
	if len(s.catalogConnectors) > 0 {
		return nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, "SELECT catalog_name, connector_name FROM system.metadata.catalogs")
	if err != nil {
		return fmt.Errorf("querying catalogs: %w", err)
	}
	defer rows.Close()

	s.catalogConnectors = make(map[string]string)
	for rows.Next() {
		var name, connector string
		if err := rows.Scan(&name, &connector); err != nil {
			continue
		}
		s.catalogConnectors[name] = connector
	}
	return rows.Err()
}

// ioPlan mirrors the parts of EXPLAIN (TYPE IO, FORMAT JSON) output the
// planner needs.
type ioPlan struct {
	InputTableColumnInfos []struct {
		Table struct {
			Catalog     string `json:"catalog"`
			SchemaTable struct {
				Schema string `json:"schema"`
				Table  string `json:"table"`
			} `json:"schemaTable"`
		} `json:"table"`
	} `json:"inputTableColumnInfos"`
	Estimate struct {
		OutputSizeInBytes float64 `json:"outputSizeInBytes"`
	} `json:"estimate"`
}

// PlanQuery asks Trino what a statement would read via EXPLAIN (TYPE IO),
// so the engine does the SQL parsing, then maps each referenced table to
// the same MRN the discovery path would give it so grants written against
// catalogue assets bind on the query path too.
func (s *Source) PlanQuery(ctx context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest) (*pluginsdk.QueryPlan, error) {
	if err := s.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("connecting to Trino: %w", err)
	}
	if err := s.loadCatalogConnectors(ctx); err != nil {
		return nil, fmt.Errorf("loading catalog connectors: %w", err)
	}

	var planJSON string
	if err := s.db.QueryRowContext(ctx, "EXPLAIN (TYPE IO, FORMAT JSON) "+req.Statement).Scan(&planJSON); err != nil {
		return nil, fmt.Errorf("planning statement: %w", err)
	}

	var plan ioPlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return nil, fmt.Errorf("parsing IO plan: %w", err)
	}

	seen := map[string]bool{}
	result := &pluginsdk.QueryPlan{EstimatedBytes: int64(plan.Estimate.OutputSizeInBytes)}
	for _, input := range plan.InputTableColumnInfos {
		catalog := input.Table.Catalog
		schema := input.Table.SchemaTable.Schema
		table := input.Table.SchemaTable.Table

		tableMRN := s.tableMRN(ctx, catalog, schema, table)
		if !seen[tableMRN] {
			seen[tableMRN] = true
			result.ReferencedMRNs = append(result.ReferencedMRNs, tableMRN)
		}
	}
	return result, nil
}

// tableMRN reproduces the discovery path's MRN for a referenced table.
// Internal connectors (memory, tpch, ...) are skipped by discovery, so
// their tables get a Trino-scoped MRN that grants can still target.
func (s *Source) tableMRN(ctx context.Context, catalog, schema, table string) string {
	info, ok := connectorInfoForName(s.catalogConnectors[catalog])
	if !ok {
		info = connectorInfo{Provider: "Trino", MRNName: defaultMRNName}
	}

	assetType := "Table"
	if s.isView(ctx, catalog, schema, table) {
		assetType = "View"
	}
	return mrn.New(assetType, info.Provider, info.MRNName(catalog, schema, table))
}

func (s *Source) isView(ctx context.Context, catalog, schema, table string) bool {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitized via quoteIdentifier/escapeString
		"SELECT table_type FROM %s.information_schema.tables WHERE table_schema = '%s' AND table_name = '%s'",
		quoteIdentifier(catalog), escapeString(schema), escapeString(table),
	)

	var tableType string
	if err := s.db.QueryRowContext(queryCtx, query).Scan(&tableType); err != nil {
		return false
	}
	return strings.EqualFold(tableType, "VIEW")
}

// ExecuteQuery runs the statement and streams rows back in chunks, the
// first carrying the column schema.
func (s *Source) ExecuteQuery(ctx context.Context, _ pluginsdk.RawConfig, req pluginsdk.QueryRequest, emit func(pluginsdk.QueryResultChunk) error) error {
	if err := s.ensureConnection(ctx); err != nil {
		return fmt.Errorf("connecting to Trino: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, req.Statement)
	if err != nil {
		return fmt.Errorf("executing statement: %w", err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("reading columns: %w", err)
	}
	columns := make([]pluginsdk.QueryColumn, len(columnNames))
	for i, name := range columnNames {
		columns[i] = pluginsdk.QueryColumn{Name: name}
	}
	if columnTypes, err := rows.ColumnTypes(); err == nil {
		for i, ct := range columnTypes {
			columns[i].Type = strings.ToLower(ct.DatabaseTypeName())
		}
	}

	chunk := pluginsdk.QueryResultChunk{Columns: columns}
	var sent int64
	for rows.Next() {
		values := make([]any, len(columnNames))
		pointers := make([]any, len(columnNames))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}

		chunk.Rows = append(chunk.Rows, values)
		sent++
		if req.MaxRows > 0 && sent >= req.MaxRows {
			break
		}
		if len(chunk.Rows) >= executeChunkRows {
			if err := emit(chunk); err != nil {
				return err
			}
			chunk = pluginsdk.QueryResultChunk{}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating rows: %w", err)
	}

	// Always emit the final chunk, even empty, so the schema reaches the
	// host for zero-row results.
	if len(chunk.Rows) > 0 || chunk.Columns != nil {
		return emit(chunk)
	}
	return nil
}
