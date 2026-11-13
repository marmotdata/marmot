package openmetadata

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// OpenMetadata has no endpoint that dumps every lineage edge, so lineage
// is read one entity at a time. Asking each entity for its immediate
// neighbours reaches every edge from both ends, which is why the results
// are deduplicated by source and target.

// discoverLineage fetches lineage for every asset that came from an
// OpenMetadata entity and turns the edges into Marmot lineage.
func (c *collector) discoverLineage(ctx context.Context, client *client) {
	targets := c.lineageTargets()
	if len(targets) == 0 {
		return
	}

	var (
		mu       sync.Mutex
		edges    = make(map[string]pluginsdk.LineageEdge)
		failures int
		wg       sync.WaitGroup
		sem      = make(chan struct{}, c.config.Concurrency)
	)

	for _, target := range targets {
		wg.Add(1)
		go func(t lineageTarget) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			resp, err := client.lineageOf(ctx, t.kind, t.id)
			if err != nil {
				log.Debug().Err(err).Str("entity", t.id).Msg("Failed to read lineage")
				mu.Lock()
				failures++
				mu.Unlock()
				return
			}

			mu.Lock()
			defer mu.Unlock()
			c.collectEdges(resp, edges)
		}(target)
	}

	wg.Wait()

	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		c.lineage = append(c.lineage, edges[key])
	}

	// A credentials or connectivity problem shows up here as every
	// request failing, which would otherwise look like an instance with
	// no lineage at all.
	if failures > 0 {
		log.Warn().
			Int("failed", failures).
			Int("of", len(targets)).
			Msg("Some entities could not be asked for lineage, so the imported graph is incomplete")
	}

	log.Debug().Int("count", len(edges)).Msg("Discovered lineage edges")
}

// lineageTarget is one entity to ask OpenMetadata about.
type lineageTarget struct {
	kind string
	id   string
}

// lineageTargets lists the entities worth asking for lineage. Only the
// kinds OpenMetadata records lineage between are asked, and only the
// ones this run actually catalogued.
func (c *collector) lineageTargets() []lineageTarget {
	targets := make([]lineageTarget, 0, len(c.lineageKinds))
	for id, kind := range c.lineageKinds {
		if _, ok := c.mrnByID[id]; ok {
			targets = append(targets, lineageTarget{kind: kind, id: id})
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].id < targets[j].id })
	return targets
}

// collectEdges converts one lineage response into Marmot edges, keyed so
// the same edge seen from both endpoints is stored once.
func (c *collector) collectEdges(resp *lineageResponse, edges map[string]pluginsdk.LineageEdge) {
	// Column lineage names columns by FQN, so the FQN of each end is needed
	// to tell the column name apart from the table it belongs to.
	fqnByID := make(map[string]string, len(resp.Nodes)+1)
	for _, node := range append([]entityRef{resp.Entity}, resp.Nodes...) {
		if node.ID != "" && node.FullyQualifiedName != "" {
			fqnByID[node.ID] = node.FullyQualifiedName
		}
	}

	for _, edge := range append(resp.UpstreamEdges, resp.DownstreamEdges...) {
		source, ok := c.mrnByID[edge.FromEntity]
		if !ok {
			continue
		}
		target, ok := c.mrnByID[edge.ToEntity]
		if !ok {
			continue
		}
		if source == target {
			continue
		}

		lineageEdge := pluginsdk.LineageEdge{
			Source: source,
			Target: target,
			Type:   "DEPENDS_ON",
		}

		// OpenMetadata records which pipeline moved the data. Marmot
		// carries that on the edge, so the graph shows the job.
		if edge.Details != nil && edge.Details.Pipeline != nil {
			if jobMRN, ok := c.mrnByID[edge.Details.Pipeline.ID]; ok {
				lineageEdge.JobMRN = jobMRN
			}
		}
		if edge.Details != nil {
			lineageEdge.ColumnLineage = columnEdges(edge.Details.ColumnsLineage, fqnByID[edge.FromEntity], fqnByID[edge.ToEntity])
		}

		// The same edge is seen from both ends; a view that lost its
		// column lineage must not replace one that has it.
		key := source + "\x00" + target
		if existing, seen := edges[key]; seen && len(existing.ColumnLineage) > len(lineageEdge.ColumnLineage) {
			continue
		}
		edges[key] = lineageEdge
	}
}

// columnEdges converts OpenMetadata's column lineage into Marmot's. Columns
// are named by FQN; the table FQN is stripped to leave the column name. A
// source column that does not belong to the edge's source table is dropped,
// and a target column without a source column says nothing and is skipped.
func columnEdges(columns []columnLineage, fromFQN, toFQN string) []pluginsdk.ColumnEdge {
	var out []pluginsdk.ColumnEdge
	for _, column := range columns {
		to, ok := columnName(column.ToColumn, toFQN)
		if !ok {
			continue
		}

		var from []string
		for _, fromColumn := range column.FromColumns {
			if name, ok := columnName(fromColumn, fromFQN); ok && !slices.Contains(from, name) {
				from = append(from, name)
			}
		}
		if len(from) == 0 {
			continue
		}

		out = append(out, pluginsdk.ColumnEdge{
			FromColumns: from,
			ToColumn:    to,
			Transform:   strings.TrimSpace(column.Function),
		})
	}
	return out
}

// columnName returns the name of a column given its FQN and the FQN of the
// table it should belong to. A nested column keeps its parent's name, as
// in address.city. When the table's FQN is not known the last FQN part is
// the best name available.
func columnName(columnFQN, tableFQN string) (string, bool) {
	if columnFQN == "" {
		return "", false
	}
	if tableFQN == "" {
		parts := splitFQN(columnFQN)
		name := parts[len(parts)-1]
		return name, name != ""
	}

	rest, ok := strings.CutPrefix(columnFQN, tableFQN+".")
	if !ok || rest == "" {
		return "", false
	}
	return strings.Join(splitFQN(rest), "."), true
}

// entityKindOf maps an OpenMetadata entity kind to the path segment the
// lineage endpoint expects. Kinds absent from this list do not
// participate in OpenMetadata lineage.
var lineageKinds = map[string]bool{
	"table":              true,
	"topic":              true,
	"dashboard":          true,
	"dashboardDataModel": true,
	"mlmodel":            true,
	"container":          true,
	"searchIndex":        true,
	"pipeline":           true,
	"storedProcedure":    true,
	"apiEndpoint":        true,
	"worksheet":          true,
	"spreadsheet":        true,
	"file":               true,
	"directory":          true,
}

// trackForLineage remembers that an entity can be asked for lineage.
func (c *collector) trackForLineage(kind, id string) {
	if id == "" || !lineageKinds[strings.TrimSpace(kind)] {
		return
	}
	c.lineageKinds[id] = kind
}
