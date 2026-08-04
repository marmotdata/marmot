package openmetadata

import (
	"context"
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

		edges[source+"\x00"+target] = lineageEdge
	}
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
}

// trackForLineage remembers that an entity can be asked for lineage.
func (c *collector) trackForLineage(kind, id string) {
	if id == "" || !lineageKinds[strings.TrimSpace(kind)] {
		return
	}
	c.lineageKinds[id] = kind
}
