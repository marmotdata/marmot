package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"time"

	iceberggo "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
	"github.com/apache/iceberg-go/view"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// viewCatalog is implemented by catalog types that support views.
type viewCatalog interface {
	ListViews(ctx context.Context, namespace icetable.Identifier) iter.Seq2[icetable.Identifier, error]
	LoadView(ctx context.Context, identifier icetable.Identifier) (*view.View, error)
}

func (s *Source) discoverTables(ctx context.Context, namespaces map[string]icetable.Identifier) ([]asset.Asset, error) {
	var assets []asset.Asset

	for nsPath, ns := range namespaces {
		for ident, err := range s.cat.ListTables(ctx, ns) {
			if err != nil {
				log.Warn().Err(err).Str("namespace", nsPath).Msg("Error listing tables")
				break
			}

			tbl, err := s.cat.LoadTable(ctx, ident)
			if err != nil {
				tableName := catalog.TableNameFromIdent(ident)
				log.Warn().Err(err).Str("table", tableName).Str("namespace", nsPath).Msg("Failed to load table")
				continue
			}

			a := s.createTableAsset(tbl, ident)
			assets = append(assets, a)
		}
	}

	return assets, nil
}

func (s *Source) discoverViews(ctx context.Context, namespaces map[string]icetable.Identifier) ([]asset.Asset, error) {
	vc, ok := s.cat.(viewCatalog)
	if !ok {
		log.Debug().Msg("Catalog does not support views")
		return nil, nil
	}

	var assets []asset.Asset

	for nsPath, ns := range namespaces {
		for ident, err := range vc.ListViews(ctx, ns) {
			if err != nil {
				log.Warn().Err(err).Str("namespace", nsPath).Msg("Error listing views")
				break
			}

			v, err := vc.LoadView(ctx, ident)
			if err != nil {
				viewName := catalog.TableNameFromIdent(ident)
				log.Warn().Err(err).Str("view", viewName).Str("namespace", nsPath).Msg("Failed to load view")
				continue
			}

			a := s.createViewAsset(v, ident)
			assets = append(assets, a)
		}
	}

	return assets, nil
}

func (s *Source) createTableAsset(tbl *icetable.Table, ident icetable.Identifier) asset.Asset {
	metadata := make(map[string]interface{})
	tableName := catalog.TableNameFromIdent(ident)
	fullName := strings.Join(ident, ".")
	meta := tbl.Metadata()

	metadata["table_uuid"] = meta.TableUUID().String()
	metadata["location"] = meta.Location()
	metadata["format_version"] = meta.Version()
	metadata["last_updated_ms"] = meta.LastUpdatedMillis()

	var schemaMap map[string]string
	schema := tbl.Schema()
	if schema != nil {
		metadata["schema_field_count"] = len(schema.Fields())
		schemaMap = map[string]string{"columns": extractSchemaFields(schema)}
	}

	sortOrder := tbl.SortOrder()
	if sortOrder.Len() > 0 {
		metadata["sort_order"] = sortOrder.String()
	}

	snapshots := meta.Snapshots()
	metadata["snapshot_count"] = len(snapshots)

	if snap := tbl.CurrentSnapshot(); snap != nil {
		metadata["current_snapshot_id"] = strconv.FormatInt(snap.SnapshotID, 10)
		if snap.Summary != nil {
			if v, ok := snap.Summary.Properties["total-records"]; ok {
				metadata["total_records"] = v
			}
			if v, ok := snap.Summary.Properties["total-data-files"]; ok {
				metadata["total_data_files"] = v
			}
			if v, ok := snap.Summary.Properties["total-files-size"]; ok {
				metadata["total_file_size"] = v
			}
		}
	}

	var description *string
	for k, v := range tbl.Properties() {
		if k == "description" {
			desc := v
			description = &desc
			continue
		}
		metadata["property."+k] = v
	}

	mrnValue := mrn.New("Table", "Iceberg", fullName)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &tableName,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"Iceberg"},
		Description: description,
		Schema:      schemaMap,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Iceberg",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) createViewAsset(v *view.View, ident icetable.Identifier) asset.Asset {
	metadata := make(map[string]interface{})
	viewName := catalog.TableNameFromIdent(ident)
	fullName := strings.Join(ident, ".")
	meta := v.Metadata()

	metadata["view_uuid"] = meta.ViewUUID().String()
	metadata["location"] = meta.Location()
	metadata["format_version"] = meta.FormatVersion()

	var schemaMap map[string]string
	schema := v.CurrentSchema()
	if schema != nil {
		metadata["schema_field_count"] = len(schema.Fields())
		schemaMap = map[string]string{"columns": extractSchemaFields(schema)}
	}

	var query *string
	sqlLang := "SQL"
	if ver := v.CurrentVersion(); ver != nil {
		if len(ver.Representations) > 0 {
			query = &ver.Representations[0].Sql
		}
	}

	var description *string
	for k, v := range v.Properties() {
		if k == "description" {
			desc := v
			description = &desc
			continue
		}
		metadata["property."+k] = v
	}

	mrnValue := mrn.New("View", "Iceberg", fullName)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:          &viewName,
		MRN:           &mrnValue,
		Type:          "View",
		Providers:     []string{"Iceberg"},
		Description:   description,
		Schema:        schemaMap,
		Query:         query,
		QueryLanguage: &sqlLang,
		Metadata:      metadata,
		Tags:          processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Iceberg",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

type schemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Doc      string `json:"doc,omitempty"`
}

// extractSchemaFields returns the schema fields as a JSON string.
func extractSchemaFields(schema *iceberggo.Schema) string {
	fields := schema.Fields()
	result := make([]schemaField, 0, len(fields))
	for _, f := range fields {
		sf := schemaField{
			Name:     f.Name,
			Required: f.Required,
			Doc:      f.Doc,
		}
		if f.Type != nil {
			sf.Type = f.Type.String()
		}
		result = append(result, sf)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("error marshaling schema: %v", err)
	}
	return string(data)
}
