package cassandra

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// The types below mirror the system_schema rows this plugin reads. They
// carry only the columns that end up in an asset.

type keyspaceInfo struct {
	Name          string
	DurableWrites bool
	Replication   map[string]string
}

type tableInfo struct {
	Name                string
	ID                  gocql.UUID
	Comment             string
	Compaction          map[string]string
	Compression         map[string]string
	Caching             map[string]string
	DefaultTTL          int
	GCGraceSeconds      int
	BloomFilterFPChance float64
	Flags               []string
}

type viewInfo struct {
	Name              string
	BaseTable         string
	IncludeAllColumns bool
	WhereClause       string
	Comment           string
	ID                gocql.UUID
}

type columnInfo struct {
	Table           string
	Name            string
	ClusteringOrder string
	Kind            string
	Position        int
	Type            string
}

type indexInfo struct {
	Table   string
	Name    string
	Kind    string
	Options map[string]string
}

type userType struct {
	Name       string
	FieldNames []string
	FieldTypes []string
}

type clusterInfo struct {
	Version    string
	Name       string
	Datacenter string
}

// Column kinds as stored in system_schema.columns.kind.
const (
	kindPartitionKey = "partition_key"
	kindClustering   = "clustering"
	kindRegular      = "regular"
	kindStatic       = "static"
)

func (s *Source) clusterInfo(ctx context.Context) (clusterInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var info clusterInfo
	err := s.session.Query(`SELECT release_version, cluster_name, data_center FROM system.local`).
		WithContext(ctx).
		Scan(&info.Version, &info.Name, &info.Datacenter)
	if err != nil {
		return clusterInfo{}, fmt.Errorf("querying system.local: %w", err)
	}
	return info, nil
}

func (s *Source) listKeyspaces(ctx context.Context) ([]keyspaceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`SELECT keyspace_name, durable_writes, replication FROM system_schema.keyspaces`).
		WithContext(ctx).
		Iter()

	var keyspaces []keyspaceInfo
	var ks keyspaceInfo
	for iter.Scan(&ks.Name, &ks.DurableWrites, &ks.Replication) {
		keyspaces = append(keyspaces, ks)
		ks = keyspaceInfo{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.keyspaces: %w", err)
	}

	sort.Slice(keyspaces, func(i, j int) bool { return keyspaces[i].Name < keyspaces[j].Name })
	return keyspaces, nil
}

func (s *Source) listTables(ctx context.Context, keyspace string) ([]tableInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`
		SELECT table_name, id, comment, compaction, compression, default_time_to_live,
		       gc_grace_seconds, bloom_filter_fp_chance, caching, flags
		FROM system_schema.tables
		WHERE keyspace_name = ?`, keyspace).
		WithContext(ctx).
		Iter()

	var tables []tableInfo
	var t tableInfo
	for iter.Scan(&t.Name, &t.ID, &t.Comment, &t.Compaction, &t.Compression, &t.DefaultTTL,
		&t.GCGraceSeconds, &t.BloomFilterFPChance, &t.Caching, &t.Flags) {
		tables = append(tables, t)
		t = tableInfo{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.tables: %w", err)
	}

	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
	return tables, nil
}

func (s *Source) listViews(ctx context.Context, keyspace string) ([]viewInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`
		SELECT view_name, base_table_name, include_all_columns, where_clause, comment, id
		FROM system_schema.views
		WHERE keyspace_name = ?`, keyspace).
		WithContext(ctx).
		Iter()

	var views []viewInfo
	var v viewInfo
	for iter.Scan(&v.Name, &v.BaseTable, &v.IncludeAllColumns, &v.WhereClause, &v.Comment, &v.ID) {
		views = append(views, v)
		v = viewInfo{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.views: %w", err)
	}

	sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	return views, nil
}

// listColumns reads every column in the keyspace in one query and groups
// them by table. Materialized views list their columns here too, under the
// view's name.
func (s *Source) listColumns(ctx context.Context, keyspace string) (map[string][]columnInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`
		SELECT table_name, column_name, clustering_order, kind, position, type
		FROM system_schema.columns
		WHERE keyspace_name = ?`, keyspace).
		WithContext(ctx).
		Iter()

	columns := make(map[string][]columnInfo)
	var c columnInfo
	for iter.Scan(&c.Table, &c.Name, &c.ClusteringOrder, &c.Kind, &c.Position, &c.Type) {
		columns[c.Table] = append(columns[c.Table], c)
		c = columnInfo{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.columns: %w", err)
	}

	for table := range columns {
		sortColumns(columns[table])
	}
	return columns, nil
}

func (s *Source) listIndexes(ctx context.Context, keyspace string) (map[string][]indexInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`
		SELECT table_name, index_name, kind, options
		FROM system_schema.indexes
		WHERE keyspace_name = ?`, keyspace).
		WithContext(ctx).
		Iter()

	indexes := make(map[string][]indexInfo)
	var idx indexInfo
	for iter.Scan(&idx.Table, &idx.Name, &idx.Kind, &idx.Options) {
		indexes[idx.Table] = append(indexes[idx.Table], idx)
		idx = indexInfo{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.indexes: %w", err)
	}

	return indexes, nil
}

func (s *Source) listUserTypes(ctx context.Context, keyspace string) ([]userType, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	iter := s.session.Query(`
		SELECT type_name, field_names, field_types
		FROM system_schema.types
		WHERE keyspace_name = ?`, keyspace).
		WithContext(ctx).
		Iter()

	var types []userType
	var t userType
	for iter.Scan(&t.Name, &t.FieldNames, &t.FieldTypes) {
		types = append(types, t)
		t = userType{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("querying system_schema.types: %w", err)
	}

	return types, nil
}

// replication is the parsed form of a keyspace's replication map.
type replication struct {
	Class string
	// Factor is the single replication factor of SimpleStrategy, or of a
	// NetworkTopologyStrategy declared with one factor for every
	// datacenter. Zero when factors are given per datacenter.
	Factor int
	// PerDatacenter holds NetworkTopologyStrategy factors keyed by
	// datacenter.
	PerDatacenter map[string]int
}

// parseReplication reads the replication map stored on a keyspace. Class
// names are stored fully qualified and factors as strings; a transient
// replication factor like "3/1" counts its full replica total.
func parseReplication(options map[string]string) replication {
	rep := replication{Class: shortClassName(options["class"])}

	for key, value := range options {
		if key == "class" {
			continue
		}
		factor, ok := parseReplicationFactor(value)
		if !ok {
			continue
		}
		if key == "replication_factor" {
			rep.Factor = factor
			continue
		}
		if rep.PerDatacenter == nil {
			rep.PerDatacenter = make(map[string]int)
		}
		rep.PerDatacenter[key] = factor
	}

	return rep
}

func parseReplicationFactor(value string) (int, bool) {
	total, _, _ := strings.Cut(value, "/")
	factor, err := strconv.Atoi(strings.TrimSpace(total))
	if err != nil || factor < 0 {
		return 0, false
	}
	return factor, true
}

// sortColumns orders columns the way a CQL DESCRIBE would: partition key
// columns by position, then clustering columns by position, then the rest
// alphabetically.
func sortColumns(cols []columnInfo) {
	rank := func(c columnInfo) int {
		switch c.Kind {
		case kindPartitionKey:
			return 0
		case kindClustering:
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(cols, func(i, j int) bool {
		ri, rj := rank(cols[i]), rank(cols[j])
		if ri != rj {
			return ri < rj
		}
		if ri < 2 && cols[i].Position != cols[j].Position {
			return cols[i].Position < cols[j].Position
		}
		return cols[i].Name < cols[j].Name
	})
}

// keyColumns extracts the primary key layout from an already sorted column
// list: partition key names, clustering column names, and each clustering
// column's sort direction.
func keyColumns(cols []columnInfo) (partition, clustering []string, order map[string]string) {
	for _, c := range cols {
		switch c.Kind {
		case kindPartitionKey:
			partition = append(partition, c.Name)
		case kindClustering:
			clustering = append(clustering, c.Name)
			if order == nil {
				order = make(map[string]string)
			}
			order[c.Name] = c.ClusteringOrder
		}
	}
	return partition, clustering, order
}

// schemaColumn is the per-column shape serialised into an asset's schema:
// the canonical column plus the fields that only make sense in Cassandra.
type schemaColumn struct {
	pluginsdk.Column
	// Kind is partition_key, clustering, regular or static.
	Kind string `json:"kind"`
	// Position is the column's index within the partition or clustering
	// key, and -1 for every other column.
	Position int `json:"position"`
	// ClusteringOrder is asc or desc for clustering columns.
	ClusteringOrder string `json:"clustering_order,omitempty"`
}

// schemaColumns converts system_schema rows to the schema shape. The data
// type is the CQL type text exactly as Cassandra stores it, so collections
// and frozen user types keep their full spelling. Key columns can never be
// null, which is what PrimaryKey and Nullable reflect.
func schemaColumns(cols []columnInfo) []schemaColumn {
	out := make([]schemaColumn, 0, len(cols))
	for _, c := range cols {
		isKey := c.Kind == kindPartitionKey || c.Kind == kindClustering
		col := schemaColumn{
			Column: pluginsdk.Column{
				Name:       c.Name,
				DataType:   c.Type,
				Nullable:   !isKey,
				PrimaryKey: isKey,
			},
			Kind:     c.Kind,
			Position: c.Position,
		}
		if c.Kind == kindClustering && c.ClusteringOrder != "none" {
			col.ClusteringOrder = c.ClusteringOrder
		}
		out = append(out, col)
	}
	return out
}

// viewQuery rebuilds the SELECT a materialized view was defined with.
// Cassandra keeps only the pieces (base table, column list, filter), not
// the original statement.
func viewQuery(keyspace string, v viewInfo, cols []columnInfo) string {
	selected := "*"
	if !v.IncludeAllColumns && len(cols) > 0 {
		names := make([]string, 0, len(cols))
		for _, c := range cols {
			names = append(names, cqlIdent(c.Name))
		}
		selected = strings.Join(names, ", ")
	}

	query := fmt.Sprintf("SELECT %s FROM %s.%s", selected, cqlIdent(keyspace), cqlIdent(v.BaseTable))
	if v.WhereClause != "" {
		query += " WHERE " + v.WhereClause
	}
	return query
}

// cqlIdent renders an identifier for display: bare when it is a plain
// lowercase name, quoted when Cassandra would need the quotes to find it.
func cqlIdent(name string) string {
	if plainIdentifier.MatchString(name) {
		return name
	}
	return quoteIdent(name)
}

var plainIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// quoteIdent double-quotes a CQL identifier so names that are keywords or
// mixed case interpolate safely. CQL cannot bind an identifier as a
// parameter, so the name has to be quoted by hand.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
