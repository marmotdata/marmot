package openmetadata

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// Database entities. OpenMetadata models every SQL-ish source as
// service > database > schema > table, filling in a level an engine
// does not have with "default". The projection for the service decides
// which of those levels becomes an asset and how table names are built.

const (
	databaseFields        = "owners,tags,domains,dataProducts"
	databaseSchemaFields  = "owners,tags,domains,dataProducts,database"
	tableFields           = "columns,owners,tags,domains,dataProducts,tableConstraints,profile"
	storedProcedureFields = "owners,tags,domains,dataProducts,storedProcedureCode"
)

// tableGroups is the set of container assets tables hang off, keyed by
// the level of the OpenMetadata path they came from, so a table can find
// its own container and link to it.
type tableGroups map[string]string

func (c *collector) discoverDatabases(ctx context.Context, client *client) (tableGroups, error) {
	groups := make(tableGroups)

	databases, err := listAll[database](ctx, client, "/v1/databases", databaseFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return groups, fmt.Errorf("listing databases: %w", err)
	}

	schemas, err := listAll[databaseSchema](ctx, client, "/v1/databaseSchemas", databaseSchemaFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return groups, fmt.Errorf("listing database schemas: %w", err)
	}

	for _, db := range databases {
		if !c.wanted(db.entityBase) {
			continue
		}
		p := projectionFor(db.ServiceType)
		if p.TableGroup != groupDatabase {
			continue
		}

		parts := fqnBelowService(db.FullyQualifiedName)
		if len(parts) == 0 {
			continue
		}
		c.addTableGroup(db.entityBase, p, "database", groups)
	}

	for _, schema := range schemas {
		if !c.wanted(schema.entityBase) {
			continue
		}
		p := projectionFor(schema.ServiceType)
		if p.TableGroup != groupSchema {
			continue
		}

		parts := fqnBelowService(schema.FullyQualifiedName)
		if len(parts) == 0 {
			continue
		}
		c.addTableGroup(schema.entityBase, p, "databaseSchema", groups)
	}

	log.Debug().Int("count", len(groups)).Msg("Discovered database containers")
	return groups, nil
}

// addTableGroup creates the container asset a service's tables hang off,
// named the way the technology's own Marmot plugin names it: the
// database for Postgres, the dataset for BigQuery.
func (c *collector) addTableGroup(base entityBase, p projection, kind string, groups tableGroups) {
	// The last level is the one that means anything: an OpenMetadata
	// database for Postgres, a schema for MySQL, a dataset for BigQuery,
	// a namespace for Iceberg. The level above it is a placeholder
	// wherever the engine has no such concept, and including it would
	// stop the asset matching the native plugin's.
	parts := fqnBelowService(base.FullyQualifiedName)
	nativeName := parts[len(parts)-1]

	metadata := map[string]interface{}{}
	putIf(metadata, "database", parts[0])
	if p.TableGroup == groupSchema && len(parts) > 1 {
		putIf(metadata, "schema", parts[len(parts)-1])
	}

	name := c.mrnName(nativeName, base.FullyQualifiedName)
	asset := c.newAsset(base, kind, p.TableGroupType, p, name, metadata)
	c.add(base.ID, asset)

	groups[containerKey(base.FullyQualifiedName, p)] = *asset.MRN
}

func (c *collector) discoverTables(ctx context.Context, client *client, groups tableGroups) error {
	tables, err := listAll[table](ctx, client, "/v1/tables", tableFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing tables: %w", err)
	}

	discovered := 0
	for _, t := range tables {
		if !c.wanted(t.entityBase) {
			continue
		}

		p := projectionFor(t.ServiceType)
		parts := fqnBelowService(t.FullyQualifiedName)
		if len(parts) < 3 {
			log.Debug().Str("fqn", t.FullyQualifiedName).Msg("Skipping table with unexpected fully qualified name")
			continue
		}
		databaseName, schemaName, tableName := parts[0], parts[1], strings.Join(parts[2:], ".")

		metadata := map[string]interface{}{}
		putIf(metadata, "database", databaseName)
		putIf(metadata, "schema", schemaName)
		putIf(metadata, "table_name", tableName)
		putIf(metadata, "object_type", t.TableType)
		putIf(metadata, "column_count", len(t.Columns))
		if t.Profile != nil {
			putIf(metadata, "row_count", int64(t.Profile.RowCount))
			putIf(metadata, "size", int64(t.Profile.SizeInByte))
		}
		if t.UsageSummary != nil {
			putIf(metadata, "weekly_query_count", int(t.UsageSummary.WeeklyStats.Count))
		}
		if keys := constraintColumns(t.TableConstraints, "PRIMARY_KEY"); len(keys) > 0 {
			putIf(metadata, "primary_key", keys)
		}

		assetType := p.assetTypeFor(t.TableType)
		name := c.mrnName(p.TableName(databaseName, schemaName, tableName), t.FullyQualifiedName)

		asset := c.newAsset(t.entityBase, "table", assetType, p, name, metadata)
		if c.config.IncludeColumns {
			setColumns(&asset, t.Columns)
		}
		c.add(t.ID, asset)
		discovered++

		if groupMRN, ok := groups[containerKey(t.FullyQualifiedName, p)]; ok {
			c.link(groupMRN, *asset.MRN, "CONTAINS")
		}
	}

	log.Debug().Int("count", discovered).Msg("Discovered tables")
	return nil
}

// discoverStoredProcedures catalogues database routines. Marmot has no
// stored procedure type; a procedure is a named routine, which is what
// Function already means for Lambda, so it reuses that.
func (c *collector) discoverStoredProcedures(ctx context.Context, client *client, groups tableGroups) error {
	procedures, supported, err := listOptional[storedProcedure](ctx, client, "/v1/storedProcedures", storedProcedureFields, c.config.PageSize, c.config.IncludeDeleted)
	if err != nil {
		return fmt.Errorf("listing stored procedures: %w", err)
	}
	if !supported {
		log.Debug().Msg("OpenMetadata does not expose stored procedures, skipping")
		return nil
	}

	discovered := 0
	for _, sp := range procedures {
		if !c.wanted(sp.entityBase) {
			continue
		}

		p := projectionFor(sp.ServiceType)
		parts := fqnBelowService(sp.FullyQualifiedName)
		if len(parts) < 3 {
			continue
		}
		databaseName, schemaName, procedureName := parts[0], parts[1], strings.Join(parts[2:], ".")

		metadata := map[string]interface{}{}
		putIf(metadata, "database", databaseName)
		putIf(metadata, "schema", schemaName)
		putIf(metadata, "procedure_type", sp.StoredProcedureType)

		name := c.mrnName(p.TableName(databaseName, schemaName, procedureName), sp.FullyQualifiedName)
		asset := c.newAsset(sp.entityBase, "storedProcedure", "Function", p, name, metadata)

		if sp.Code != nil && sp.Code.Code != "" {
			query, language := sp.Code.Code, sp.Code.Language
			asset.Query = &query
			if language != "" {
				asset.QueryLanguage = &language
			}
		}

		c.add(sp.ID, asset)
		discovered++

		if groupMRN, ok := groups[containerKey(sp.FullyQualifiedName, p)]; ok {
			c.link(groupMRN, *asset.MRN, "CONTAINS")
		}
	}

	log.Debug().Int("count", discovered).Msg("Discovered stored procedures")
	return nil
}

// containerKey identifies the level a table is grouped under. It is
// built from the split parts rather than the raw fully qualified name so
// that a name containing a dot, which OpenMetadata quotes, matches the
// same key on both sides of the lookup.
func containerKey(fqn string, p projection) string {
	parts := splitFQN(fqn)

	depth := 2
	switch p.TableGroup {
	case groupDatabase:
		depth = 2
	case groupSchema:
		depth = 3
	default:
		return ""
	}

	if len(parts) < depth {
		return ""
	}
	return strings.Join(parts[:depth], "\x00")
}

func constraintColumns(constraints []constraint, kind string) []string {
	for _, c := range constraints {
		if c.ConstraintType == kind {
			return c.Columns
		}
	}
	return nil
}
