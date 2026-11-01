package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagFilter(t *testing.T) {
	q, err := NewParser().Parse(`orders AND @tag = "pii"`)
	require.NoError(t, err)
	require.True(t, q.HasStructuredFilters())
	assert.Equal(t, "orders", q.FreeText)

	conditions, params, next, err := NewSearchIndexBuilder().BuildSearchConditions(q, 1)
	require.NoError(t, err)
	require.Len(t, conditions, 1)
	assert.Equal(t, "EXISTS (SELECT 1 FROM unnest(tags) AS tag WHERE lower(tag) = lower($2))", conditions[0])
	assert.Equal(t, []interface{}{"pii"}, params)
	assert.Equal(t, 2, next)
}

func TestTagFilterWildcardAndNegation(t *testing.T) {
	q, err := NewParser().Parse(`@tag = "fin*"`)
	require.NoError(t, err)
	conditions, params, _, err := NewBuilder().BuildSearchConditions(q, 0)
	require.NoError(t, err)
	assert.Contains(t, conditions[0], "tag ILIKE $1")
	assert.Equal(t, []interface{}{"fin%"}, params)

	q, err = NewParser().Parse(`NOT @tag = "deprecated"`)
	require.NoError(t, err)
	conditions, _, _, err = NewBuilder().BuildSearchConditions(q, 0)
	require.NoError(t, err)
	assert.Contains(t, conditions[0], "NOT (EXISTS (SELECT 1 FROM unnest(tags)")
}

func TestTagFilterRejectsComparisons(t *testing.T) {
	q, err := NewParser().Parse(`@tag > "a"`)
	require.NoError(t, err)
	_, _, _, err = NewBuilder().BuildSearchConditions(q, 0)
	assert.Error(t, err)
}
