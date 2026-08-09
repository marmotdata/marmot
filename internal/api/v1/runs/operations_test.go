package runs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An asset's identity is its MRN. Until this field existed the ingest API
// could not carry one, so the server derived identity from the display
// name instead and two objects sharing a name became one asset. These
// tests pin the wire contract that carries it.

func TestCreateAssetRequest_CarriesTheMRN(t *testing.T) {
	body := `{"name":"orders","mrn":"mrn://table/postgresql/public.orders","type":"Table","providers":["PostgreSQL"]}`

	var req CreateAssetRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	assert.Equal(t, "mrn://table/postgresql/public.orders", req.MRN,
		"the qualified identity has to survive the wire")
	assert.Equal(t, "orders", req.Name, "while the name stays the object's own name")
}

func TestCreateAssetRequest_OmitsAnEmptyMRN(t *testing.T) {
	// A plugin that sets no MRN must not send an empty one, so that older
	// and newer servers both fall back to deriving it.
	encoded, err := json.Marshal(CreateAssetRequest{Name: "orders", Type: "Table"})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"mrn"`)
}

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

func TestOptionalString_TreatsEmptyAsAbsent(t *testing.T) {
	// The service derives an MRN when this is nil, so an empty string must
	// not reach it as a value.
	assert.Nil(t, optionalString(""))

	got := optionalString("mrn://table/postgresql/public.orders")
	require.NotNil(t, got)
	assert.Equal(t, "mrn://table/postgresql/public.orders", *got)
}
