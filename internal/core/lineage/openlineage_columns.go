package lineage

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

// The columnLineage facet on an output says which input columns each of its columns came from. Marmot keeps column mappings on a direct edge from the input dataset to the output dataset, naming the job that moved the data, alongside the input → job → output edges.

type datasetKey struct{ namespace, name string }

// columnSources is, for one input dataset, the mapping onto each output column.
type columnSources map[string]*ColumnEdge

// createColumnLineage writes a direct input → output edge with column mappings for every output that has a columnLineage facet. An input the facet names but the event does not list, as with duck_lineage's COMPLETE events which repeat the outputs only, is looked up in the run's earlier events so it resolves the same way it did there.
func (s *service) createColumnLineage(ctx context.Context, event *RunEvent, job filedJob, inputs, outputs []filedDataset) error {
	known := map[datasetKey]filedDataset{}
	for _, in := range inputs {
		known[datasetKey{in.dataset.Namespace, in.dataset.Name}] = in
	}
	var earlier []Dataset
	loadedEarlier := false

	for _, out := range outputs {
		bySource := columnMappings(out.dataset)
		if len(bySource) == 0 {
			continue
		}
		keys := make([]datasetKey, 0, len(bySource))
		for k := range bySource {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].namespace+"\x00"+keys[i].name < keys[j].namespace+"\x00"+keys[j].name
		})
		for _, k := range keys {
			in, ok := known[k]
			if !ok {
				if !loadedEarlier && event.Kind() == KindRun {
					var err error
					earlier, err = s.repo.RunInputs(ctx, event.Run.RunID)
					if err != nil {
						log.Warn().Err(err).Str("run_id", event.Run.RunID).Msg("Failed to read the run's earlier inputs")
					}
					loadedEarlier = true
				}
				ds := &Dataset{Namespace: k.namespace, Name: k.name}
				for i := range earlier {
					if earlier[i].Namespace == k.namespace && earlier[i].Name == k.name {
						ds = &earlier[i]
						break
					}
				}
				m, err := s.upsertDataset(ctx, ds, event.Producer, "openlineage")
				if err != nil {
					log.Warn().Err(err).Str("dataset", k.namespace+"/"+k.name).Msg("Failed to file a column lineage input")
					continue
				}
				in = filedDataset{dataset: ds, mrn: m}
				known[k] = in
			}
			if in.mrn == out.mrn {
				// A statement that reads and writes one table maps its columns onto themselves; there is no edge to draw.
				continue
			}
			columns := bySource[k].edges()
			if _, err := s.CreateDeclaredLineage(ctx, in.mrn, out.mrn, "", job.mrn, openLineageWriter, columns); err != nil {
				log.Warn().Err(err).Str("source", in.mrn).Str("target", out.mrn).Msg("Failed to create column lineage")
			}
		}
	}
	return nil
}

// unresolvedColumn matches the internal binding references, such as #[4.0], that duck_lineage sends as output column names when it could not work out the column's name.
var unresolvedColumn = regexp.MustCompile(`^#\[\d+(\.\d+)*\]$`)

// columnMappings reads an output's columnLineage facet into the column mappings from each input dataset. An output column the producer could not name is skipped. Columns missing from the output's schema facet are kept, since producers such as dbt describe only the columns that are documented.
func columnMappings(out *Dataset) map[datasetKey]columnSources {
	cl := facetMap(out.Facets, "columnLineage")
	fields, _ := cl["fields"].(map[string]interface{})
	if len(fields) == 0 {
		return nil
	}

	result := map[datasetKey]columnSources{}
	for outCol, raw := range fields {
		if unresolvedColumn.MatchString(outCol) {
			continue
		}
		spec, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		// Before facet 1-2-0 the transformation was described once per output column.
		legacyType, _ := spec["transformationType"].(string)
		legacyDesc, _ := spec["transformationDescription"].(string)
		list, _ := spec["inputFields"].([]interface{})
		for _, item := range list {
			f, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			ns, _ := f["namespace"].(string)
			name, _ := f["name"].(string)
			field, _ := f["field"].(string)
			if ns == "" || name == "" || field == "" {
				continue
			}
			k := datasetKey{ns, name}
			if result[k] == nil {
				result[k] = columnSources{}
			}
			edge := result[k][outCol]
			if edge == nil {
				edge = &ColumnEdge{ToColumn: outCol}
				result[k][outCol] = edge
			}
			if !contains(edge.FromColumns, field) {
				edge.FromColumns = append(edge.FromColumns, field)
			}
			for _, t := range transformationLabels(f, legacyType, legacyDesc) {
				edge.Transform = appendLabel(edge.Transform, t)
			}
		}
	}
	return result
}

// transformationLabels describes how an input field became the output column: the subtype of each transformation, or its type when it has none, with its description and a note when the value is masked. A plain copy, IDENTITY or a bare DIRECT, is no transformation and gets no label, so only columns that really changed are marked as transformed. Producers on facets before 1-2-0 describe it once per output column instead.
func transformationLabels(field map[string]interface{}, legacyType, legacyDesc string) []string {
	var out []string
	list, _ := field["transformations"].([]interface{})
	for _, item := range list {
		t, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		label, _ := t["subtype"].(string)
		if label == "" {
			label, _ = t["type"].(string)
		}
		desc, _ := t["description"].(string)
		masked, _ := t["masking"].(bool)
		if isPlainCopy(label) && desc == "" && !masked {
			continue
		}
		label = strings.TrimSpace(label + " " + desc)
		if masked {
			label = strings.TrimSpace(label + " (masked)")
		}
		if label != "" {
			out = append(out, label)
		}
	}
	if len(list) == 0 && !(isPlainCopy(legacyType) && legacyDesc == "") {
		if label := strings.TrimSpace(legacyType + " " + legacyDesc); label != "" {
			out = append(out, label)
		}
	}
	return out
}

// isPlainCopy reports whether a transformation name says the value was carried over unchanged.
func isPlainCopy(label string) bool {
	return strings.EqualFold(label, "IDENTITY") || strings.EqualFold(label, "DIRECT")
}

func appendLabel(existing, label string) string {
	if existing == "" {
		return label
	}
	for _, part := range strings.Split(existing, ", ") {
		if part == label {
			return existing
		}
	}
	return existing + ", " + label
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// edges lists the column mappings in output column order, so the stored edge is the same whichever order the facet's map was read in.
func (c columnSources) edges() []ColumnEdge {
	names := make([]string, 0, len(c))
	for n := range c {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]ColumnEdge, 0, len(names))
	for _, n := range names {
		e := *c[n]
		sort.Strings(e.FromColumns)
		out = append(out, e)
	}
	return out
}
