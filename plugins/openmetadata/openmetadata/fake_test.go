package openmetadata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// fakeOM is a stand-in for an OpenMetadata server. Tests register the
// entities each list endpoint should return and then run a real
// discovery against it.
type fakeOM struct {
	entities map[string][]map[string]interface{}
	lineage  map[string]lineageResponse
	runs     map[string][]map[string]interface{}
	missing  map[string]bool
}

func newFakeOM() *fakeOM {
	return &fakeOM{
		entities: make(map[string][]map[string]interface{}),
		lineage:  make(map[string]lineageResponse),
		runs:     make(map[string][]map[string]interface{}),
		missing:  make(map[string]bool),
	}
}

// without makes an endpoint answer 404, the way a server too old to know
// the entity kind does.
func (f *fakeOM) without(paths ...string) *fakeOM {
	for _, path := range paths {
		f.missing[path] = true
	}
	return f
}

// with registers the entities returned by one list endpoint, for
// example "tables" or "dashboard/datamodels".
func (f *fakeOM) with(path string, entities ...map[string]interface{}) *fakeOM {
	f.entities[path] = append(f.entities[path], entities...)
	return f
}

// withLineage registers the response for one entity's lineage.
func (f *fakeOM) withLineage(entityType, id string, resp lineageResponse) *fakeOM {
	f.lineage[entityType+"/"+id] = resp
	return f
}

// withRuns registers the executions of one pipeline, by fully qualified name.
func (f *fakeOM) withRuns(fqn string, runs ...map[string]interface{}) *fakeOM {
	f.runs[fqn] = append(f.runs[fqn], runs...)
	return f
}

func (f *fakeOM) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		w.Header().Set("Content-Type", "application/json")

		if strings.HasPrefix(path, "lineage/") {
			resp, ok := f.lineage[strings.TrimPrefix(path, "lineage/")]
			if !ok {
				resp = lineageResponse{}
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if strings.HasPrefix(path, "pipelines/") && strings.HasSuffix(path, "/status") {
			fqn := strings.TrimSuffix(strings.TrimPrefix(path, "pipelines/"), "/status")
			writeList(w, f.runs[fqn])
			return
		}

		if f.missing[path] {
			http.Error(w, `{"code":404,"message":"not found"}`, http.StatusNotFound)
			return
		}
		writeList(w, f.entities[path])
	}))

	t.Cleanup(server.Close)
	return server
}

func writeList(w http.ResponseWriter, entities []map[string]interface{}) {
	if entities == nil {
		entities = []map[string]interface{}{}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   entities,
		"paging": map[string]interface{}{"total": len(entities)},
	})
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeOM, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL, "jwt_token": "test-token"}
	for key, value := range overrides {
		config[key] = value
	}

	source := &Source{}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

// findAsset looks an asset up by the qualified name that identifies it,
// not by the name shown in the UI. Those are deliberately different: the
// MRN carries the full path, while Name is the object's own name.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type != assetType || asset.MRN == nil || len(asset.Providers) == 0 {
			continue
		}
		if *asset.MRN == mrn.New(assetType, asset.Providers[0], name) {
			return &result.Assets[i]
		}
	}
	return nil
}

// findByDisplayName looks an asset up by what the UI shows.
func findByDisplayName(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

// findTerm returns the imported glossary term with the given name.
func findTerm(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.GlossaryTerm {
	for i, term := range result.GlossaryTerms {
		if term.Name == name {
			return &result.GlossaryTerms[i]
		}
	}
	return nil
}

// hasEdge reports whether the result contains an edge between two MRNs.
func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

// tableEntity builds a table entity for a fake server.
func tableEntity(service, serviceType, fqn, tableType string) map[string]interface{} {
	return map[string]interface{}{
		"id":                 "id-" + fqn,
		"name":               fqn[strings.LastIndex(fqn, ".")+1:],
		"fullyQualifiedName": fqn,
		"tableType":          tableType,
		"serviceType":        serviceType,
		"service":            map[string]interface{}{"name": service, "type": "databaseService"},
	}
}

// glossaryEntity builds a glossary for a fake server. A glossary has no
// service: it belongs to the catalog rather than to a system, and its
// fully qualified name is its own name.
func glossaryEntity(name string) map[string]interface{} {
	return map[string]interface{}{
		"id":                 "id-" + name,
		"name":               name,
		"fullyQualifiedName": name,
		"href":               "https://om.example.com/api/v1/glossaries/id-" + name,
	}
}

// termEntity builds a glossary term for a fake server, named by the
// fully qualified name OpenMetadata gives it: the glossary, then every
// term above it, then its own name.
func termEntity(glossaryName, fqn string) map[string]interface{} {
	return map[string]interface{}{
		"id":                 "id-" + fqn,
		"name":               fqn[strings.LastIndex(fqn, ".")+1:],
		"fullyQualifiedName": fqn,
		"glossary":           map[string]interface{}{"name": glossaryName, "fullyQualifiedName": glossaryName},
		"href":               "https://om.example.com/api/v1/glossaryTerms/id-" + fqn,
	}
}

// entity builds a generic entity for a fake server.
func entity(service, serviceType, fqn string) map[string]interface{} {
	return map[string]interface{}{
		"id":                 "id-" + fqn,
		"name":               fqn[strings.LastIndex(fqn, ".")+1:],
		"fullyQualifiedName": fqn,
		"serviceType":        serviceType,
		"service":            map[string]interface{}{"name": service, "type": "service"},
	}
}
