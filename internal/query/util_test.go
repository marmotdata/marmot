package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataColumnRef(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{name: "single key", field: "owner", expected: `metadata->>'owner'`},
		{name: "nested path", field: "cloud.region", expected: `metadata->'cloud'->>'region'`},
		{name: "deeply nested path", field: "a.b.c", expected: `metadata->'a'->'b'->>'c'`},
		{name: "hyphenated key", field: "team-name", expected: `metadata->>'team-name'`},
		{
			name:     "quote in single key is escaped",
			field:    "x' OR TRUE OR metadata->>'x",
			expected: `metadata->>'x'' OR TRUE OR metadata->>''x'`,
		},
		{
			name:     "quote in nested key is escaped",
			field:    "a'--.b",
			expected: `metadata->'a''--'->>'b'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MetadataColumnRef(tt.field))
		})
	}
}

func TestQuoteLiteral(t *testing.T) {
	assert.Equal(t, `'plain'`, QuoteLiteral("plain"))
	assert.Equal(t, `'it''s'`, QuoteLiteral("it's"))
	assert.Equal(t, `''''''`, QuoteLiteral("''"))
	assert.Equal(t, `'\'''`, QuoteLiteral(`\'`))
}
