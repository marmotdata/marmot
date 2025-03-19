package lineage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/marmotdata/marmot/internal/core/asset"
)

type SchemaField struct {
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Fields      []SchemaField `json:"fields"`
}

type InputDataset struct {
	Namespace   string                     `json:"namespace"`
	Name        string                     `json:"name"`
	Facets      map[string]json.RawMessage `json:"facets"`
	InputFacets map[string]json.RawMessage `json:"inputFacets,omitempty"`
}

type OutputDataset struct {
	Namespace    string                     `json:"namespace"`
	Name         string                     `json:"name"`
	Facets       map[string]json.RawMessage `json:"facets"`
	OutputFacets map[string]json.RawMessage `json:"outputFacets,omitempty"`
}

type DatasetDetector struct {
	namespace string
	name      string
	facets    map[string]json.RawMessage
}

func NewDatasetDetector(namespace, name string, facets map[string]json.RawMessage) *DatasetDetector {
	return &DatasetDetector{
		namespace: namespace,
		name:      name,
		facets:    facets,
	}
}

type AssetType struct {
	Name           string
	DetectionRules []string // List of identifiers in facets
}

var SupportedAssetTypes = map[string]AssetType{
	"TABLE": {
		Name:           "TABLE",
		DetectionRules: []string{"postgres", "snowflake", "redshift", "mysql"},
	},
	"TOPIC": {
		Name:           "TOPIC",
		DetectionRules: []string{"kafka", "pulsar"},
	},
	"BUCKET": {
		Name:           "BUCKET",
		DetectionRules: []string{"s3", "gcs", "azure"},
	},
}

func (d *DatasetDetector) DetectAssetType() string {
	if dsFacet, ok := d.facets["dataSource"]; ok {
		var ds struct {
			Name string `json:"name"`
			Uri  string `json:"uri"`
		}
		if err := json.Unmarshal(dsFacet, &ds); err == nil {
			// Look for asset type based on URI scheme
			uriScheme := strings.Split(ds.Uri, "://")[0]

			for _, assetType := range SupportedAssetTypes {
				for _, rule := range assetType.DetectionRules {
					if strings.Contains(strings.ToLower(uriScheme), rule) {
						return assetType.Name
					}
				}
			}
		}
	}

	// Default to CUSTOM if no match found
	return "CUSTOM"
}

func (d *DatasetDetector) ExtractSchema() (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	if schemaFacet, ok := d.facets["schema"]; ok {
		var schemaInfo struct {
			Fields []SchemaField `json:"fields"`
			// Add other OpenLineage schema fields if needed
			Producer  string `json:"_producer"`
			SchemaURL string `json:"_schemaURL"`
		}

		if err := json.Unmarshal(schemaFacet, &schemaInfo); err == nil {
			fmt.Printf("DEBUG: Extracted schema fields: %+v\n", schemaInfo)
			if len(schemaInfo.Fields) > 0 {
				// Create fields array with proper structure
				fields := make([]map[string]interface{}, 0)
				for _, field := range schemaInfo.Fields {
					fieldMap := map[string]interface{}{
						"name":        field.Name,
						"type":        field.Type,
						"description": field.Description,
					}
					if len(field.Fields) > 0 {
						fieldMap["fields"] = field.Fields
					}
					fields = append(fields, fieldMap)
				}
				schema["fields"] = fields
			}
		} else {
			fmt.Printf("DEBUG: Error unmarshaling schema: %v\n", err)
			return nil, fmt.Errorf("unmarshaling schema: %w", err)
		}
	} else {
		fmt.Printf("DEBUG: No schema facet found in facets: %v\n", d.facets)
	}

	// Log the final schema we're returning
	fmt.Printf("DEBUG: Returning schema: %+v\n", schema)
	return schema, nil
}

func (d *DatasetDetector) ExtractMetadata() (map[string]interface{}, string, error) {
	metadata := make(map[string]interface{})
	var dataSource string

	// Extract data source information
	if dsF, ok := d.facets["dataSource"]; ok {
		var ds struct {
			Name string `json:"name"`
			Uri  string `json:"uri"`
		}
		if err := json.Unmarshal(dsF, &ds); err == nil {
			metadata["datasource"] = ds
			dataSource = strings.Split(ds.Name, "://")[0]
		}
	}

	// Extract quality metrics if available
	if metrics, ok := d.facets["dataQualityMetrics"]; ok {
		var qualityMetrics map[string]interface{}
		if err := json.Unmarshal(metrics, &qualityMetrics); err == nil {
			metadata["qualityMetrics"] = qualityMetrics
		}
	}

	return metadata, dataSource, nil
}

func (d *InputDataset) DetectAsset() (*asset.CreateInput, error) {
	return detectDatasetAsset(d.Namespace, d.Name, d.InputFacets)
}

func (d *OutputDataset) DetectAsset() (*asset.CreateInput, error) {
	return detectDatasetAsset(d.Namespace, d.Name, d.OutputFacets)
}

func extractTableName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return name
}

func extractDataSource(namespace string) string {
	if strings.Contains(namespace, "://") {
		parts := strings.Split(namespace, "://")
		if len(parts) > 0 {
			return strings.Split(parts[0], ":")[0]
		}
	}
	return namespace
}

func detectDatasetAsset(namespace, name string, facets map[string]json.RawMessage) (*asset.CreateInput, error) {
	detector := NewDatasetDetector(namespace, name, facets)

	schema, err := detector.ExtractSchema()
	if err != nil {
		return nil, fmt.Errorf("extracting schema: %w", err)
	}

	metadata, dataSource, err := detector.ExtractMetadata()
	if err != nil {
		return nil, fmt.Errorf("extracting metadata: %w", err)
	}

	assetType := detector.DetectAssetType()
	tableName := strings.Split(name, ".")[len(strings.Split(name, "."))-1]

	mrn := fmt.Sprintf("mrn://%s/%s/%s", strings.ToLower(assetType), dataSource, tableName)

	return &asset.CreateInput{
		Name:      &tableName,
		Type:      assetType,
		Providers: []string{dataSource},
		MRN:       &mrn,
		CreatedBy: "system",
		Tags:      []string{dataSource},
		Schema:    schema,
		Metadata:  metadata,
	}, nil
}
