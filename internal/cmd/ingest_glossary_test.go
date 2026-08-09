package cmd

import (
	"encoding/json"
	"testing"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLI is what carries a source's business terms to the server. They
// travel keyed by name, next to the assets that reference them.

func TestBatchCreateRequest_SendsGlossaryTerms(t *testing.T) {
	encoded, err := json.Marshal(BatchCreateRequest{
		PipelineName: "demo",
		GlossaryTerms: []CreateGlossaryTermRequest{{
			Name:       "BusinessTerms.Customer.LifetimeValue",
			Definition: "Value over time.",
			Parent:     "BusinessTerms.Customer",
		}},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))

	terms, ok := out["glossary_terms"].([]interface{})
	require.True(t, ok, "the batch has to carry the terms it discovered")
	require.Len(t, terms, 1)

	term := terms[0].(map[string]interface{})
	assert.Equal(t, "BusinessTerms.Customer.LifetimeValue", term["name"])
	assert.Equal(t, "BusinessTerms.Customer", term["parent"],
		"a plugin cannot know Marmot's ids, so the hierarchy travels as names")
}

func TestBatchCreateRequest_OmitsAnEmptyGlossary(t *testing.T) {
	// A source with no business terms must send the request it always
	// sent, so an older server keeps working.
	encoded, err := json.Marshal(BatchCreateRequest{
		PipelineName:  "demo",
		GlossaryTerms: []CreateGlossaryTermRequest{},
	})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"glossary_terms"`)
}

func TestCreateAssetRequest_SendsItsTermNames(t *testing.T) {
	encoded, err := json.Marshal(CreateAssetRequest{
		Name:  "customers",
		Type:  "Table",
		Terms: []string{"BusinessTerms.Customer"},
	})
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(encoded, &out))
	assert.Equal(t, []interface{}{"BusinessTerms.Customer"}, out["terms"])
}

func TestCreateAssetRequest_OmitsAbsentTerms(t *testing.T) {
	encoded, err := json.Marshal(CreateAssetRequest{Name: "customers", Type: "Table"})
	require.NoError(t, err)

	assert.NotContains(t, string(encoded), `"terms"`)
}

func TestProcessGlossaryResult_FoldsCountsIntoBothSummaries(t *testing.T) {
	runSummary := &plugin.RunSummary{}
	overall := &Summary{GlossaryTermsCreated: 1}

	processGlossaryResult(&GlossaryResult{
		TermsCreated: 3,
		TermsUpdated: 2,
		AssetsLinked: 4,
	}, runSummary, overall)

	assert.Equal(t, 3, runSummary.GlossaryTermsCreated)
	assert.Equal(t, 2, runSummary.GlossaryTermsUpdated)
	assert.Equal(t, 4, runSummary.AssetsTermsLinked)
	assert.Equal(t, 4, overall.GlossaryTermsCreated, "the overall count accumulates across sources")
	assert.Equal(t, 4, overall.AssetsTermsLinked)
}

func TestProcessGlossaryResult_ToleratesAServerThatSendsNothing(t *testing.T) {
	// A server that predates glossary sync answers without the field.
	runSummary := &plugin.RunSummary{}

	processGlossaryResult(nil, runSummary, &Summary{})

	assert.Equal(t, 0, runSummary.GlossaryTermsCreated)
}
