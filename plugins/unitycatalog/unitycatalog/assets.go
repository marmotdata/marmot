package unitycatalog

import (
	"sort"
	"strconv"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// column is the per-column shape stored in an asset's schema: the SDK's
// canonical fields plus the two Unity Catalog adds.
type column struct {
	pluginsdk.Column
	TypeName       string `json:"type_name,omitempty"`
	PartitionIndex *int   `json:"partition_index,omitempty"`
}

func (d *discovery) catalogAsset(catalog catalogInfo, schemaCount, tableCount int) pluginsdk.Asset {
	metadata := map[string]any{
		"catalog_name": catalog.Name,
		"schema_count": schemaCount,
		"table_count":  tableCount,
	}
	setString(metadata, "comment", catalog.Comment)
	setString(metadata, "owner", catalog.Owner)
	setString(metadata, "catalog_id", catalog.ID)
	setProperties(metadata, catalog.Properties)
	setTime(metadata, "created_at", catalog.CreatedAt)
	setTime(metadata, "updated_at", catalog.UpdatedAt)

	return d.newAsset("Catalog", catalog.Name, catalog.Comment, metadata)
}

func (d *discovery) tableAsset(table tableInfo) pluginsdk.Asset {
	name := fullName(table.CatalogName, table.SchemaName, table.Name)

	assetType := "Table"
	switch table.TableType {
	case "VIEW", "MATERIALIZED_VIEW":
		assetType = "View"
	}

	metadata := map[string]any{
		"catalog":    table.CatalogName,
		"schema":     table.SchemaName,
		"table_name": table.Name,
	}
	setString(metadata, "table_type", table.TableType)
	setString(metadata, "data_source_format", table.DataSourceFormat)
	setString(metadata, "storage_location", table.StorageLocation)
	setString(metadata, "owner", table.Owner)
	setString(metadata, "comment", table.Comment)
	setString(metadata, "table_id", table.TableID)
	setProperties(metadata, table.Properties)
	setTime(metadata, "created_at", table.CreatedAt)
	setTime(metadata, "updated_at", table.UpdatedAt)

	if partitions := partitionColumns(table.Columns); len(partitions) > 0 {
		metadata["partition_columns"] = partitions
	}

	primaryKey, foreignKeys := constraints(table.TableConstraints)
	if len(primaryKey) > 0 {
		metadata["primary_key"] = primaryKey
	}
	if len(foreignKeys) > 0 {
		metadata["foreign_keys"] = foreignKeys
	}

	asset := d.newAsset(assetType, name, table.Comment, metadata)

	if assetType == "View" && table.ViewDefinition != "" {
		query := table.ViewDefinition
		language := "SQL"
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	if d.config.IncludeColumns && len(table.Columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, buildColumns(table.Columns, primaryKey)); err != nil {
			log.Warn().Err(err).Str("table", name).Msg("Failed to set columns")
		}
	}

	return asset
}

func (d *discovery) volumeAsset(volume volumeInfo) pluginsdk.Asset {
	name := fullName(volume.CatalogName, volume.SchemaName, volume.Name)

	metadata := map[string]any{
		"catalog":     volume.CatalogName,
		"schema":      volume.SchemaName,
		"volume_name": volume.Name,
	}
	setString(metadata, "volume_type", volume.VolumeType)
	setString(metadata, "storage_location", volume.StorageLocation)
	setString(metadata, "comment", volume.Comment)
	setString(metadata, "owner", volume.Owner)
	setString(metadata, "volume_id", volume.VolumeID)
	setTime(metadata, "created_at", volume.CreatedAt)
	setTime(metadata, "updated_at", volume.UpdatedAt)

	return d.newAsset("Volume", name, volume.Comment, metadata)
}

func (d *discovery) functionAsset(function functionInfo) pluginsdk.Asset {
	name := fullName(function.CatalogName, function.SchemaName, function.Name)

	metadata := map[string]any{
		"catalog":       function.CatalogName,
		"schema":        function.SchemaName,
		"function_name": function.Name,
	}
	if params := parameterList(function.InputParams.Parameters); len(params) > 0 {
		metadata["parameters"] = params
	}
	returnType := function.FullDataType
	if returnType == "" {
		returnType = function.DataType
	}
	setString(metadata, "return_type", returnType)
	setString(metadata, "routine_body", function.RoutineBody)
	setString(metadata, "language", functionLanguage(function))
	metadata["is_deterministic"] = function.IsDeterministic
	setString(metadata, "sql_data_access", function.SQLDataAccess)
	setString(metadata, "comment", function.Comment)
	setString(metadata, "owner", function.Owner)
	setString(metadata, "function_id", function.FunctionID)
	setTime(metadata, "created_at", function.CreatedAt)
	setTime(metadata, "updated_at", function.UpdatedAt)

	asset := d.newAsset("Function", name, function.Comment, metadata)

	if function.RoutineDefinition != "" {
		query := function.RoutineDefinition
		language := functionLanguage(function)
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	return asset
}

func (d *discovery) modelAsset(model registeredModelInfo, versions []modelVersionInfo) pluginsdk.Asset {
	name := fullName(model.CatalogName, model.SchemaName, model.Name)

	metadata := map[string]any{
		"catalog":       model.CatalogName,
		"schema":        model.SchemaName,
		"model_name":    model.Name,
		"version_count": len(versions),
	}
	setString(metadata, "comment", model.Comment)
	setString(metadata, "owner", model.Owner)
	setString(metadata, "storage_location", model.StorageLocation)
	setString(metadata, "model_id", model.ID)
	setTime(metadata, "created_at", model.CreatedAt)
	setTime(metadata, "updated_at", model.UpdatedAt)

	if len(versions) > 0 {
		latest := 0
		statuses := make(map[string]string, len(versions))
		for _, v := range versions {
			if v.Version > latest {
				latest = v.Version
			}
			statuses[strconv.Itoa(v.Version)] = v.Status
		}
		metadata["latest_version"] = latest
		metadata["versions"] = statuses
	}

	return d.newAsset("Model", name, model.Comment, metadata)
}

// newAsset fills in the fields every asset shares.
func (d *discovery) newAsset(assetType, name, comment string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	var description *string
	if comment != "" {
		description = &comment
	}

	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        assetType,
		Providers:   []string{provider},
		Description: description,
		Metadata:    metadata,
		Schema:      make(map[string]string),
		Tags:        pluginsdk.InterpolateTags(d.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// buildColumns orders columns by their declared position and marks the
// ones named by the primary key constraint.
func buildColumns(columns []columnInfo, primaryKey []string) []column {
	sorted := make([]columnInfo, len(columns))
	copy(sorted, columns)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })

	pk := make(map[string]struct{}, len(primaryKey))
	for _, name := range primaryKey {
		pk[name] = struct{}{}
	}

	result := make([]column, 0, len(sorted))
	for _, c := range sorted {
		_, isPK := pk[c.Name]
		result = append(result, column{
			Column: pluginsdk.Column{
				Name:        c.Name,
				DataType:    c.TypeText,
				Nullable:    c.Nullable,
				PrimaryKey:  isPK,
				Description: c.Comment,
			},
			TypeName:       c.TypeName,
			PartitionIndex: c.PartitionIndex,
		})
	}
	return result
}

// partitionColumns lists partition column names in partition order.
func partitionColumns(columns []columnInfo) []string {
	var partitioned []columnInfo
	for _, c := range columns {
		if c.PartitionIndex != nil {
			partitioned = append(partitioned, c)
		}
	}
	sort.SliceStable(partitioned, func(i, j int) bool {
		return *partitioned[i].PartitionIndex < *partitioned[j].PartitionIndex
	})

	names := make([]string, 0, len(partitioned))
	for _, c := range partitioned {
		names = append(names, c.Name)
	}
	return names
}

// constraints splits the table's constraints into the primary key columns
// and a description of each foreign key.
func constraints(tableConstraints []tableConstraint) ([]string, []map[string]any) {
	var primaryKey []string
	var foreignKeys []map[string]any

	for _, tc := range tableConstraints {
		if tc.PrimaryKey != nil {
			primaryKey = append(primaryKey, tc.PrimaryKey.ChildColumns...)
		}
		if tc.ForeignKey != nil {
			fk := map[string]any{
				"columns":        tc.ForeignKey.ChildColumns,
				"parent_table":   tc.ForeignKey.ParentTable,
				"parent_columns": tc.ForeignKey.ParentColumns,
			}
			setString(fk, "name", tc.ForeignKey.Name)
			foreignKeys = append(foreignKeys, fk)
		}
	}

	return primaryKey, foreignKeys
}

// parameterList renders a function's parameters as "name type", in
// declaration order.
func parameterList(parameters []functionParameter) []string {
	sorted := make([]functionParameter, len(parameters))
	copy(sorted, parameters)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })

	result := make([]string, 0, len(sorted))
	for _, p := range sorted {
		result = append(result, p.Name+" "+p.TypeText)
	}
	return result
}

// functionLanguage is the language a function body is written in: the
// external language for EXTERNAL routines, SQL otherwise.
func functionLanguage(function functionInfo) string {
	if function.RoutineBody == "EXTERNAL" && function.ExternalLanguage != "" {
		return function.ExternalLanguage
	}
	return "SQL"
}

func setString(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

func setProperties(metadata map[string]any, properties map[string]string) {
	if len(properties) > 0 {
		metadata["properties"] = properties
	}
}

// setTime records an epoch-millisecond timestamp as RFC 3339, skipping the
// zero the API sends for objects that were never updated.
func setTime(metadata map[string]any, key string, millis int64) {
	if millis > 0 {
		metadata[key] = time.UnixMilli(millis).UTC().Format(time.RFC3339)
	}
}
