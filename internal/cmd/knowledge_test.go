package cmd

import (
	"testing"

	marmot "github.com/marmotdata/marmot/sdk/go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKinds() []*marmot.KnowledgeKind {
	return []*marmot.KnowledgeKind{
		{ID: "runbook", Type: "Runbook", Label: "Runbook", Fields: []*marmot.KnowledgeField{
			{Key: "triggers", Type: "list"},
			{Key: "escalation_contact", Type: "text"},
		}},
		{ID: "brand-voice", Type: "Brand Voice", Label: "Brand voice", Fields: []*marmot.KnowledgeField{
			{Key: "traits", Type: "table"},
		}},
	}
}

func TestFindKindTakesIDTypeLabelOrPlural(t *testing.T) {
	kinds := testKinds()
	for _, ref := range []string{"runbook", "Runbook", "runbooks", " RUNBOOK "} {
		require.NotNil(t, findKind(kinds, ref), ref)
		assert.Equal(t, "runbook", findKind(kinds, ref).ID, ref)
	}
	for _, ref := range []string{"brand-voice", "Brand Voice", "brand_voice", "brand voice"} {
		require.NotNil(t, findKind(kinds, ref), ref)
		assert.Equal(t, "brand-voice", findKind(kinds, ref).ID, ref)
	}
	assert.Nil(t, findKind(kinds, "memo"))
}

func TestFieldValueFollowsTheFieldType(t *testing.T) {
	runbook, voice := testKinds()[0], testKinds()[1]

	v, err := fieldValue(runbook, "triggers", "disk full, , WAL stuck ")
	require.NoError(t, err)
	assert.Equal(t, []string{"disk full", "WAL stuck"}, v, "a list is comma separated, blanks dropped")

	v, err = fieldValue(runbook, "escalation_contact", "#data-platform, on-call")
	require.NoError(t, err)
	assert.Equal(t, "#data-platform, on-call", v, "text keeps its commas")

	v, err = fieldValue(voice, "traits", `[{"trait":"Direct","not":"Blunt"}]`)
	require.NoError(t, err)
	assert.Equal(t, []map[string]any{{"trait": "Direct", "not": "Blunt"}}, v)

	_, err = fieldValue(voice, "traits", "Direct")
	assert.ErrorContains(t, err, "JSON array of rows")

	_, err = fieldValue(runbook, "nope", "x")
	assert.ErrorContains(t, err, "its fields are triggers, escalation_contact")
}

func TestFieldTextShowsListsAndTables(t *testing.T) {
	assert.Equal(t, "a, b", fieldText([]any{"a", "b"}))
	assert.Equal(t, "2 rows (see -o json)", fieldText([]any{map[string]any{}, map[string]any{}}))
	assert.Equal(t, "2026-01-01", fieldText("2026-01-01"))
	assert.Equal(t, "", fieldText(nil))
}

func TestReadOnlyBlocksKnowledgeWrites(t *testing.T) {
	viper.Set("read_only", true)
	t.Cleanup(func() { viper.Set("read_only", false) })

	for _, c := range []*struct {
		name  string
		write bool
	}{
		{"get", false}, {"list", false}, {"kinds", false}, {"about", false},
		{"create", true}, {"update", true}, {"reviewed", true}, {"delete", true},
	} {
		sub, _, err := knowledgeCmd.Find([]string{c.name})
		require.NoError(t, err, c.name)
		err = enforceReadOnly(sub, nil)
		if c.write {
			assert.Equal(t, ExitReadOnly, ExitCode(err), c.name)
		} else {
			assert.NoError(t, err, c.name)
		}
	}
}
