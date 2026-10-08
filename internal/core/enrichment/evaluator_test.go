package enrichment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenumberParameters(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "no parameters",
			sql:      "SELECT id FROM assets WHERE is_stub = FALSE",
			expected: "SELECT id FROM assets WHERE is_stub = FALSE",
		},
		{
			name:     "single parameter",
			sql:      "SELECT id FROM assets WHERE lower(type) = lower($2)",
			expected: "SELECT id FROM assets WHERE lower(type) = lower($1)",
		},
		{
			// Two filters used to collapse onto a single $1, leaving the query
			// with one placeholder and two arguments.
			name:     "two parameters keep their distance",
			sql:      "SELECT id FROM assets WHERE lower(type) = lower($2) AND providers @> $3",
			expected: "SELECT id FROM assets WHERE lower(type) = lower($1) AND providers @> $2",
		},
		{
			name:     "four parameters",
			sql:      "WHERE a = $2 AND b = $3 AND c = $4 AND d = $5",
			expected: "WHERE a = $1 AND b = $2 AND c = $3 AND d = $4",
		},
		{
			name:     "repeated parameter stays one parameter",
			sql:      "WHERE (search_text @@ to_tsquery($2) OR word_similarity($2, name) > 0.3)",
			expected: "WHERE (search_text @@ to_tsquery($1) OR word_similarity($1, name) > 0.3)",
		},
		{
			name:     "two-digit parameters",
			sql:      "WHERE a = $9 AND b = $10 AND c = $11",
			expected: "WHERE a = $8 AND b = $9 AND c = $10",
		},
		{
			name:     "$1 is left alone",
			sql:      "WHERE a = $1",
			expected: "WHERE a = $1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, RenumberParameters(tt.sql))
		})
	}
}
