package presto

// PrestoCatalogFields describes the metadata fields emitted for catalog
// assets. It is a documentation-only struct so downstream tooling can
// introspect the shape of the metadata map.
type PrestoCatalogFields struct {
	CatalogName   string `json:"catalog_name" metadata:"catalog_name" description:"Presto catalog name"`
	ConnectorName string `json:"connector_name" metadata:"connector_name" description:"Connector backing the catalog"`
	SchemaCount   int    `json:"schema_count" metadata:"schema_count" description:"Number of discovered schemas"`
	TableCount    int    `json:"table_count" metadata:"table_count" description:"Number of discovered tables"`
	ViewCount     int    `json:"view_count" metadata:"view_count" description:"Number of discovered views"`
	Host          string `json:"host" metadata:"host" description:"Presto coordinator hostname"`
	Port          int    `json:"port" metadata:"port" description:"Presto coordinator port"`
	PrestoVersion string `json:"presto_version" metadata:"presto_version" description:"Presto coordinator version"`
}

// PrestoTableFields describes the metadata fields emitted for table and
// view assets.
type PrestoTableFields struct {
	Catalog       string `json:"catalog" metadata:"catalog" description:"Parent catalog name"`
	Schema        string `json:"schema" metadata:"schema" description:"Parent schema name"`
	TableName     string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	TableType     string `json:"table_type" metadata:"table_type" description:"BASE TABLE or VIEW"`
	ConnectorName string `json:"connector_name" metadata:"connector_name" description:"Connector backing the catalog"`
	Comment       string `json:"comment" metadata:"comment" description:"Table comment"`
}

// PrestoColumnFields describes the per-column fields embedded in an
// asset's schema.
type PrestoColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Presto data type, nested types verbatim"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	OrdinalPosition   int    `json:"ordinal_position" metadata:"ordinal_position" description:"Column position"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value expression"`
}
