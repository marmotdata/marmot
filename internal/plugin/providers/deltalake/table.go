package deltalake

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func createTableAsset(snapshot *deltaSnapshot, tablePath string, config *Config) asset.Asset {
	metadata := make(map[string]interface{})
	tableName := filepath.Base(tablePath)

	metadata["location"] = tablePath
	metadata["current_version"] = snapshot.CurrentVersion
	metadata["num_files"] = snapshot.NumFiles
	metadata["total_size"] = snapshot.TotalSize

	if snapshot.MetaData != nil {
		md := snapshot.MetaData
		if md.ID != "" {
			metadata["table_id"] = md.ID
		}
		if md.Format.Provider != "" {
			metadata["format"] = md.Format.Provider
		}
		if len(md.PartitionColumns) > 0 {
			metadata["partition_columns"] = strings.Join(md.PartitionColumns, ", ")
		}
		if md.CreatedTime > 0 {
			metadata["created_time"] = md.CreatedTime
		}

		if md.SchemaString != "" {
			schema, err := parseSchemaString(md.SchemaString)
			if err == nil {
				metadata["schema_field_count"] = len(schema.Fields)
			}
		}

		for k, v := range md.Configuration {
			metadata["property."+k] = v
		}
	}

	if snapshot.Protocol != nil {
		metadata["min_reader_version"] = snapshot.Protocol.MinReaderVersion
		metadata["min_writer_version"] = snapshot.Protocol.MinWriterVersion
	}

	var description *string
	if snapshot.MetaData != nil && snapshot.MetaData.Description != "" {
		desc := snapshot.MetaData.Description
		description = &desc
	}

	var schemaMap map[string]string
	if snapshot.MetaData != nil && snapshot.MetaData.SchemaString != "" {
		schema, err := parseSchemaString(snapshot.MetaData.SchemaString)
		if err == nil && len(schema.Fields) > 0 {
			schemaMap = map[string]string{"columns": extractSchemaFields(schema)}
		}
	}

	mrnValue := mrn.New("Table", "DeltaLake", tableName)
	processedTags := plugin.InterpolateTags(config.Tags, metadata)

	return asset.Asset{
		Name:        &tableName,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"Delta Lake"},
		Description: description,
		Schema:      schemaMap,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Delta Lake",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

type schemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// extractSchemaFields converts Delta schema fields to a JSON string.
func extractSchemaFields(schema *deltaSchema) string {
	result := make([]schemaField, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		sf := schemaField{
			Name:     f.Name,
			Nullable: f.Nullable,
		}
		switch t := f.Type.(type) {
		case string:
			sf.Type = t
		default:
			data, err := json.Marshal(t)
			if err == nil {
				sf.Type = string(data)
			} else {
				sf.Type = fmt.Sprintf("%v", t)
			}
		}
		result = append(result, sf)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("error marshaling schema: %v", err)
	}
	return string(data)
}
