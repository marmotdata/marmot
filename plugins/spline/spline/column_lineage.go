package spline

import (
	"context"
	"sort"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// Column lineage is carried on a direct edge from each input table to the
// output table, with the pipeline as the edge's job. The FEEDS and PRODUCES
// edges through the pipeline stay as they are: a pipeline has no columns, so
// they cannot carry a column mapping themselves.

// columnLineage resolves, for every column an execution plan wrote, the input
// columns it was derived from, keyed by the MRN of the input they were read
// from. Spline answers one column per request, so the number of lookups is
// capped.
func (s *Source) columnLineage(ctx context.Context, detailed *LineageDetailed) map[string][]pluginsdk.ColumnEdge {
	plan := &detailed.ExecutionPlan

	attributes, written := s.writtenColumns(ctx, detailed)
	if len(attributes) > maxAttributesPerPlan {
		log.Warn().
			Str("plan_id", plan.ID).
			Int("attributes", len(attributes)).
			Int("limit", maxAttributesPerPlan).
			Msg("Execution plan has more columns than the lookup limit, column lineage is truncated")
		attributes = attributes[:maxAttributesPerPlan]
	}

	inputs := newInputResolver(s.client, detailed)
	byInput := make(map[string][]pluginsdk.ColumnEdge)

	for _, attribute := range attributes {
		if attribute.ID == "" || attribute.Name == "" {
			continue
		}

		result, err := s.client.GetAttributeLineage(ctx, attribute.ID)
		if err != nil {
			log.Warn().Err(err).Str("attribute", attribute.Name).Msg("Failed to read column lineage")
			continue
		}

		origins, derived := originColumns(result.Lineage, attribute.ID)
		// Without the write's schema the attribute list also holds every column
		// the plan merely read, so a column with no derivation cannot be told
		// apart from one that was never written.
		if !derived && !written {
			continue
		}

		sources := make(map[string][]string)
		for _, origin := range origins {
			inputMRN, ok := inputs.resolve(ctx, origin.OriginOpID)
			if !ok {
				continue
			}
			sources[inputMRN] = appendUnique(sources[inputMRN], origin.Name)
		}

		for _, inputMRN := range sortedKeys(sources) {
			from := sources[inputMRN]
			sort.Strings(from)
			byInput[inputMRN] = append(byInput[inputMRN], pluginsdk.ColumnEdge{
				FromColumns: from,
				ToColumn:    attribute.Name,
				Transform:   transformOf(from, attribute.Name, derived),
			})
		}
	}

	return byInput
}

// writtenColumns returns the columns the plan's write operation wrote, and
// true. When the write cannot be read it falls back to the plan's attribute
// list, which holds every column of the plan, and reports false.
func (s *Source) writtenColumns(ctx context.Context, detailed *LineageDetailed) ([]PlanAttribute, bool) {
	for _, node := range detailed.Graph.Nodes {
		if node.Type != "Write" || node.ID == "" {
			continue
		}
		details, err := s.client.GetOperation(ctx, node.ID)
		if err != nil {
			log.Debug().Err(err).Str("operation", node.ID).Msg("Failed to read the write operation, using the plan's attributes")
			break
		}
		if len(details.Inputs) > 0 && details.Inputs[0] >= 0 && details.Inputs[0] < len(details.Schemas) {
			return details.Schemas[details.Inputs[0]], true
		}
		break
	}
	return detailed.ExecutionPlan.Extra.Attributes, false
}

// transformOf labels a derived column "expression". A single source column of
// the same name is a copy, whatever path it took through the plan.
func transformOf(from []string, to string, derived bool) string {
	if !derived {
		return ""
	}
	if len(from) == 1 && strings.EqualFold(from[0], to) {
		return ""
	}
	return "expression"
}

// originColumns walks the attribute graph from rootID and returns the columns
// it was ultimately read from: the nodes with nothing further to follow.
// Spline points an edge from a column to the column it was derived from, so
// following edges forward finds its sources. derived is false when the root
// itself has no sources, meaning the column was written as it was read.
func originColumns(graph AttributeGraph, rootID string) (origins []AttributeNode, derived bool) {
	nodes := make(map[string]AttributeNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}

	outgoing := make(map[string][]string, len(graph.Edges))
	for _, edge := range graph.Edges {
		outgoing[edge.Source] = append(outgoing[edge.Source], edge.Target)
	}

	visited := map[string]struct{}{rootID: {}}
	queue := []string{rootID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		next := outgoing[current]
		if len(next) == 0 {
			if node, ok := nodes[current]; ok && node.Name != "" {
				origins = append(origins, node)
			}
			continue
		}
		for _, target := range next {
			if _, seen := visited[target]; seen {
				continue
			}
			visited[target] = struct{}{}
			queue = append(queue, target)
		}
	}

	sort.Slice(origins, func(i, j int) bool { return origins[i].Name < origins[j].Name })
	return origins, len(outgoing[rootID]) > 0
}

// inputResolver maps a Read operation of a plan to the MRN of the input it
// read. A plan with a single input needs no lookups; otherwise each Read
// operation is asked once for its input source.
type inputResolver struct {
	client *Client
	reads  map[string]struct{}
	// single is the MRN of the plan's only input, when it has exactly one.
	single   string
	multiple bool
	cache    map[string]string
}

func newInputResolver(client *Client, detailed *LineageDetailed) *inputResolver {
	r := &inputResolver{
		client: client,
		reads:  make(map[string]struct{}),
		cache:  make(map[string]string),
	}

	for _, node := range detailed.Graph.Nodes {
		if node.Type == "Read" {
			r.reads[node.ID] = struct{}{}
		}
	}

	var sources []string
	for _, input := range detailed.ExecutionPlan.Inputs {
		sources = appendUnique(sources, input.Source)
	}
	switch {
	case len(sources) == 1:
		if ref, ok := parseDataSourceURI(sources[0]); ok {
			r.single = nativeMRN(ref)
		}
	case len(sources) > 1:
		r.multiple = true
	}

	return r
}

// resolve reports the MRN of the input a Read operation read. A column that
// originates anywhere else, such as a literal, has no input column to map.
func (r *inputResolver) resolve(ctx context.Context, operationID string) (string, bool) {
	if _, ok := r.reads[operationID]; !ok {
		return "", false
	}
	if !r.multiple {
		return r.single, r.single != ""
	}
	if inputMRN, ok := r.cache[operationID]; ok {
		return inputMRN, inputMRN != ""
	}

	inputMRN := ""
	details, err := r.client.GetOperation(ctx, operationID)
	if err != nil {
		log.Debug().Err(err).Str("operation", operationID).Msg("Failed to read a read operation, skipping its columns")
	} else if sources := details.Operation.Properties.InputSources; len(sources) == 1 {
		if ref, ok := parseDataSourceURI(sources[0]); ok {
			inputMRN = nativeMRN(ref)
		}
	}

	r.cache[operationID] = inputMRN
	return inputMRN, inputMRN != ""
}

// addColumnEdge records a column-carrying edge, merging the columns into an
// edge already recorded between the same two assets by another plan.
func addColumnEdge(edges *[]pluginsdk.LineageEdge, index map[string]int, edge pluginsdk.LineageEdge) {
	key := edge.Source + "|" + edge.Target
	position, ok := index[key]
	if !ok {
		index[key] = len(*edges)
		*edges = append(*edges, edge)
		return
	}

	existing := &(*edges)[position]
	have := make(map[string]struct{}, len(existing.ColumnLineage))
	for _, column := range existing.ColumnLineage {
		have[column.ToColumn] = struct{}{}
	}
	for _, column := range edge.ColumnLineage {
		if _, ok := have[column.ToColumn]; !ok {
			existing.ColumnLineage = append(existing.ColumnLineage, column)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
