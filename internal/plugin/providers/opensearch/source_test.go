package opensearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	opensearchgo "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      plugin.RawPluginConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with addresses",
			config: plugin.RawPluginConfig{
				"addresses": []interface{}{"http://localhost:9200"},
			},
			wantErr: false,
		},
		{
			name: "valid config with basic auth",
			config: plugin.RawPluginConfig{
				"addresses": []interface{}{"http://localhost:9200"},
				"username":  "admin",
				"password":  "admin",
			},
			wantErr: false,
		},
		{
			name:        "missing addresses",
			config:      plugin.RawPluginConfig{},
			wantErr:     true,
			errContains: "addresses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{}
			_, err := s.Validate(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSource_Validate_Defaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"addresses": []interface{}{"http://localhost:9200"},
	})
	require.NoError(t, err)

	assert.True(t, s.config.IncludeDataStreams)
	assert.True(t, s.config.IncludeAliases)
	assert.True(t, s.config.IncludeIndexStats)
	assert.False(t, s.config.IncludeSystemIndices)
}

func TestSource_Validate_ExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"addresses":            []interface{}{"http://localhost:9200"},
		"include_data_streams": false,
		"include_aliases":      false,
		"include_index_stats":  false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeDataStreams)
	assert.False(t, s.config.IncludeAliases)
	assert.False(t, s.config.IncludeIndexStats)
}

func TestFlattenMappingProperties(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		properties map[string]interface{}
		wantCount  int
		wantFields map[string]string
	}{
		{
			name:   "simple fields",
			prefix: "",
			properties: map[string]interface{}{
				"title": map[string]interface{}{"type": "text"},
				"count": map[string]interface{}{"type": "long"},
			},
			wantCount: 2,
			wantFields: map[string]string{
				"title": "text",
				"count": "long",
			},
		},
		{
			name:   "nested properties",
			prefix: "",
			properties: map[string]interface{}{
				"user": map[string]interface{}{
					"properties": map[string]interface{}{
						"name":  map[string]interface{}{"type": "keyword"},
						"email": map[string]interface{}{"type": "keyword"},
					},
				},
			},
			wantCount: 2,
			wantFields: map[string]string{
				"user.name":  "keyword",
				"user.email": "keyword",
			},
		},
		{
			name:   "deeply nested",
			prefix: "",
			properties: map[string]interface{}{
				"location": map[string]interface{}{
					"properties": map[string]interface{}{
						"address": map[string]interface{}{
							"properties": map[string]interface{}{
								"street": map[string]interface{}{"type": "text"},
							},
						},
					},
				},
			},
			wantCount:  1,
			wantFields: map[string]string{"location.address.street": "text"},
		},
		{
			name:       "empty properties",
			prefix:     "",
			properties: map[string]interface{}{},
			wantCount:  0,
			wantFields: map[string]string{},
		},
		{
			name:   "field without type is skipped",
			prefix: "",
			properties: map[string]interface{}{
				"weird": map[string]interface{}{"meta": "something"},
			},
			wantCount:  0,
			wantFields: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			columns := flattenMappingProperties(tt.prefix, tt.properties)
			assert.Len(t, columns, tt.wantCount)

			for _, col := range columns {
				name, _ := col["name"].(string)
				expectedType, exists := tt.wantFields[name]
				if assert.True(t, exists, "unexpected field: %s", name) {
					assert.Equal(t, expectedType, col["type"])
				}
			}
		})
	}
}

func TestSource_Discover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"cluster_name": "test-cluster",
				"version":      map[string]interface{}{"number": "2.12.0", "distribution": "opensearch"},
			})

		case r.URL.Path == "/_cat/indices":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"index":                "logs-2024.01",
					"health":               "green",
					"status":               "open",
					"uuid":                 "abc123",
					"pri":                  "1",
					"rep":                  "0",
					"docs.count":           "1000",
					"store.size":           "10mb",
					"creation.date.string": "2024-01-01T00:00:00.000Z",
				},
				{
					"index":      ".internal-index",
					"health":     "green",
					"status":     "open",
					"uuid":       "def456",
					"pri":        "1",
					"rep":        "0",
					"docs.count": "50",
					"store.size": "1mb",
				},
			})

		case r.URL.Path == "/logs-2024.01/_mapping":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"logs-2024.01": map[string]interface{}{
					"mappings": map[string]interface{}{
						"properties": map[string]interface{}{
							"message":   map[string]interface{}{"type": "text"},
							"timestamp": map[string]interface{}{"type": "date"},
						},
					},
				},
			})

		case r.URL.Path == "/_data_stream":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data_streams": []map[string]interface{}{
					{
						"name":            "logs-nginx",
						"timestamp_field": map[string]interface{}{"name": "@timestamp"},
						"indices": []map[string]interface{}{
							{"index_name": ".ds-logs-nginx-000001", "index_uuid": "u1"},
							{"index_name": ".ds-logs-nginx-000002", "index_uuid": "u2"},
						},
						"generation": 2,
						"status":     "GREEN",
						"template":   "logs-template",
					},
				},
			})

		case r.URL.Path == "/_cat/aliases":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"alias":          "current-logs",
					"index":          "logs-2024.01",
					"filter":         "-",
					"is_write_index": "true",
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"addresses": []interface{}{server.URL},
	})
	require.NoError(t, err)

	s.client, err = opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearchgo.Config{Addresses: []string{server.URL}},
	})
	require.NoError(t, err)

	result, err := s.Discover(context.Background(), plugin.RawPluginConfig{
		"addresses": []interface{}{server.URL},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	var indexAssets, dsAssets, aliasAssets int
	for _, a := range result.Assets {
		switch a.Type {
		case "Table":
			indexAssets++
			assert.Equal(t, "logs-2024.01", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://table/opensearch/")
			assert.Equal(t, []string{"OpenSearch"}, a.Providers)
			assert.Equal(t, "test-cluster", a.Metadata["cluster"])
		case "Data Stream":
			dsAssets++
			assert.Equal(t, "logs-nginx", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://data-stream/opensearch/")
		case "Alias":
			aliasAssets++
			assert.Equal(t, "current-logs", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://alias/opensearch/")
		}
	}

	assert.Equal(t, 1, indexAssets, "should discover 1 non-system index")
	assert.Equal(t, 1, dsAssets, "should discover 1 data stream")
	assert.Equal(t, 1, aliasAssets, "should discover 1 alias")

	assert.GreaterOrEqual(t, len(result.Lineage), 1)

	var referencesCount int
	for _, edge := range result.Lineage {
		if edge.Type == "REFERENCES" {
			referencesCount++
		}
	}
	assert.Equal(t, 1, referencesCount, "alias should REFERENCE 1 index")

	assert.GreaterOrEqual(t, len(result.Statistics), 1)
	assert.Equal(t, "docs_count", result.Statistics[0].MetricName)
	assert.Equal(t, float64(1000), result.Statistics[0].Value)
}
