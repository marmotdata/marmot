package query

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuilder(t *testing.T) {
	builder := NewBuilder()
	assert.NotNil(t, builder)
}

func TestBuildSQL(t *testing.T) {
	builder := NewBuilder()
	baseQuery := "SELECT id, metadata FROM documents"

	tests := []struct {
		name           string
		query          *Query
		baseQuery      string
		expectedSQL    string
		expectedParams []interface{}
	}{
		{
			name:           "Empty Query",
			query:          &Query{},
			baseQuery:      baseQuery,
			expectedSQL:    "SELECT id, metadata FROM documents) SELECT * FROM search_results ORDER BY search_rank DESC",
			expectedParams: []interface{}{""},
		},
		{
			name: "Bool Query",
			query: &Query{
				Bool: &BooleanQuery{
					Must: []Filter{
						{
							Field:    []string{"field1"},
							Operator: OpEquals,
							Value:    "value1",
						},
					},
				},
			},
			baseQuery:      baseQuery,
			expectedSQL:    "SELECT id, metadata FROM documents WHERE metadata->>'field1' = $2) SELECT * FROM search_results ORDER BY search_rank DESC",
			expectedParams: []interface{}{"", "value1"},
		},
		{
			name: "FreeText Query",
			query: &Query{
				FreeText: "search term",
			},
			baseQuery:      baseQuery,
			expectedSQL:    "SELECT id, metadata FROM documents WHERE (search_text @@ websearch_to_tsquery('english', $2) OR similarity(name, $2) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC",
			expectedParams: []interface{}{"", "search term"},
		},
		{
			name: "Combined Query",
			query: &Query{
				Bool: &BooleanQuery{
					Must: []Filter{
						{
							Field:    []string{"field1"},
							Operator: OpEquals,
							Value:    "value1",
						},
					},
				},
				FreeText: "search term",
			},
			baseQuery:      baseQuery,
			expectedSQL:    "SELECT id, metadata FROM documents WHERE metadata->>'field1' = $2 AND (search_text @@ websearch_to_tsquery('english', $3) OR similarity(name, $3) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC",
			expectedParams: []interface{}{"", "value1", "search term"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, err := builder.BuildSQL(tt.query, tt.baseQuery)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedParams, params)
		})
	}
}

func TestBuildConditions(t *testing.T) {
	b := NewBuilder()

	tests := []struct {
		name             string
		bq               *BooleanQuery
		expectedConds    []string
		expectedParams   []interface{}
		expectedStartIdx int
	}{
		{
			name: "Must, Should, MustNot",
			bq: &BooleanQuery{
				Must: []Filter{
					{
						Field:    []string{"field1"},
						Operator: OpEquals,
						Value:    "value1",
					},
				},
				Should: []Filter{
					{
						Field:    []string{"field2"},
						Operator: OpEquals,
						Value:    "value2",
					},
				},
				MustNot: []Filter{
					{
						Field:    []string{"field3"},
						Operator: OpEquals,
						Value:    "value3",
					},
				},
			},
			expectedConds:    []string{"(metadata->>'field1' = $1) OR metadata->>'field2' = $2 OR NOT (metadata->>'field3' = $3)"},
			expectedParams:   []interface{}{"value1", "value2", "value3"},
			expectedStartIdx: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions, params, err := b.BuildConditions(tt.bq)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedConds, conditions)
			assert.Equal(t, tt.expectedParams, params)
		})
	}
}

func TestBuildBooleanConditions(t *testing.T) {
	b := NewBuilder()

	tests := []struct {
		name             string
		bq               *BooleanQuery
		expectedConds    []string
		expectedParams   []interface{}
		expectedStartIdx int
	}{
		{
			name: "Must",
			bq: &BooleanQuery{
				Must: []Filter{
					{
						Field:    []string{"field1"},
						Operator: OpEquals,
						Value:    "value1",
					},
					{
						Field:    []string{"field2"},
						Operator: OpEquals,
						Value:    "value2",
					},
				},
			},
			expectedConds:    []string{"metadata->>'field1' = $1", "metadata->>'field2' = $2"},
			expectedParams:   []interface{}{"value1", "value2"},
			expectedStartIdx: 1,
		},
		{
			name: "Should",
			bq: &BooleanQuery{
				Should: []Filter{
					{
						Field:    []string{"field1"},
						Operator: OpEquals,
						Value:    "value1",
					},
					{
						Field:    []string{"field2"},
						Operator: OpEquals,
						Value:    "value2",
					},
				},
			},
			expectedConds:    []string{"metadata->>'field1' = $1 OR metadata->>'field2' = $2"},
			expectedParams:   []interface{}{"value1", "value2"},
			expectedStartIdx: 1,
		},
		{
			name: "MustNot",
			bq: &BooleanQuery{
				MustNot: []Filter{
					{
						Field:    []string{"field1"},
						Operator: OpEquals,
						Value:    "value1",
					},
					{
						Field:    []string{"field2"},
						Operator: OpEquals,
						Value:    "value2",
					},
				},
			},
			expectedConds:    []string{"NOT (metadata->>'field1' = $1) OR NOT (metadata->>'field2' = $2)"},
			expectedParams:   []interface{}{"value1", "value2"},
			expectedStartIdx: 1,
		},
		{
			name: "Nested",
			bq: &BooleanQuery{
				Must: []Filter{
					{
						Field:    []string{"nested"},
						Operator: OpEquals,
						Value: &BooleanQuery{
							Must: []Filter{
								{
									Field:    []string{"field1"},
									Operator: OpEquals,
									Value:    "value1",
								},
							},
						},
					},
				},
			},
			expectedConds:    []string{"(metadata->>'field1' = $1)"},
			expectedParams:   []interface{}{"value1"},
			expectedStartIdx: 1,
		},
		{
			name: "Combined",
			bq: &BooleanQuery{
				Must: []Filter{
					{
						Field:    []string{"field1"},
						Operator: OpEquals,
						Value:    "value1",
					},
				},
				Should: []Filter{
					{
						Field:    []string{"field2"},
						Operator: OpEquals,
						Value:    "value2",
					},
				},
				MustNot: []Filter{
					{
						Field:    []string{"field3"},
						Operator: OpEquals,
						Value:    "value3",
					},
				},
			},
			expectedConds:    []string{"(metadata->>'field1' = $1) OR metadata->>'field2' = $2 OR NOT (metadata->>'field3' = $3)"},
			expectedParams:   []interface{}{"value1", "value2", "value3"},
			expectedStartIdx: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions, params, newIndex, err := b.buildBooleanConditions(tt.bq, tt.expectedStartIdx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedConds, conditions)
			assert.Equal(t, tt.expectedParams, params)
			assert.Equal(t, tt.expectedStartIdx+len(tt.expectedParams), newIndex)
		})
	}
}

func TestBuildFilterCondition(t *testing.T) {
	b := NewBuilder()

	tests := []struct {
		name             string
		filter           Filter
		expectedCond     string
		expectedParams   []interface{}
		expectedStartIdx int
		expectedErr      error
	}{
		{
			name: "Equals",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpEquals,
				Value:    "value1",
			},
			expectedCond:     "metadata->>'field1' = $1",
			expectedParams:   []interface{}{"value1"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "NotEquals",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpNotEquals,
				Value:    "value1",
			},
			expectedCond:     "metadata->>'field1' != $1",
			expectedParams:   []interface{}{"value1"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Contains",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpContains,
				Value:    "value1",
			},
			expectedCond:     "metadata->>'field1' ILIKE $1",
			expectedParams:   []interface{}{"%value1%"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Greater",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpGreater,
				Value:    10,
			},
			expectedCond:     "(metadata->>'field1')::numeric > $1",
			expectedParams:   []interface{}{"10"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Less",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpLess,
				Value:    10,
			},
			expectedCond:     "(metadata->>'field1')::numeric < $1",
			expectedParams:   []interface{}{"10"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "GreaterEqual",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpGreaterEqual,
				Value:    10,
			},
			expectedCond:     "(metadata->>'field1')::numeric >= $1",
			expectedParams:   []interface{}{"10"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "LessEqual",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpLessEqual,
				Value:    10,
			},
			expectedCond:     "(metadata->>'field1')::numeric <= $1",
			expectedParams:   []interface{}{"10"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Wildcard",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: OpWildcard,
				Value:    "val*ue*",
			},
			expectedCond:     "metadata->>'field1' LIKE $1",
			expectedParams:   []interface{}{"val%ue%"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Unsupported Operator",
			filter: Filter{
				Field:    []string{"field1"},
				Operator: "invalid",
				Value:    "value1",
			},
			expectedCond:     "",
			expectedParams:   nil,
			expectedStartIdx: 1,
			expectedErr:      fmt.Errorf("unsupported operator: invalid"),
		},
		{
			name: "Nested Field",
			filter: Filter{
				Field:    []string{"parent", "child", "field1"},
				Operator: OpEquals,
				Value:    "value1",
			},
			expectedCond:     "metadata->'parent'->'child'->>'field1' = $1",
			expectedParams:   []interface{}{"value1"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "FreeText",
			filter: Filter{
				Field:    []string{"freetext"},
				Operator: OpEquals,
				Value:    "searchTerm",
			},
			expectedCond:     "(search_text @@ websearch_to_tsquery('english', $1) OR similarity(name, $1) > 0.3)",
			expectedParams:   []interface{}{"searchTerm"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
		{
			name: "Nested Boolean",
			filter: Filter{
				Field:    []string{"nested"},
				Operator: OpEquals,
				Value: &BooleanQuery{
					Must: []Filter{
						{
							Field:    []string{"field1"},
							Operator: OpEquals,
							Value:    "value1",
						},
					},
				},
			},
			expectedCond:     "(metadata->>'field1' = $1)",
			expectedParams:   []interface{}{"value1"},
			expectedStartIdx: 1,
			expectedErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition, params, newIndex, err := b.buildFilterCondition(tt.filter, tt.expectedStartIdx)
			assert.Equal(t, tt.expectedCond, condition)
			assert.Equal(t, tt.expectedParams, params)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStartIdx+len(tt.expectedParams), newIndex)
			}
		})
	}
}
