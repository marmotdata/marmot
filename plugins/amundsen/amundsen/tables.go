package amundsen

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Amundsen's hierarchy is Database -> Cluster -> Schema -> Table, where
// the Database node holds the technology ("postgres", "hive") rather
// than a database. The four levels are read together so a table arrives
// with everything needed to project it onto a native Marmot identity.
//
// Read counts are deliberately not part of this query. Every OPTIONAL
// MATCH multiplies the rows a table is grouped from, so a SUM over the
// READ_BY relationship counts each read once per combination of tag,
// badge and column and reports a number several times the truth. They
// are aggregated on their own in tableUsageQuery instead.
const tableQuery = `
MATCH (db:Database)<-[:CLUSTER_OF]-(cluster:Cluster)<-[:SCHEMA_OF]-(schema:Schema)<-[:TABLE_OF]-(table:Table)
OPTIONAL MATCH (table)-[:DESCRIPTION]->(table_description:Description)
OPTIONAL MATCH (schema)-[:DESCRIPTION]->(schema_description:Description)
OPTIONAL MATCH (table)-[:DESCRIPTION]->(prog_description:Programmatic_Description)
OPTIONAL MATCH (table)-[:TAGGED_BY]->(tag:Tag) WHERE tag.tag_type = 'default'
OPTIONAL MATCH (table)-[:HAS_BADGE]->(badge:Badge)
OPTIONAL MATCH (table)-[:COLUMN]->(col:Column)
OPTIONAL MATCH (col)-[:DESCRIPTION]->(col_description:Description)
OPTIONAL MATCH (table)-[:LAST_UPDATED_AT]->(t:Timestamp)
RETURN db.name AS database, cluster.name AS cluster, schema.name AS schema,
       schema_description.description AS schema_description,
       table.name AS name, table.key AS key, table.is_view AS is_view,
       table_description.description AS description,
       t.last_updated_timestamp AS last_updated_timestamp,
       COLLECT(DISTINCT {name: col.name, type: col.col_type, sort_order: col.sort_order, description: col_description.description}) AS columns,
       COLLECT(DISTINCT tag.key) AS tags,
       COLLECT(DISTINCT badge.key) AS badges,
       COLLECT(DISTINCT prog_description.description) AS programmatic_descriptions
ORDER BY key
SKIP $skip LIMIT $limit
`

const tableUsageQuery = `
MATCH (table:Table)-[read:READ_BY]->(user:User)
RETURN table.key AS key, SUM(read.read_count) AS total_usage, COUNT(DISTINCT user.email) AS unique_usage
ORDER BY key
SKIP $skip LIMIT $limit
`

const tableOwnerQuery = `
MATCH (table:Table)<-[:OWNER_OF]-(user:User)
RETURN table.key AS table_key, user.email AS email, user.full_name AS full_name, user.team_name AS team
ORDER BY table_key, email
SKIP $skip LIMIT $limit
`

// Amundsen writes a lineage edge in both directions, so reading one of
// them is enough. A graph loaded by an older Amundsen has neither, which
// Neo4j answers with no rows rather than an error.
const tableLineageQuery = `
MATCH (a:Table)-[:HAS_UPSTREAM]->(b:Table)
RETURN a.key AS downstream, b.key AS upstream
ORDER BY downstream, upstream
SKIP $skip LIMIT $limit
`

// tableRow is one row of tableQuery.
type tableRow struct {
	Database                 string
	Cluster                  string
	Schema                   string
	Name                     string
	Key                      string
	IsView                   bool
	Description              string
	SchemaDescription        string
	LastUpdated              int64
	Columns                  []tableColumn
	Tags                     []string
	Badges                   []string
	ProgrammaticDescriptions []string
}

// tableColumn is one Amundsen Column node. Name, type and description
// are collected together in the query rather than as three parallel
// lists, so a column missing a description cannot shift every later
// description onto the wrong column.
type tableColumn struct {
	Name        string
	Type        string
	Description string
	SortOrder   int64
}

// owner is a User node that owns a table.
type owner struct {
	Email    string
	FullName string
	Team     string
}

// Name is what to show for an owner. Amundsen keys users by email and
// fills in the full name only when its user source knows one.
func (o owner) displayName() string {
	if o.FullName != "" {
		return o.FullName
	}
	return o.Email
}

func (c *collector) discoverOwners(ctx context.Context, r reader) error {
	count := 0
	err := eachPage(ctx, r, "table owners", tableOwnerQuery, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			key := textOf(row, "table_key")
			email := textOf(row, "email")
			if key == "" || email == "" {
				continue
			}
			c.ownersByTable[key] = append(c.ownersByTable[key], owner{
				Email:    email,
				FullName: textOf(row, "full_name"),
				Team:     textOf(row, "team"),
			})
			count++
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Int("owners", count).Int("tables", len(c.ownersByTable)).Msg("Read Amundsen table owners")
	return nil
}

func (c *collector) discoverTables(ctx context.Context, r reader) error {
	usage, err := c.readUsage(ctx, r, "table usage", tableUsageQuery)
	if err != nil {
		return err
	}

	discovered := 0
	err = eachPage(ctx, r, "tables", tableQuery, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			table := decodeTableRow(row)
			if table.Name == "" {
				log.Warn().Str("key", table.Key).Msg("Skipping an Amundsen table with no name")
				continue
			}
			if c.addTable(table, usage[table.Key]) {
				discovered++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Int("tables", discovered).Msg("Discovered Amundsen tables")
	return nil
}

func (c *collector) addTable(table tableRow, use usage) bool {
	p := projectionFor(table.Database)
	name := p.TableName(table.Database, table.Cluster, table.Schema, table.Name)
	if name == "" {
		log.Warn().Str("key", table.Key).Msg("Skipping an Amundsen table whose projected name is empty")
		return false
	}

	// The cluster is recorded whether or not the projection put it in
	// the name, because for most technologies it is an environment label
	// Marmot does not address by but a reader still wants to see.
	provenance := map[string]any{}
	putIf(provenance, "key", table.Key)
	putIf(provenance, "database", table.Database)
	putIf(provenance, "cluster", table.Cluster)
	putIf(provenance, "schema", table.Schema)
	putIf(provenance, "table", table.Name)
	putIf(provenance, "badges", table.Badges)
	putIf(provenance, "tags", table.Tags)
	if c.config.IncludeDescriptions {
		putIf(provenance, "programmatic_descriptions", table.ProgrammaticDescriptions)
	}
	if table.LastUpdated > 0 {
		provenance["last_updated_at"] = time.Unix(table.LastUpdated, 0).UTC().Format(time.RFC3339)
	}
	if link := c.tableURL(table); link != "" {
		provenance["url"] = link
	}

	metadata := map[string]any{"amundsen": provenance}
	if c.config.IncludeDescriptions {
		putIf(metadata, "schema_description", table.SchemaDescription)
	}

	owners := c.ownersByTable[table.Key]
	if len(owners) > 0 {
		names := make([]string, 0, len(owners))
		emails := make([]string, 0, len(owners))
		var teams []string
		for _, o := range owners {
			names = append(names, o.displayName())
			emails = append(emails, o.Email)
			if o.Team != "" && !contains(teams, o.Team) {
				teams = append(teams, o.Team)
			}
		}
		metadata["owners"] = names
		metadata["owner_emails"] = emails
		putIf(metadata, "owner_teams", teams)
	}

	asset := c.newAsset(table.assetType(), p.Provider, name, table.Description, c.assetTags(table.Tags), metadata)

	if columns := table.assetColumns(c.config.IncludeDescriptions); len(columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, columns); err != nil {
			log.Warn().Err(err).Str("key", table.Key).Msg("Failed to attach columns")
		}
	}

	if link := c.tableURL(table); link != "" {
		asset.ExternalLinks = append([]pluginsdk.AssetExternalLink{{
			Name: "Open in Amundsen",
			Icon: "mdi:open-in-new",
			URL:  link,
		}}, asset.ExternalLinks...)
	}

	if !c.add(table.Key, asset) {
		return false
	}

	c.stat(*asset.MRN, metricReadCount, use.Total)
	c.stat(*asset.MRN, metricUniqueReaders, use.Unique)
	return true
}

// tableURL is the table's page in the Amundsen web app. Amundsen routes
// a table by cluster, database, schema and name rather than by its key.
func (c *collector) tableURL(table tableRow) string {
	if c.config.AmundsenURL == "" || table.Cluster == "" || table.Database == "" || table.Name == "" {
		return ""
	}
	return fmt.Sprintf("%s/table_detail/%s/%s/%s/%s",
		c.config.AmundsenURL,
		url.PathEscape(table.Cluster),
		url.PathEscape(table.Database),
		url.PathEscape(table.Schema),
		url.PathEscape(table.Name))
}

// assetTags is the Amundsen tags a run copies onto its assets. The tags
// the run itself was configured with are added later, by newAsset.
func (c *collector) assetTags(tags []string) []string {
	if !c.config.IncludeTags {
		return nil
	}
	return append([]string(nil), tags...)
}

func (c *collector) discoverTableLineage(ctx context.Context, r reader) error {
	edges := 0
	err := eachPage(ctx, r, "table lineage", tableLineageQuery, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			upstream := c.tableMRN(textOf(row, "upstream"))
			downstream := c.tableMRN(textOf(row, "downstream"))
			if upstream == "" || downstream == "" {
				continue
			}
			c.link(upstream, downstream, "FEEDS")
			edges++
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Int("edges", edges).Msg("Discovered Amundsen table lineage")
	return nil
}

// tableMRN resolves an Amundsen table key to an MRN. A key discovered in
// this run resolves through the assets already built; anything else is
// projected straight from the key, so lineage still reaches a table this
// run did not import but the technology's own plugin did.
func (c *collector) tableMRN(key string) string {
	if key == "" {
		return ""
	}
	if known, ok := c.mrnByKey[key]; ok {
		return known
	}

	parts, ok := parseTableKey(key)
	if !ok {
		log.Debug().Str("key", key).Msg("Skipping a lineage endpoint whose Amundsen key could not be read")
		return ""
	}

	p := projectionFor(parts.Database)
	name := p.TableName(parts.Database, parts.Cluster, parts.Schema, parts.Table)
	if name == "" {
		return ""
	}
	return assetMRN("Table", p.Provider, name)
}

// tableKey is an Amundsen table key split into its levels.
type tableKey struct {
	Database string
	Cluster  string
	Schema   string
	Table    string
}

// parseTableKey reads an Amundsen table key, which is written
// "<database>://<cluster>.<schema>/<table>", for example
// "postgres://prod.public/orders". A key that does not have that shape
// cannot be projected onto an identity, so it is reported as unusable
// rather than guessed at.
func parseTableKey(key string) (tableKey, bool) {
	database, rest, ok := strings.Cut(key, "://")
	if !ok || database == "" {
		return tableKey{}, false
	}

	location, table, ok := strings.Cut(rest, "/")
	if !ok || table == "" {
		return tableKey{}, false
	}

	cluster, schema, _ := strings.Cut(location, ".")
	if cluster == "" {
		return tableKey{}, false
	}

	return tableKey{Database: database, Cluster: cluster, Schema: schema, Table: table}, true
}

// usage is the read counts Amundsen holds for one node.
type usage struct {
	Total  float64
	Unique float64
}

// readUsage aggregates read counts on their own, one row per node, so
// the numbers are not multiplied by the other things a table is joined
// to. A run that does not want usage does not run the query at all.
func (c *collector) readUsage(ctx context.Context, r reader, name, query string) (map[string]usage, error) {
	counts := make(map[string]usage)
	if !c.config.IncludeUsage {
		return counts, nil
	}

	err := eachPage(ctx, r, name, query, c.config.PageSize, func(rows []map[string]any) error {
		for _, row := range rows {
			key := textOf(row, "key")
			if key == "" {
				continue
			}
			total, _ := numberOf(row, "total_usage")
			unique, _ := numberOf(row, "unique_usage")
			counts[key] = usage{Total: total, Unique: unique}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return counts, nil
}

func decodeTableRow(row map[string]any) tableRow {
	table := tableRow{
		Database:                 textOf(row, "database"),
		Cluster:                  textOf(row, "cluster"),
		Schema:                   textOf(row, "schema"),
		Name:                     textOf(row, "name"),
		Key:                      textOf(row, "key"),
		IsView:                   boolOf(row, "is_view"),
		Description:              textOf(row, "description"),
		SchemaDescription:        textOf(row, "schema_description"),
		Tags:                     textsOf(row, "tags"),
		Badges:                   textsOf(row, "badges"),
		ProgrammaticDescriptions: textsOf(row, "programmatic_descriptions"),
	}

	if updated, ok := numberOf(row, "last_updated_timestamp"); ok {
		table.LastUpdated = int64(updated)
	}

	for _, column := range mapsOf(row, "columns") {
		name := textOf(column, "name")
		if name == "" {
			// A table with no columns still collects one entry, made
			// entirely of the nulls the OPTIONAL MATCH did not fill.
			continue
		}
		sortOrder, _ := numberOf(column, "sort_order")
		table.Columns = append(table.Columns, tableColumn{
			Name:        name,
			Type:        textOf(column, "type"),
			Description: textOf(column, "description"),
			SortOrder:   int64(sortOrder),
		})
	}

	return table
}

// assetType is Table unless Amundsen marks the entry as a view, either
// with the is_view property its Presto and Hive extractors set or with a
// "view" badge someone applied by hand.
func (t tableRow) assetType() string {
	if t.IsView {
		return "View"
	}
	for _, badge := range t.Badges {
		if strings.EqualFold(badge, "view") {
			return "View"
		}
	}
	return "Table"
}

// assetColumns is the table's columns in the order Amundsen records,
// which a COLLECT does not preserve. Columns with the same sort order,
// which happens when an extractor never set one, fall back to their name
// so the order is at least stable between runs.
func (t tableRow) assetColumns(includeDescriptions bool) []pluginsdk.Column {
	ordered := append([]tableColumn(nil), t.Columns...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].SortOrder != ordered[j].SortOrder {
			return ordered[i].SortOrder < ordered[j].SortOrder
		}
		return ordered[i].Name < ordered[j].Name
	})

	columns := make([]pluginsdk.Column, 0, len(ordered))
	for _, column := range ordered {
		out := pluginsdk.Column{
			Name:     column.Name,
			DataType: column.Type,
			// Amundsen does not record nullability, and every column it
			// knows about is one a reader may find empty.
			Nullable: true,
		}
		if includeDescriptions {
			out.Description = column.Description
		}
		columns = append(columns, out)
	}
	return columns
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
