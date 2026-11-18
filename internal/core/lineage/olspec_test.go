package lineage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

// The OpenLineage schemas are vendored under testdata/openlineage/spec at a pinned release, so every fixture these tests send can be checked against the official spec without network access.

const (
	specDir       = "testdata/openlineage/spec"
	coreSchemaURL = "https://openlineage.io/spec/2-0-2/OpenLineage.json"
)

// facetPlace is where a facet sits in an event, matching the base type its schema extends.
type facetPlace string

const (
	placeDataset facetPlace = "DatasetFacet"
	placeInput   facetPlace = "InputDatasetFacet"
	placeOutput  facetPlace = "OutputDatasetFacet"
	placeJob     facetPlace = "JobFacet"
	placeRun     facetPlace = "RunFacet"
)

type olSpec struct {
	core   *jsonschema.Schema
	facets map[facetPlace]map[string]*jsonschema.Schema
}

var (
	specOnce   sync.Once
	loadedSpec *olSpec
	specErr    error
)

// openLineageSpec compiles the core schema and every standard facet schema once per test binary.
func openLineageSpec(t testing.TB) *olSpec {
	t.Helper()
	specOnce.Do(func() { loadedSpec, specErr = compileSpec() })
	require.NoError(t, specErr)
	return loadedSpec
}

func compileSpec() (*olSpec, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()

	core, err := loadJSONFile(filepath.Join(specDir, "OpenLineage.json"))
	if err != nil {
		return nil, err
	}
	if err := c.AddResource(coreSchemaURL, core); err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(specDir, "facets", "*.json"))
	if err != nil {
		return nil, err
	}
	type facetRef struct {
		place facetPlace
		key   string
		loc   string
	}
	var refs []facetRef
	for _, f := range files {
		doc, err := loadJSONFile(f)
		if err != nil {
			return nil, err
		}
		obj, _ := doc.(map[string]any)
		id, _ := obj["$id"].(string)
		if id == "" {
			return nil, fmt.Errorf("%s has no $id", f)
		}
		if err := c.AddResource(id, doc); err != nil {
			return nil, fmt.Errorf("adding %s: %w", f, err)
		}
		defs, _ := obj["$defs"].(map[string]any)
		props, _ := obj["properties"].(map[string]any)
		for key, p := range props {
			ref, _ := p.(map[string]any)["$ref"].(string)
			name := strings.TrimPrefix(ref, "#/$defs/")
			def, _ := defs[name].(map[string]any)
			if place := basePlace(def); place != "" {
				refs = append(refs, facetRef{place: place, key: key, loc: id + "#/$defs/" + name})
				continue
			}
			// A facet such as subset has one definition per place it can sit, so register each of them under the same key.
			found := false
			for defName, d := range defs {
				if place := basePlace(d.(map[string]any)); place != "" {
					refs = append(refs, facetRef{place: place, key: key, loc: id + "#/$defs/" + defName})
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("%s: cannot tell where facet %q sits", f, key)
			}
		}
	}

	spec := &olSpec{facets: map[facetPlace]map[string]*jsonschema.Schema{}}
	if spec.core, err = c.Compile(coreSchemaURL); err != nil {
		return nil, fmt.Errorf("compiling the core schema: %w", err)
	}
	for _, r := range refs {
		sch, err := c.Compile(r.loc)
		if err != nil {
			return nil, fmt.Errorf("compiling %s: %w", r.loc, err)
		}
		if spec.facets[r.place] == nil {
			spec.facets[r.place] = map[string]*jsonschema.Schema{}
		}
		spec.facets[r.place][r.key] = sch
	}
	return spec, nil
}

// basePlace reads which core facet type a facet definition extends through its allOf.
func basePlace(def map[string]any) facetPlace {
	all, _ := def["allOf"].([]any)
	for _, a := range all {
		ref, _ := a.(map[string]any)["$ref"].(string)
		if i := strings.LastIndex(ref, "/"); i >= 0 && strings.HasPrefix(ref, coreSchemaURL) {
			return facetPlace(ref[i+1:])
		}
	}
	return ""
}

func loadJSONFile(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// validateEvent checks one event against the core schema and each standard facet it carries against that facet's schema. Facets the spec does not define only have to satisfy the base facet shape, which the core schema checks.
func (s *olSpec) validateEvent(raw []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	if err := s.core.Validate(inst); err != nil {
		return fmt.Errorf("core schema: %w", err)
	}
	ev, _ := inst.(map[string]any)
	var problems []string
	check := func(where string, place facetPlace, facets any) {
		m, _ := facets.(map[string]any)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sch := s.facets[place][k]
			if sch == nil {
				continue
			}
			if err := sch.Validate(m[k]); err != nil {
				problems = append(problems, fmt.Sprintf("%s facet %q: %v", where, k, err))
			}
		}
	}
	datasets := func(where string, list any, extra facetPlace, extraKey string) {
		items, _ := list.([]any)
		for i, d := range items {
			dm, _ := d.(map[string]any)
			label := fmt.Sprintf("%s[%d]", where, i)
			check(label, placeDataset, dm["facets"])
			if extraKey != "" {
				check(label, extra, dm[extraKey])
			}
		}
	}
	if run, ok := ev["run"].(map[string]any); ok {
		check("run", placeRun, run["facets"])
	}
	if job, ok := ev["job"].(map[string]any); ok {
		check("job", placeJob, job["facets"])
	}
	if ds, ok := ev["dataset"].(map[string]any); ok {
		check("dataset", placeDataset, ds["facets"])
	}
	datasets("inputs", ev["inputs"], placeInput, "inputFacets")
	datasets("outputs", ev["outputs"], placeOutput, "outputFacets")
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

// requireSpecValid fails the test when an event is not valid OpenLineage.
func requireSpecValid(t testing.TB, raw []byte) {
	t.Helper()
	require.NoError(t, openLineageSpec(t).validateEvent(raw), "event is not valid against OpenLineage %s:\n%s", coreSchemaURL, raw)
}

// mustJSON marshals a fixture built in Go.
func mustJSON(t testing.TB, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

func TestSpecCompilesWithEveryStandardFacet(t *testing.T) {
	spec := openLineageSpec(t)
	for _, key := range []string{"schema", "columnLineage", "symlinks", "dataSource", "documentation", "ownership", "lifecycleStateChange", "version", "storage", "datasetType", "tags", "catalog", "hierarchy", "dataQualityMetrics"} {
		require.Contains(t, spec.facets[placeDataset], key)
	}
	for _, key := range []string{"inputStatistics", "dataQualityMetrics", "dataQualityAssertions"} {
		require.Contains(t, spec.facets[placeInput], key)
	}
	require.Contains(t, spec.facets[placeOutput], "outputStatistics")
	for _, key := range []string{"sql", "jobType", "sourceCode", "sourceCodeLocation", "documentation", "ownership", "tags"} {
		require.Contains(t, spec.facets[placeJob], key)
	}
	for _, key := range []string{"parent", "nominalTime", "errorMessage", "processing_engine", "externalQuery", "tags", "environmentVariables"} {
		require.Contains(t, spec.facets[placeRun], key)
	}
}
