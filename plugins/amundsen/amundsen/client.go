package amundsen

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/rs/zerolog/log"
)

// maxPages stops a run that would page forever, which is what a query
// whose ORDER BY is not unique enough would do.
const maxPages = 10000

// reader runs one Cypher statement and returns its records as maps.
// Discovery only ever needs this much of Neo4j, so the discovery passes
// take the interface and the tests replay recorded graph rows through it
// without a database.
type reader interface {
	read(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
}

// client is a Bolt connection to the Neo4j behind Amundsen.
type client struct {
	driver   neo4j.DriverWithContext
	database string
	timeout  time.Duration
}

func newClient(ctx context.Context, config *Config) (*client, error) {
	uri, err := boltURI(config.URI, config.Encrypted, config.TrustAllCertificates)
	if err != nil {
		return nil, err
	}

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(config.Username, config.Password, ""), func(c *neo4j.Config) {
		c.UserAgent = "marmot-plugin-amundsen"
	})
	if err != nil {
		return nil, fmt.Errorf("creating neo4j driver: %w", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("connecting to neo4j at %s: %w", uri, err)
	}

	log.Debug().Str("uri", uri).Str("database", config.Database).Msg("Connected to Amundsen's Neo4j")

	return &client{
		driver:   driver,
		database: config.Database,
		timeout:  time.Duration(config.QueryTimeoutSeconds) * time.Second,
	}, nil
}

func (c *client) close(ctx context.Context) {
	if err := c.driver.Close(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to close the Neo4j driver")
	}
}

func (c *client) read(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: c.database,
	})
	defer session.Close(ctx)

	rows, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		records, err := result.Collect(ctx)
		if err != nil {
			return nil, err
		}

		out := make([]map[string]any, 0, len(records))
		for _, record := range records {
			out = append(out, record.AsMap())
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}

	return rows.([]map[string]any), nil
}

// eachPage walks a query to the end, one page at a time, so a graph with
// a million tables never has to fit in memory. Every paged query orders
// by a unique key, because SKIP and LIMIT only add up to the whole
// result when the order is stable between pages.
func eachPage(ctx context.Context, r reader, name, cypher string, pageSize int, fn func(rows []map[string]any) error) error {
	for page := 0; page < maxPages; page++ {
		rows, err := r.read(ctx, cypher, map[string]any{"skip": page * pageSize, "limit": pageSize})
		if err != nil {
			return fmt.Errorf("reading %s from Amundsen: %w", name, err)
		}

		if err := fn(rows); err != nil {
			return err
		}

		if len(rows) < pageSize {
			return nil
		}
	}

	log.Warn().Str("query", name).Int("pages", maxPages).Msg("Stopped paging at the page cap; some Amundsen records were not read")
	return nil
}

// Neo4j hands back int64 for integers, float64 for floats and nil for a
// property a node does not have, so every field is read through one of
// these rather than type asserted at the call site.

func textOf(row map[string]any, key string) string {
	if s, ok := row[key].(string); ok {
		return s
	}
	return ""
}

func numberOf(row map[string]any, key string) (float64, bool) {
	switch v := row[key].(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func boolOf(row map[string]any, key string) bool {
	switch v := row[key].(type) {
	case bool:
		return v
	case string:
		// Some Amundsen extractors write the flag as a string.
		return v == "true" || v == "True"
	}
	return false
}

// textsOf reads a COLLECT of strings, dropping anything that is not one.
func textsOf(row map[string]any, key string) []string {
	items, ok := row[key].([]any)
	if !ok {
		return nil
	}

	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mapsOf reads a COLLECT of maps. Neo4j answers a COLLECT over an
// OPTIONAL MATCH that found nothing with a single map of nulls, so
// callers still have to check the field they key on.
func mapsOf(row map[string]any, key string) []map[string]any {
	items, ok := row[key].([]any)
	if !ok {
		return nil
	}

	var out []map[string]any
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
