package elasticsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
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
			name: "valid config with cloud_id",
			config: plugin.RawPluginConfig{
				"cloud_id": "my-deployment:dXMtY2VudHJhbC0xLmdjcC5jbG91ZC5lcy5pbzo0NDMkMGNmNQ==",
			},
			wantErr: false,
		},
		{
			name: "valid config with api_key",
			config: plugin.RawPluginConfig{
				"addresses": []interface{}{"http://localhost:9200"},
				"api_key":   "my-api-key",
			},
			wantErr: false,
		},
		{
			name: "valid config with basic auth",
			config: plugin.RawPluginConfig{
				"addresses": []interface{}{"http://localhost:9200"},
				"username":  "elastic",
				"password":  "changeme",
			},
			wantErr: false,
		},
		{
			name:        "missing addresses and cloud_id",
			config:      plugin.RawPluginConfig{},
			wantErr:     true,
			errContains: "either addresses or cloud_id is required",
		},
		{
			name: "conflicting auth methods",
			config: plugin.RawPluginConfig{
				"addresses": []interface{}{"http://localhost:9200"},
				"api_key":   "my-api-key",
				"username":  "elastic",
				"password":  "changeme",
			},
			wantErr:     true,
			errContains: "mutually exclusive",
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

	assert.True(t, s.config.IncludeDataStreams, "IncludeDataStreams should default to true")
	assert.True(t, s.config.IncludeAliases, "IncludeAliases should default to true")
	assert.True(t, s.config.IncludeIndexStats, "IncludeIndexStats should default to true")
	assert.False(t, s.config.IncludeSystemIndices, "IncludeSystemIndices should default to false")
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
		wantFields map[string]string // field name -> expected type
	}{
		{
			name:   "simple fields",
			prefix: "",
			properties: map[string]interface{}{
				"title": map[string]interface{}{
					"type": "text",
				},
				"count": map[string]interface{}{
					"type": "long",
				},
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
						"name": map[string]interface{}{
							"type": "keyword",
						},
						"email": map[string]interface{}{
							"type": "keyword",
						},
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
			name:   "deeply nested properties",
			prefix: "",
			properties: map[string]interface{}{
				"location": map[string]interface{}{
					"properties": map[string]interface{}{
						"address": map[string]interface{}{
							"properties": map[string]interface{}{
								"street": map[string]interface{}{
									"type": "text",
								},
							},
						},
					},
				},
			},
			wantCount: 1,
			wantFields: map[string]string{
				"location.address.street": "text",
			},
		},
		{
			name:   "with prefix",
			prefix: "root",
			properties: map[string]interface{}{
				"field": map[string]interface{}{
					"type": "keyword",
				},
			},
			wantCount: 1,
			wantFields: map[string]string{
				"root.field": "keyword",
			},
		},
		{
			name:       "empty properties",
			prefix:     "",
			properties: map[string]interface{}{},
			wantCount:  0,
			wantFields: map[string]string{},
		},
		{
			name:   "field with analyzer",
			prefix: "",
			properties: map[string]interface{}{
				"content": map[string]interface{}{
					"type":     "text",
					"analyzer": "standard",
				},
			},
			wantCount: 1,
			wantFields: map[string]string{
				"content": "text",
			},
		},
		{
			name:   "field without type is skipped",
			prefix: "",
			properties: map[string]interface{}{
				"weird": map[string]interface{}{
					"meta": "something",
				},
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
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		switch {
		case r.URL.Path == "/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"cluster_name": "test-cluster",
				"version": map[string]interface{}{
					"number": "8.12.0",
				},
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
					"index":                ".internal-index",
					"health":               "green",
					"status":               "open",
					"uuid":                 "def456",
					"pri":                  "1",
					"rep":                  "0",
					"docs.count":           "50",
					"store.size":           "1mb",
					"creation.date.string": "2024-01-01T00:00:00.000Z",
				},
			})

		case r.URL.Path == "/logs-2024.01/_mapping":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"logs-2024.01": map[string]interface{}{
					"mappings": map[string]interface{}{
						"properties": map[string]interface{}{
							"message": map[string]interface{}{
								"type": "text",
							},
							"timestamp": map[string]interface{}{
								"type": "date",
							},
						},
					},
				},
			})

		case r.URL.Path == "/_data_stream":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data_streams": []map[string]interface{}{
					{
						"name": "logs-nginx",
						"timestamp_field": map[string]interface{}{
							"name": "@timestamp",
						},
						"indices": []map[string]interface{}{
							{"index_name": ".ds-logs-nginx-000001"},
							{"index_name": ".ds-logs-nginx-000002"},
						},
						"generation": 2,
						"status":     "GREEN",
						"ilm_policy": "logs-policy",
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

	// Override the client to use the test server
	s.client, err = elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{server.URL},
	})
	require.NoError(t, err)

	result, err := s.Discover(context.Background(), plugin.RawPluginConfig{
		"addresses": []interface{}{server.URL},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify index assets (system index should be excluded)
	var indexAssets, dsAssets, aliasAssets int
	for _, a := range result.Assets {
		switch a.Type {
		case "Table":
			indexAssets++
			assert.Equal(t, "logs-2024.01", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://table/elasticsearch/")
			assert.Equal(t, []string{"Elasticsearch"}, a.Providers)
			assert.Equal(t, "test-cluster", a.Metadata["cluster"])
		case "Data Stream":
			dsAssets++
			assert.Equal(t, "logs-nginx", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://data-stream/elasticsearch/")
		case "Alias":
			aliasAssets++
			assert.Equal(t, "current-logs", *a.Name)
			assert.Contains(t, *a.MRN, "mrn://alias/elasticsearch/")
		}
	}

	assert.Equal(t, 1, indexAssets, "should discover 1 non-system index")
	assert.Equal(t, 1, dsAssets, "should discover 1 data stream")
	assert.Equal(t, 1, aliasAssets, "should discover 1 alias")

	// Verify lineage
	assert.Equal(t, 1, len(result.Lineage), "should have 1 lineage edge (alias REFERENCES)")

	var containsCount, referencesCount int
	for _, edge := range result.Lineage {
		switch edge.Type {
		case "CONTAINS":
			containsCount++
		case "REFERENCES":
			referencesCount++
		}
	}
	assert.Equal(t, 0, containsCount, "backing indices are system indices, no CONTAINS edges")
	assert.Equal(t, 1, referencesCount, "alias should REFERENCE 1 index")

	// Verify statistics
	assert.GreaterOrEqual(t, len(result.Statistics), 1, "should have statistics")
	assert.Equal(t, "docs_count", result.Statistics[0].MetricName)
	assert.Equal(t, float64(1000), result.Statistics[0].Value)
}

func TestSource_Discover_SystemIndicesIncluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		switch {
		case r.URL.Path == "/":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"cluster_name": "test-cluster",
			})

		case r.URL.Path == "/_cat/indices":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"index":      "logs",
					"health":     "green",
					"status":     "open",
					"uuid":       "abc",
					"pri":        "1",
					"rep":        "0",
					"docs.count": "100",
					"store.size": "1mb",
				},
				{
					"index":      ".kibana",
					"health":     "green",
					"status":     "open",
					"uuid":       "def",
					"pri":        "1",
					"rep":        "0",
					"docs.count": "50",
					"store.size": "500kb",
				},
			})

		case r.URL.Path == "/_data_stream":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data_streams": []interface{}{},
			})

		case r.URL.Path == "/_cat/aliases":
			_ = json.NewEncoder(w).Encode([]interface{}{})

		default:
			// Mapping requests
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer server.Close()

	s := &Source{}
	_, err := s.Validate(plugin.RawPluginConfig{
		"addresses":              []interface{}{server.URL},
		"include_system_indices": true,
	})
	require.NoError(t, err)

	s.client, err = elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{server.URL},
	})
	require.NoError(t, err)

	result, err := s.Discover(context.Background(), plugin.RawPluginConfig{
		"addresses":              []interface{}{server.URL},
		"include_system_indices": true,
	})
	require.NoError(t, err)

	assert.Len(t, result.Assets, 2, "should include system index when configured")
}
