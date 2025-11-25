package query

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryParsing(t *testing.T) {
	parser := NewParser()
	builder := NewBuilder()

	tests := []struct {
		name           string
		searchQuery    string
		expectedSQL    string
		expectedParams []interface{}
	}{
		{
			name:           "Exact metadata match",
			searchQuery:    `@metadata.name: "{prod}_command_CreateOrder"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' = $2) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "{prod}_command_CreateOrder"},
		},
		{
			name:           "Contains operator",
			searchQuery:    `@metadata.name contains "CreateOrder"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' ILIKE $2) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "%CreateOrder%"},
		},
		{
			name:           "Numeric comparison",
			searchQuery:    `@metadata.partitions > 5`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'partitions')::numeric > $2) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "5"},
		},
		{
			name:           "Combined free text and metadata",
			searchQuery:    `order @metadata.team: "logistics"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'team' = $2 AND (search_text @@ websearch_to_tsquery('english', $3) OR similarity(name, $3) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "logistics", "order"},
		},
		{
			name:           "Complex AND condition",
			searchQuery:    `@metadata.partitions > 5 AND @metadata.team: "orders"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'partitions')::numeric > $2 AND metadata->>'team' = $3) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "5", "orders"},
		},
		{
			name:           "Complex_OR_condition",
			searchQuery:    `@metadata.partitions < 3 OR @metadata.team: "orders"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE ((metadata->>'partitions')::numeric < $2) OR metadata->>'team' = $3) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "3", "orders"},
		},
		{
			name:           "Range query",
			searchQuery:    `@metadata.partitions range [1 TO 10]`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'partitions')::numeric >= $2 AND (metadata->>'partitions')::numeric <= $3) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", float64(1), float64(10)},
		},
		{
			name:           "NOT condition",
			searchQuery:    `NOT @metadata.team: "orders"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE NOT (metadata->>'team' = $2)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "orders"},
		},
		{
			name:           "Complex query with multiple conditions",
			searchQuery:    `@metadata.partitions > 5 AND @metadata.team: "orders" order service`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'partitions')::numeric > $2 AND metadata->>'team' = $3 AND (search_text @@ websearch_to_tsquery('english', $4) OR similarity(name, $4) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "5", "orders", "order service"},
		},
		{
			name:           "Free text before metadata filter",
			searchQuery:    `testing service @metadata.team: "orders"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'team' = $2 AND (search_text @@ websearch_to_tsquery('english', $3) OR similarity(name, $3) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "orders", "testing service"},
		},
		{
			name:           "Free text after metadata filter",
			searchQuery:    `@metadata.team: "orders" testing service`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'team' = $2 AND (search_text @@ websearch_to_tsquery('english', $3) OR similarity(name, $3) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "orders", "testing service"},
		},
		{
			name:           "Free text between metadata filters",
			searchQuery:    `@metadata.team: "orders" critical service @metadata.status: "active"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'team' = $2 AND metadata->>'status' = $3 AND (search_text @@ websearch_to_tsquery('english', $4) OR similarity(name, $4) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "orders", "active", "critical service"},
		},
		// Wildcard queries
		{
			name:           "Simple wildcard in metadata",
			searchQuery:    `@metadata.name: "order*"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' ILIKE $2) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "order%"},
		},
		{
			name:           "Multiple wildcards in metadata",
			searchQuery:    `@metadata.name: "ord*_serv*"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' ILIKE $2) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "ord%_serv%"},
		},
		{
			name:           "Wildcard with free text",
			searchQuery:    `critical @metadata.name: "ord*" service`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' ILIKE $2 AND (search_text @@ websearch_to_tsquery('english', $3) OR similarity(name, $3) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "ord%", "service"},
		},
		{
			name:           "Multiple_wildcards_with_boolean_operators",
			searchQuery:    `@metadata.name: "ord*" AND @metadata.environment: "*prod*" OR @metadata.team: "dev*"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'name' ILIKE $2 AND metadata->>'environment' ILIKE $3) OR metadata->>'team' ILIKE $4) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "ord%", "%prod%", "dev%"},
		},
		{
			name:           "Complex query with wildcards and free text",
			searchQuery:    `critical @metadata.name: "ord*_service" AND @metadata.team: "dev*" production`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE metadata->>'name' ILIKE $2 AND metadata->>'team' ILIKE $3 AND (search_text @@ websearch_to_tsquery('english', $4) OR similarity(name, $4) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "ord%_service", "dev%", "production"},
		},
		{
			name:           "NOT_operator_with_wildcards_and_free_text",
			searchQuery:    `production NOT @metadata.environment: "*test*" @metadata.name: "api*"`,
			expectedSQL:    `WITH search_results AS (SELECT *, ts_rank_cd(search_text, websearch_to_tsquery('english', $1), 32) as search_rank, similarity(name, $1) as name_similarity FROM assets WHERE (metadata->>'name' ILIKE $2) OR NOT (metadata->>'environment' ILIKE $3) AND (search_text @@ websearch_to_tsquery('english', $4) OR similarity(name, $4) > 0.3)) SELECT * FROM search_results ORDER BY search_rank DESC`,
			expectedParams: []interface{}{"", "api%", "%test%", "production"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedQuery, err := parser.Parse(tt.searchQuery)
			require.NoError(t, err)

			baseQuery := `WITH search_results AS (
                        SELECT 
                            *,
                            ts_rank_cd(
                                search_text,
                                websearch_to_tsquery('english', $1),
                                32
                            ) as search_rank,
                            similarity(name, $1) as name_similarity
                        FROM assets`

			sql, params, err := builder.BuildSQL(parsedQuery, baseQuery)
			require.NoError(t, err)

			// Compare normalized SQL
			normalizedActual := normalizeSQL(sql)
			normalizedExpected := normalizeSQL(tt.expectedSQL)

			assert.Equal(t, normalizedExpected, normalizedActual, "SQL queries should match")

			// Compare params
			assert.Equal(t, tt.expectedParams, params, "Parameters should match")
		})
	}
}

// Additional test for parsing edge cases
func TestQueryParsingEdgeCases(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		searchQuery string
		expectError bool
		validate    func(*testing.T, *Query)
	}{
		{
			name:        "Empty query",
			searchQuery: "",
			expectError: false,
			validate: func(t *testing.T, q *Query) {
				assert.Empty(t, q.FreeText)
				assert.Empty(t, q.Filters)
			},
		},
		{
			name:        "Invalid metadata field",
			searchQuery: "@metadata.",
			expectError: true,
		},
		{
			name:        "Invalid operator",
			searchQuery: "@metadata.field invalidop value",
			expectError: true,
		},
		{
			name:        "Unclosed quotes",
			searchQuery: `@metadata.name: "unclosed`,
			expectError: true,
		},
		{
			name:        "Mixed quotes",
			searchQuery: `@metadata.name: "mixed' quotes"`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.searchQuery)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestWildcardParsing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		searchQuery string
		expectError bool
		validate    func(*testing.T, *Query)
	}{
		{
			name:        "Single wildcard at end",
			searchQuery: `@metadata.name: "test*"`,
			validate: func(t *testing.T, q *Query) {
				assert.Equal(t, "test*", q.Bool.Must[0].Value)
				assert.Equal(t, OpWildcard, q.Bool.Must[0].Operator)
			},
		},
		{
			name:        "Single wildcard at start",
			searchQuery: `@metadata.name: "*test"`,
			validate: func(t *testing.T, q *Query) {
				assert.Equal(t, "*test", q.Bool.Must[0].Value)
				assert.Equal(t, OpWildcard, q.Bool.Must[0].Operator)
			},
		},
		{
			name:        "Multiple wildcards",
			searchQuery: `@metadata.name: "test*service*api"`,
			validate: func(t *testing.T, q *Query) {
				assert.Equal(t, "test*service*api", q.Bool.Must[0].Value)
				assert.Equal(t, OpWildcard, q.Bool.Must[0].Operator)
			},
		},
		{
			name:        "Wildcards with special characters",
			searchQuery: `@metadata.name: "*test_*-service"`,
			validate: func(t *testing.T, q *Query) {
				assert.Equal(t, "*test_*-service", q.Bool.Must[0].Value)
				assert.Equal(t, OpWildcard, q.Bool.Must[0].Operator)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.searchQuery)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestBooleanQueryParsing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		searchQuery string
		validate    func(*testing.T, *Query)
	}{
		{
			name:        "Simple AND query",
			searchQuery: `@metadata.team: "orders" AND @metadata.partitions > 5`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Must, 2)
			},
		},
		{
			name:        "Simple OR query",
			searchQuery: `@metadata.team: "orders" OR @metadata.team: "logistics"`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Should, 1)
				assert.Equal(t, []string{"team"}, q.Bool.Should[0].Field)
				assert.Equal(t, "logistics", q.Bool.Should[0].Value)
				assert.Equal(t, []string{"team"}, q.Bool.Must[0].Field)
				assert.Equal(t, "orders", q.Bool.Must[0].Value)
			},
		},
		{
			name:        "Complex nested query",
			searchQuery: `(@metadata.team: "orders" AND @metadata.partitions > 5) OR @metadata.type: "service"`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Must, 1)
				assert.Len(t, q.Bool.Should, 1)

				// Verify the nested query
				nestedBool, ok := q.Bool.Must[0].Value.(*BooleanQuery)
				require.True(t, ok)
				assert.Len(t, nestedBool.Must, 2)

				// Check the OR clause
				assert.Equal(t, []string{"type"}, q.Bool.Should[0].Field)
				assert.Equal(t, "service", q.Bool.Should[0].Value)
			},
		},
		{
			name:        "AND NOT query",
			searchQuery: `@metadata.team: "orders" AND NOT @metadata.environment: "prod"`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Must, 1)
				assert.Len(t, q.Bool.MustNot, 1)

				assert.Equal(t, []string{"team"}, q.Bool.Must[0].Field)
				assert.Equal(t, "orders", q.Bool.Must[0].Value)
				assert.Equal(t, []string{"environment"}, q.Bool.MustNot[0].Field)
				assert.Equal(t, "prod", q.Bool.MustNot[0].Value)
			},
		},
		{
			name:        "OR NOT query",
			searchQuery: `@metadata.team: "orders" OR NOT @metadata.environment: "prod"`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Must, 1)
				assert.Len(t, q.Bool.MustNot, 1)

				assert.Equal(t, []string{"team"}, q.Bool.Must[0].Field)
				assert.Equal(t, "orders", q.Bool.Must[0].Value)
				assert.Equal(t, []string{"environment"}, q.Bool.MustNot[0].Field)
				assert.Equal(t, "prod", q.Bool.MustNot[0].Value)
			},
		},
		{
			name:        "Complex NOT query",
			searchQuery: `(@metadata.team: "orders" AND NOT @metadata.environment: "prod") OR @metadata.type: "service"`,
			validate: func(t *testing.T, q *Query) {
				require.NotNil(t, q.Bool)
				assert.Len(t, q.Bool.Must, 1)
				assert.Len(t, q.Bool.Should, 1)

				nestedBool, ok := q.Bool.Must[0].Value.(*BooleanQuery)
				require.True(t, ok)
				assert.Len(t, nestedBool.Must, 1)
				assert.Len(t, nestedBool.MustNot, 1)

				assert.Equal(t, []string{"type"}, q.Bool.Should[0].Field)
				assert.Equal(t, "service", q.Bool.Should[0].Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse(tt.searchQuery)
			require.NoError(t, err)
			tt.validate(t, result)
		})
	}
}

// normalizeSQL normalizes a SQL string for more accurate comparison
func normalizeSQL(sql string) string {
	// Remove all newlines and extra whitespace
	sql = strings.Join(strings.Fields(sql), " ")
	// Normalize parentheses spacing
	sql = strings.ReplaceAll(sql, "( ", "(")
	sql = strings.ReplaceAll(sql, " )", ")")
	return sql
}
