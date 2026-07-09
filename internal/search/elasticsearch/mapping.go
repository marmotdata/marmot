package elasticsearch

// buildTypeMappings returns the type mapping JSON for the marmot search index.
func buildTypeMappings() map[string]any {
	return map[string]any{
		"dynamic_templates": []map[string]any{
			{
				"metadata_as_text": map[string]any{
					"path_match": []string{"metadata.*"},
					"mapping":    map[string]any{"type": "text"},
				},
			},
		},
		"properties": map[string]any{
			"type":      map[string]any{"type": "keyword"},
			"entity_id": map[string]any{"type": "keyword"},
			"name": map[string]any{
				"type":     "text",
				"analyzer": "name_analyzer",
				"fields": map[string]any{
					"keyword": map[string]any{"type": "keyword"},
				},
			},
			"description":      map[string]any{"type": "text"},
			"url_path":         map[string]any{"type": "keyword", "index": false},
			"asset_type":       map[string]any{"type": "keyword"},
			"primary_provider": map[string]any{"type": "keyword"},
			"providers":        map[string]any{"type": "keyword"},
			"tags":             map[string]any{"type": "keyword"},
			"mrn":              map[string]any{"type": "keyword"},
			"created_by":       map[string]any{"type": "keyword"},
			"created_at":       map[string]any{"type": "date"},
			"updated_at":       map[string]any{"type": "date"},
			"documentation":    map[string]any{"type": "text"},
			"metadata": map[string]any{
				"type":    "object",
				"dynamic": true,
			},
		},
	}
}

func (c *Client) buildIndexSettings() map[string]any {
	settings := map[string]any{
		"analysis": map[string]any{
			"analyzer": map[string]any{
				"name_analyzer": map[string]any{
					"type":      "custom",
					"tokenizer": "standard",
					"filter":    []string{"lowercase", "asciifolding"},
				},
			},
		},
	}

	if c.shards != nil {
		settings["number_of_shards"] = *c.shards
	}
	if c.replicas != nil {
		settings["number_of_replicas"] = *c.replicas
	}

	return settings
}
