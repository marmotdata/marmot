package timescale

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// timescaleInfo is everything the TimescaleDB catalog knows about one
// database, keyed by schema-qualified object name.
type timescaleInfo struct {
	Version     string
	hypertables map[string]*hypertable
	aggregates  map[string]*aggregate
	sizes       map[string]objectSize
}

// hypertable is a table TimescaleDB partitions into chunks.
type hypertable struct {
	Schema             string
	Name               string
	Owner              string
	NumDimensions      int
	NumChunks          int64
	CompressionEnabled bool
	IsDistributed      *bool
	Tablespaces        []string
	Dimensions         []dimension
	SegmentBy          []string
	OrderBy            []string
	Policies           []map[string]interface{}
	ChunkRange         *chunkRange
}

func (h hypertable) key() string { return h.Schema + "." + h.Name }

// dimension is one partitioning column of a hypertable. Dimension 1 is
// always the time dimension; higher numbers are space partitions.
type dimension struct {
	Number          int64
	ColumnName      string
	ColumnType      string
	Kind            string
	IntervalSeconds *int64
	IntegerInterval *int64
	IntegerNowFunc  string
	NumPartitions   *int
}

// aggregate is a continuous aggregate: a view whose results TimescaleDB keeps
// materialized in a hidden hypertable and refreshes on a schedule.
type aggregate struct {
	ViewSchema            string
	ViewName              string
	MaterializationSchema string
	MaterializationName   string
	Definition            string
	CompressionEnabled    bool
	MaterializedOnly      bool
	Finalized             *bool
	HypertableSchema      string
	HypertableName        string
	Policies              []map[string]interface{}
}

func (a aggregate) key() string { return a.ViewSchema + "." + a.ViewName }

// chunkRange is the time span covered by a hypertable's chunks.
type chunkRange struct {
	Oldest string
	Newest string
}

// objectSize is what TimescaleDB reports for a hypertable, which is not what
// PostgreSQL reports for its parent table.
type objectSize struct {
	Rows  *int64
	Bytes *int64
}

// readTimescale reads the TimescaleDB catalog of the connected database, or
// returns nil when the extension is not installed there.
func (s *Source) readTimescale(ctx context.Context) (*timescaleInfo, error) {
	version, err := s.extensionVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking for the timescaledb extension: %w", err)
	}
	if version == "" {
		log.Debug().Msg("The timescaledb extension is not installed, discovering as plain PostgreSQL")
		return nil, nil
	}

	log.Debug().Str("version", version).Msg("Found the timescaledb extension")

	ts := &timescaleInfo{
		Version:     version,
		hypertables: make(map[string]*hypertable),
		aggregates:  make(map[string]*aggregate),
		sizes:       make(map[string]objectSize),
	}

	// TimescaleDB changes its catalog views between minor versions, so the
	// optional columns are looked up once rather than assumed.
	columns := s.catalogColumns(ctx)

	// Policies belong to both hypertables and continuous aggregates, so they
	// are read once and handed to each.
	policies := s.readPolicies(ctx)

	if s.config.IncludeHypertables {
		if err := s.readHypertables(ctx, ts, columns, policies); err != nil {
			log.Warn().Err(err).Msg("Failed to read hypertables")
		}
	}

	if s.config.IncludeContinuousAggregates {
		if err := s.readAggregates(ctx, ts, columns, policies); err != nil {
			log.Warn().Err(err).Msg("Failed to read continuous aggregates")
		}
	}

	return ts, nil
}

func (s *Source) extensionVersion(ctx context.Context) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, `SELECT extversion FROM pg_catalog.pg_extension WHERE extname = 'timescaledb'`)
	if err != nil {
		return "", fmt.Errorf("querying pg_extension: %w", err)
	}
	defer rows.Close()

	var version string
	for rows.Next() {
		if err := rows.Scan(&version); err != nil {
			return "", fmt.Errorf("scanning extension version: %w", err)
		}
	}
	return version, rows.Err()
}

// catalogColumns lists the columns of every timescaledb_information view.
// Columns come and go between TimescaleDB releases: is_distributed was
// dropped when multi-node was removed, and finalized when every continuous
// aggregate became finalized.
func (s *Source) catalogColumns(ctx context.Context) map[string]map[string]bool {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result := make(map[string]map[string]bool)

	rows, err := s.pool.Query(queryCtx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = 'timescaledb_information'
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list the TimescaleDB catalog columns")
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var view, column string
		if err := rows.Scan(&view, &column); err != nil {
			continue
		}
		if result[view] == nil {
			result[view] = make(map[string]bool)
		}
		result[view][column] = true
	}

	return result
}

// optionalColumn selects a column when the running TimescaleDB has it, and a
// typed NULL when it does not, so the scan target stays the same either way.
func optionalColumn(columns map[string]map[string]bool, view, name, sqlType string) string {
	if columns[view][name] {
		return name
	}
	return "NULL::" + sqlType
}

func (s *Source) readHypertables(ctx context.Context, ts *timescaleInfo, columns map[string]map[string]bool, policies map[string][]map[string]interface{}) error {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// tablespaces is a name[]; joining it into a string here avoids depending
	// on array decoding over the simple protocol.
	query := fmt.Sprintf(`
		SELECT
			hypertable_schema,
			hypertable_name,
			owner,
			num_dimensions::int,
			num_chunks::bigint,
			compression_enabled,
			COALESCE(array_to_string(tablespaces, ','), '') AS tablespaces,
			%s AS is_distributed
		FROM timescaledb_information.hypertables
		ORDER BY hypertable_schema, hypertable_name
	`, optionalColumn(columns, "hypertables", "is_distributed", "boolean"))

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return fmt.Errorf("querying hypertables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		ht := &hypertable{}
		var tablespaces string

		if err := rows.Scan(
			&ht.Schema, &ht.Name, &ht.Owner, &ht.NumDimensions,
			&ht.NumChunks, &ht.CompressionEnabled, &tablespaces, &ht.IsDistributed,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan hypertable row")
			continue
		}

		if isInternalSchema(ht.Schema) {
			continue
		}
		if tablespaces != "" {
			ht.Tablespaces = strings.Split(tablespaces, ",")
		}
		ht.Policies = policies[ht.key()]

		ts.hypertables[ht.key()] = ht
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating hypertable rows: %w", err)
	}

	if len(ts.hypertables) == 0 {
		return nil
	}

	s.readDimensions(ctx, ts)
	if s.config.IncludeCompression {
		s.readCompression(ctx, ts)
	}
	if s.config.IncludeChunks {
		s.readChunkRanges(ctx, ts)
	}
	s.readSizes(ctx, ts)

	log.Debug().Int("count", len(ts.hypertables)).Msg("Discovered hypertables")
	return nil
}

func (s *Source) readDimensions(ctx context.Context, ts *timescaleInfo) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// The interval is reduced to seconds so it can be rendered the same way
	// whatever units it was declared in.
	rows, err := s.pool.Query(queryCtx, `
		SELECT
			hypertable_schema,
			hypertable_name,
			dimension_number,
			column_name,
			column_type::text,
			COALESCE(dimension_type, '') AS dimension_type,
			EXTRACT(EPOCH FROM time_interval)::bigint AS interval_seconds,
			integer_interval,
			COALESCE(integer_now_func, '') AS integer_now_func,
			num_partitions::int
		FROM timescaledb_information.dimensions
		ORDER BY hypertable_schema, hypertable_name, dimension_number
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read hypertable dimensions")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, name string
		var d dimension

		if err := rows.Scan(
			&schema, &name, &d.Number, &d.ColumnName, &d.ColumnType, &d.Kind,
			&d.IntervalSeconds, &d.IntegerInterval, &d.IntegerNowFunc, &d.NumPartitions,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan dimension row")
			continue
		}

		// The dimensions view also lists the hidden materialization
		// hypertables behind continuous aggregates.
		ht, ok := ts.hypertables[schema+"."+name]
		if !ok {
			continue
		}
		ht.Dimensions = append(ht.Dimensions, d)
	}
}

func (s *Source) readCompression(ctx context.Context, ts *timescaleInfo) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, `
		SELECT
			hypertable_schema,
			hypertable_name,
			attname,
			segmentby_column_index::int,
			orderby_column_index::int,
			COALESCE(orderby_asc, false) AS orderby_asc,
			COALESCE(orderby_nullsfirst, false) AS orderby_nullsfirst
		FROM timescaledb_information.compression_settings
		ORDER BY hypertable_schema, hypertable_name, segmentby_column_index, orderby_column_index
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read compression settings")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, name, attname string
		var segmentByIndex, orderByIndex *int
		var asc, nullsFirst bool

		if err := rows.Scan(&schema, &name, &attname, &segmentByIndex, &orderByIndex, &asc, &nullsFirst); err != nil {
			log.Warn().Err(err).Msg("Failed to scan compression setting row")
			continue
		}

		ht, ok := ts.hypertables[schema+"."+name]
		if !ok {
			continue
		}

		// A column is listed once per role: it either groups the compressed
		// batches or orders the rows inside them.
		if segmentByIndex != nil {
			ht.SegmentBy = append(ht.SegmentBy, attname)
		}
		if orderByIndex != nil {
			ht.OrderBy = append(ht.OrderBy, formatOrderBy(attname, asc, nullsFirst))
		}
	}
}

func (s *Source) readChunkRanges(ctx context.Context, ts *timescaleInfo) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// A hypertable partitioned on an integer column reports its range in
	// range_start_integer instead of range_start.
	rows, err := s.pool.Query(queryCtx, `
		SELECT
			hypertable_schema,
			hypertable_name,
			COALESCE(min(range_start)::text, min(range_start_integer)::text, '') AS oldest,
			COALESCE(max(range_end)::text, max(range_end_integer)::text, '') AS newest
		FROM timescaledb_information.chunks
		GROUP BY 1, 2
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read chunk ranges")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, name, oldest, newest string
		if err := rows.Scan(&schema, &name, &oldest, &newest); err != nil {
			log.Warn().Err(err).Msg("Failed to scan chunk range row")
			continue
		}

		ht, ok := ts.hypertables[schema+"."+name]
		if !ok {
			continue
		}
		ht.ChunkRange = &chunkRange{Oldest: oldest, Newest: newest}
	}
}

// readSizes reads TimescaleDB's own row and size estimates. A hypertable's
// parent table holds no rows, so PostgreSQL's reltuples reports zero for it
// and pg_total_relation_size counts only the empty parent.
func (s *Source) readSizes(ctx context.Context, ts *timescaleInfo) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := s.pool.Query(queryCtx, `
		SELECT
			h.hypertable_schema,
			h.hypertable_name,
			approximate_row_count(format('%I.%I', h.hypertable_schema, h.hypertable_name)::regclass)::bigint AS rows,
			hypertable_size(format('%I.%I', h.hypertable_schema, h.hypertable_name)::regclass)::bigint AS bytes
		FROM timescaledb_information.hypertables h
	`)
	if err != nil {
		// Falling back leaves the PostgreSQL numbers in place, which
		// understate a hypertable but are never wrong for a plain table.
		log.Warn().Err(err).Msg("Failed to read hypertable sizes, falling back to the PostgreSQL estimates")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var schema, name string
		var size objectSize

		if err := rows.Scan(&schema, &name, &size.Rows, &size.Bytes); err != nil {
			log.Warn().Err(err).Msg("Failed to scan hypertable size row")
			continue
		}
		if _, ok := ts.hypertables[schema+"."+name]; !ok {
			continue
		}
		ts.sizes[schema+"."+name] = size
	}
}

// readPolicies reads the background jobs attached to a hypertable or a
// continuous aggregate: compression, retention and refresh.
func (s *Source) readPolicies(ctx context.Context) map[string][]map[string]interface{} {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result := make(map[string][]map[string]interface{})

	// config is jsonb; reading it as text and decoding here avoids depending
	// on jsonb decoding over the simple protocol.
	rows, err := s.pool.Query(queryCtx, `
		SELECT
			job_id,
			application_name,
			EXTRACT(EPOCH FROM schedule_interval)::bigint AS schedule_seconds,
			hypertable_schema,
			hypertable_name,
			proc_name,
			COALESCE(config::text, '') AS config,
			scheduled
		FROM timescaledb_information.jobs
		WHERE hypertable_name IS NOT NULL
		ORDER BY job_id
	`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read TimescaleDB policies")
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var (
			jobID           int64
			name            string
			scheduleSeconds *int64
			schema, table   string
			proc            string
			configJSON      string
			scheduled       bool
		)

		if err := rows.Scan(&jobID, &name, &scheduleSeconds, &schema, &table, &proc, &configJSON, &scheduled); err != nil {
			log.Warn().Err(err).Msg("Failed to scan policy row")
			continue
		}

		policy := map[string]interface{}{
			"job_id":    jobID,
			"name":      name,
			"proc":      proc,
			"scheduled": scheduled,
		}
		if scheduleSeconds != nil {
			policy["schedule"] = formatInterval(*scheduleSeconds)
		}
		if configJSON != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
				log.Warn().Err(err).Int64("job_id", jobID).Msg("Failed to decode policy config")
			} else {
				policy["config"] = config
			}
		}

		key := schema + "." + table
		result[key] = append(result[key], policy)
	}

	return result
}

func (s *Source) readAggregates(ctx context.Context, ts *timescaleInfo, columns map[string]map[string]bool, policies map[string][]map[string]interface{}) error {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	query := fmt.Sprintf(`
		SELECT
			view_schema,
			view_name,
			materialization_hypertable_schema,
			materialization_hypertable_name,
			COALESCE(view_definition, '') AS view_definition,
			compression_enabled,
			materialized_only,
			hypertable_schema,
			hypertable_name,
			%s AS finalized
		FROM timescaledb_information.continuous_aggregates
		ORDER BY view_schema, view_name
	`, optionalColumn(columns, "continuous_aggregates", "finalized", "boolean"))

	rows, err := s.pool.Query(queryCtx, query)
	if err != nil {
		return fmt.Errorf("querying continuous aggregates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		agg := &aggregate{}

		if err := rows.Scan(
			&agg.ViewSchema, &agg.ViewName,
			&agg.MaterializationSchema, &agg.MaterializationName,
			&agg.Definition, &agg.CompressionEnabled, &agg.MaterializedOnly,
			&agg.HypertableSchema, &agg.HypertableName, &agg.Finalized,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan continuous aggregate row")
			continue
		}

		agg.Definition = strings.TrimSpace(agg.Definition)
		// A refresh policy is registered against the aggregate's view name,
		// not against the hypertable it reads.
		agg.Policies = policies[agg.key()]

		ts.aggregates[agg.key()] = agg
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating continuous aggregate rows: %w", err)
	}

	log.Debug().Int("count", len(ts.aggregates)).Msg("Discovered continuous aggregates")
	return nil
}

// applyHypertableMetadata records what makes a table a hypertable: how it is
// partitioned, how much of it there is, and what runs against it.
func applyHypertableMetadata(metadata map[string]interface{}, ht *hypertable, ts *timescaleInfo, includeChunks bool) {
	metadata["hypertable"] = true
	metadata["timescaledb_version"] = ts.Version
	metadata["num_dimensions"] = ht.NumDimensions
	metadata["num_chunks"] = ht.NumChunks
	metadata["compression_enabled"] = ht.CompressionEnabled

	if ht.IsDistributed != nil {
		metadata["is_distributed"] = *ht.IsDistributed
	}
	if len(ht.Tablespaces) > 0 {
		metadata["tablespaces"] = ht.Tablespaces
	}

	for _, d := range ht.Dimensions {
		if d.Number != 1 {
			continue
		}
		metadata["time_column"] = d.ColumnName
		metadata["time_column_type"] = d.ColumnType
		if d.IntervalSeconds != nil {
			metadata["time_interval"] = formatInterval(*d.IntervalSeconds)
			metadata["time_interval_seconds"] = *d.IntervalSeconds
		}
		if d.IntegerInterval != nil {
			metadata["integer_interval"] = *d.IntegerInterval
		}
		if d.IntegerNowFunc != "" {
			metadata["integer_now_func"] = d.IntegerNowFunc
		}
	}

	if partitions := spacePartitions(ht.Dimensions); len(partitions) > 0 {
		metadata["space_partitions"] = partitions
	}
	if len(ht.SegmentBy) > 0 {
		metadata["compression_segment_by"] = ht.SegmentBy
	}
	if len(ht.OrderBy) > 0 {
		metadata["compression_order_by"] = ht.OrderBy
	}
	if len(ht.Policies) > 0 {
		metadata["policies"] = ht.Policies
	}
	if includeChunks && ht.ChunkRange != nil {
		metadata["chunk_range"] = map[string]interface{}{
			"oldest": ht.ChunkRange.Oldest,
			"newest": ht.ChunkRange.Newest,
		}
	}
}

// spacePartitions lists the hypertable's partitioning columns beyond time.
func spacePartitions(dimensions []dimension) []map[string]interface{} {
	var partitions []map[string]interface{}
	for _, d := range dimensions {
		if d.Number == 1 {
			continue
		}
		partition := map[string]interface{}{"column": d.ColumnName}
		if d.NumPartitions != nil {
			partition["partitions"] = *d.NumPartitions
		}
		partitions = append(partitions, partition)
	}
	return partitions
}

// applyAggregateMetadata records what makes a view a continuous aggregate.
func applyAggregateMetadata(metadata map[string]interface{}, agg *aggregate, ts *timescaleInfo) {
	metadata["continuous_aggregate"] = true
	metadata["timescaledb_version"] = ts.Version
	metadata["materialized_only"] = agg.MaterializedOnly
	metadata["compression_enabled"] = agg.CompressionEnabled
	metadata["source_hypertable"] = agg.HypertableSchema + "." + agg.HypertableName
	metadata["materialization_hypertable"] = agg.MaterializationSchema + "." + agg.MaterializationName

	if agg.Finalized != nil {
		metadata["finalized"] = *agg.Finalized
	}
	if len(agg.Policies) > 0 {
		metadata["policies"] = agg.Policies
	}
	for _, policy := range agg.Policies {
		if policy["proc"] == "policy_refresh_continuous_aggregate" {
			metadata["refresh_policy"] = policy
			break
		}
	}
}

// formatInterval renders a number of seconds the way a person would write the
// interval, so "1 day" reads as such rather than as PostgreSQL's 24:00:00.
func formatInterval(seconds int64) string {
	if seconds <= 0 {
		return ""
	}

	units := []struct {
		name string
		size int64
	}{
		{"day", 86400},
		{"hour", 3600},
		{"minute", 60},
		{"second", 1},
	}

	var parts []string
	for _, unit := range units {
		count := seconds / unit.size
		if count == 0 {
			continue
		}
		seconds -= count * unit.size

		name := unit.name
		if count != 1 {
			name += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", count, name))
	}

	return strings.Join(parts, " ")
}

// formatOrderBy renders one compression ordering column as it would be
// written in SQL. PostgreSQL sorts nulls last when ascending and first when
// descending, so the NULLS clause is only spelled out when it differs.
func formatOrderBy(column string, asc, nullsFirst bool) string {
	direction := "DESC"
	defaultNullsFirst := true
	if asc {
		direction = "ASC"
		defaultNullsFirst = false
	}

	out := column + " " + direction
	if nullsFirst == defaultNullsFirst {
		return out
	}
	if nullsFirst {
		return out + " NULLS FIRST"
	}
	return out + " NULLS LAST"
}

// fromClausePattern finds the object named directly after FROM or JOIN,
// past an ONLY that excludes inherited tables, optionally schema-qualified
// and optionally double-quoted.
var fromClausePattern = regexp.MustCompile(`(?is)\b(?:from|join)\s+(?:only\s+)?("[^"]+"|[a-z_][a-z0-9_$]*)(?:\s*\.\s*("[^"]+"|[a-z_][a-z0-9_$]*))?`)

// referencedObjects lists the objects a view definition reads from. Only
// names that were actually discovered are returned, so a keyword or a column
// that happens to follow FROM can never become an edge to an asset that does
// not exist. It is a scan rather than a parse: it finds the tables a query
// names directly, not those reached through a nested view or a CTE.
func referencedObjects(definition string, known map[string]bool) []string {
	seen := make(map[string]bool)
	var names []string

	for _, match := range fromClausePattern.FindAllStringSubmatch(definition, -1) {
		// The last captured group is the object name; anything before it is
		// the schema.
		name := match[1]
		if match[2] != "" {
			name = match[2]
		}
		name = strings.Trim(name, `"`)

		if !known[name] || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// viewLineage links every view to the tables it reads. A continuous
// aggregate's source hypertable comes from the TimescaleDB catalog, which
// names it exactly; the rest is read off the definition.
func viewLineage(objects []object, ts *timescaleInfo) []pluginsdk.LineageEdge {
	known := make(map[string]bool, len(objects))
	types := make(map[string]string, len(objects))
	for _, obj := range objects {
		known[obj.Name] = true
		if obj.Kind == kindTable {
			types[obj.Name] = "Table"
		} else {
			types[obj.Name] = "View"
		}
	}

	var lineages []pluginsdk.LineageEdge
	seen := make(map[string]bool)

	for _, obj := range objects {
		if obj.Kind == kindTable {
			continue
		}

		viewMRN := assetMRN("View", obj.Name)
		definition := obj.Definition

		var sources []string
		if ts != nil {
			if agg, ok := ts.aggregates[obj.key()]; ok {
				// PostgreSQL rewrites a continuous aggregate to read from the
				// hidden materialization hypertable, so its own definition
				// names an object this plugin deliberately skips. The catalog
				// keeps the original query and names the source hypertable.
				definition = agg.Definition
				if known[agg.HypertableName] {
					sources = append(sources, agg.HypertableName)
				}
			}
		}
		sources = append(sources, referencedObjects(definition, known)...)

		for _, source := range sources {
			sourceMRN := assetMRN(types[source], source)
			if sourceMRN == viewMRN {
				continue
			}
			key := sourceMRN + ":" + viewMRN
			if seen[key] {
				continue
			}
			seen[key] = true

			// Source is the table the view reads, Target is the view.
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: sourceMRN,
				Target: viewMRN,
				Type:   "VIEW_OF",
			})
		}
	}

	return lineages
}
