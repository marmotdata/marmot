package runs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Glossary terms ride the same batch as the assets that reference them,
// keyed by name because a plugin cannot know the ids Marmot will assign.

func TestBatchCreateRequest_CarriesGlossaryTerms(t *testing.T) {
	body := `{"assets":[],"glossary_terms":[{"name":"BusinessTerms.Customer.LifetimeValue","definition":"Value over time.","parent":"BusinessTerms.Customer","synonyms":["CLV"]}]}`

	var req BatchCreateRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	require.Len(t, req.GlossaryTerms, 1)
	assert.Equal(t, "BusinessTerms.Customer.LifetimeValue", req.GlossaryTerms[0].Name)
	assert.Equal(t, "BusinessTerms.Customer", req.GlossaryTerms[0].Parent,
		"the hierarchy travels as names, not ids")
	assert.Equal(t, []string{"CLV"}, req.GlossaryTerms[0].Synonyms)
}

func TestBatchCreateRequest_OmitsAnEmptyGlossary(t *testing.T) {
	// A plugin that curates no terms must not send the field, so an
	// older server sees exactly the request it saw before.
	encoded, err := json.Marshal(BatchCreateRequest{PipelineName: "demo", SourceName: "postgres"})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"glossary_terms"`)
}

func TestCreateAssetRequest_CarriesItsTermNames(t *testing.T) {
	body := `{"name":"customers","type":"Table","terms":["BusinessTerms.Customer"]}`

	var req CreateAssetRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	assert.Equal(t, []string{"BusinessTerms.Customer"}, req.Terms)
}

func TestCreateAssetRequest_OmitsAbsentTerms(t *testing.T) {
	encoded, err := json.Marshal(CreateAssetRequest{Name: "customers", Type: "Table"})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"terms"`)
}

// A server is routinely newer than the plugins reporting to it, because
// plugins ship on their own cadence. The batch endpoint therefore
// has to read both shapes of the sources field.

func TestAssetSources_ReadsTheObjectFormCurrentClientsSend(t *testing.T) {
	var got AssetSources
	require.NoError(t, json.Unmarshal(
		[]byte(`[{"name":"PostgreSQL","priority":1,"properties":{"schema":"public"}}]`), &got))

	require.Len(t, got, 1)
	assert.Equal(t, "PostgreSQL", got[0].Name)
	assert.Equal(t, 1, got[0].Priority)
	assert.Equal(t, "public", got[0].Properties["schema"])
}

func TestAssetSources_ReadsTheBareNamesOlderClientsSend(t *testing.T) {
	// Rejecting this shape would return 400 to every batch from a plugin
	// that had not been upgraded alongside the server, which stops
	// ingestion entirely.
	var got AssetSources
	require.NoError(t, json.Unmarshal([]byte(`["PostgreSQL","dbt"]`), &got))

	require.Len(t, got, 2)
	assert.Equal(t, "PostgreSQL", got[0].Name)
	assert.Equal(t, "dbt", got[1].Name)
	assert.Zero(t, got[0].Priority, "a name carries no other detail")
}

func TestAssetSources_RejectsAShapeItCannotRead(t *testing.T) {
	var got AssetSources
	err := json.Unmarshal([]byte(`{"name":"PostgreSQL"}`), &got)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "array of names or of source objects")
}

func TestCreateAssetRequest_AcceptsABatchFromAnOlderClient(t *testing.T) {
	// The whole request, as a pre-upgrade plugin would send it.
	body := `{"name":"orders","type":"Table","providers":["PostgreSQL"],"sources":["PostgreSQL"]}`

	var req CreateAssetRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	require.Len(t, req.Sources, 1)
	assert.Equal(t, "PostgreSQL", req.Sources[0].Name)
}
