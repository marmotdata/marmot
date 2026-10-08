package mcp

import (
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/stretchr/testify/assert"
)

func TestBestFilterMatch(t *testing.T) {
	providers := []string{"Airflow", "BigQuery", "Elasticsearch", "Kafka", "Metabase", "MongoDB", "PostgreSQL", "Redis", "S3", "dbt"}
	types := []string{"Bucket", "Collection", "Dashboard", "Database", "Dataset", "Model", "Pipeline", "Table", "Topic", "View"}

	cases := []struct {
		name  string
		value string
		known []string
		want  string
		ok    bool
	}{
		{"exact", "PostgreSQL", providers, "PostgreSQL", true},
		{"lowercase provider", "kafka", providers, "Kafka", true},
		{"abbreviation resolves by prefix", "postgres", providers, "PostgreSQL", true},
		{"lowercase type", "database", types, "Database", true},
		{"plural type resolves to singular", "databases", types, "Database", true},
		{"mixed case", "bigquery", providers, "BigQuery", true},
		{"unknown value passes through", "snowflake", providers, "", false},
		{"empty value", "", providers, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := bestFilterMatch(c.value, c.known)
			assert.Equal(t, c.ok, ok)
			if c.ok {
				assert.Equal(t, c.want, got)
			}
		})
	}
}

// An exact case-insensitive match must win over a looser prefix or substring match, so "S3" never resolves to some other provider that merely contains the letters.
func TestBestFilterMatch_ExactWinsOverLooser(t *testing.T) {
	known := []string{"S3", "S3-Glacier", "S3-Express"}
	got, ok := bestFilterMatch("s3", known)
	assert.True(t, ok)
	assert.Equal(t, "S3", got)
}

// A very short value must not latch onto an arbitrary catalog entry through the substring fallback.
func TestBestFilterMatch_ShortValueNoSubstringFallback(t *testing.T) {
	_, ok := bestFilterMatch("q", []string{"BigQuery"})
	assert.False(t, ok)
}

func TestCanonicaliseFilterValues(t *testing.T) {
	summary := &asset.AssetSummary{
		Providers: map[string]int{"PostgreSQL": 3, "Kafka": 1},
		Tags:      map[string]int{"pii-scrubbed": 2},
	}

	values, rewrites := canonicaliseFilterValues([]string{"postgres", "Kafka", "unknown"}, filterKindProvider, summary)
	assert.Equal(t, []string{"PostgreSQL", "Kafka", "unknown"}, values)
	assert.Equal(t, []string{`provider "postgres" matched catalog value "PostgreSQL"`}, rewrites, "only genuine rewrites must be reported")

	values, rewrites = canonicaliseFilterValues([]string{"pii"}, filterKindTag, summary)
	assert.Equal(t, []string{"pii-scrubbed"}, values)
	assert.Len(t, rewrites, 1)

	values, rewrites = canonicaliseFilterValues([]string{"x"}, filterKindTag, nil)
	assert.Equal(t, []string{"x"}, values, "a nil summary must leave values untouched")
	assert.Empty(t, rewrites)
}
