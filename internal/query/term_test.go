package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTermFilterOnAssets(t *testing.T) {
	q, err := NewParser().Parse(`@term: "Churn"`)
	require.NoError(t, err)
	require.True(t, q.HasStructuredFilters())

	conditions, params, _, err := NewBuilder().BuildSearchConditions(q, 0)
	require.NoError(t, err)
	require.Len(t, conditions, 1)
	assert.Contains(t, conditions[0], "at.asset_id::text = assets.id::text")
	assert.Contains(t, conditions[0], "lower(gt.name) = lower($1)")
	assert.Contains(t, conditions[0], "gt.deleted_at IS NULL")
	assert.Equal(t, []interface{}{"Churn"}, params)
}

func TestTermFilterOnSearchIndexWithWildcard(t *testing.T) {
	q, err := NewParser().Parse(`@term: "Rev*"`)
	require.NoError(t, err)

	conditions, params, _, err := NewSearchIndexBuilder().BuildSearchConditions(q, 2)
	require.NoError(t, err)
	require.Len(t, conditions, 1)
	assert.Contains(t, conditions[0], "search_index.entity_id::text")
	assert.Contains(t, conditions[0], "gt.name ILIKE $3")
	assert.Equal(t, []interface{}{"Rev%"}, params)
}

func TestTermFilterRejectsRanges(t *testing.T) {
	q, err := NewParser().Parse(`@term > "a"`)
	if err != nil {
		return
	}
	_, _, _, err = NewBuilder().BuildSearchConditions(q, 0)
	assert.Error(t, err)
}
