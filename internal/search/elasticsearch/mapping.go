package elasticsearch

import (
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/dynamicmapping"
)

// buildTypeMappings returns the type mapping for the marmot search index.
func buildTypeMappings() *types.TypeMapping {
	nameAnalyzer := "name_analyzer"
	falseVal := false
	dynTrue := dynamicmapping.True

	return &types.TypeMapping{
		DynamicTemplates: []map[string]types.DynamicTemplate{
			{
				"metadata_as_text": {
					PathMatch: []string{"metadata.*"},
					Mapping: &types.TextProperty{
						Type: "text",
					},
				},
			},
		},
		Properties: map[string]types.Property{
			"type":      types.NewKeywordProperty(),
			"entity_id": types.NewKeywordProperty(),
			"name": &types.TextProperty{
				Type:     "text",
				Analyzer: &nameAnalyzer,
				Fields: map[string]types.Property{
					"keyword": types.NewKeywordProperty(),
				},
			},
			"description":      &types.TextProperty{Type: "text"},
			"url_path":         &types.KeywordProperty{Type: "keyword", Index: &falseVal},
			"asset_type":       types.NewKeywordProperty(),
			"primary_provider": types.NewKeywordProperty(),
			"providers":        types.NewKeywordProperty(),
			"tags":             types.NewKeywordProperty(),
			"mrn":              types.NewKeywordProperty(),
			"created_by":       types.NewKeywordProperty(),
			"created_at":       &types.DateProperty{Type: "date"},
			"updated_at":       &types.DateProperty{Type: "date"},
			"documentation":    &types.TextProperty{Type: "text"},
			"metadata": &types.ObjectProperty{
				Type:    "object",
				Dynamic: &dynTrue,
			},
		},
	}
}

func (c *Client) buildIndexSettings() *types.IndexSettings {
	settings := &types.IndexSettings{
		Analysis: &types.IndexSettingsAnalysis{
			Analyzer: map[string]types.Analyzer{
				"name_analyzer": &types.CustomAnalyzer{
					Tokenizer: "standard",
					Filter:    []string{"lowercase", "asciifolding"},
				},
			},
		},
	}

	if c.shards != nil {
		s := fmt.Sprintf("%d", *c.shards)
		settings.NumberOfShards = &s
	}
	if c.replicas != nil {
		s := fmt.Sprintf("%d", *c.replicas)
		settings.NumberOfReplicas = &s
	}

	return settings
}
