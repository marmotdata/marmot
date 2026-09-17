package superset

// SupersetDashboardFields describes the metadata fields the plugin emits
// for Dashboard assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type SupersetDashboardFields struct {
	ID         int      `json:"id" metadata:"id" description:"Dashboard id in Superset"`
	Slug       string   `json:"slug" metadata:"slug" description:"URL slug"`
	URL        string   `json:"url" metadata:"url" description:"Link to the dashboard"`
	Published  bool     `json:"published" metadata:"published" description:"Whether the dashboard is published"`
	Status     string   `json:"status" metadata:"status" description:"Dashboard status (published, draft)"`
	Owners     []string `json:"owners" metadata:"owners" description:"Names of the owners"`
	Tags       []string `json:"tags" metadata:"tags" description:"Tags applied in Superset"`
	ChangedOn  string   `json:"changed_on" metadata:"changed_on" description:"Last change time (UTC)"`
	ChartCount int      `json:"chart_count" metadata:"chart_count" description:"Number of charts on the dashboard"`
}

// SupersetChartFields describes the metadata fields the plugin emits for
// Chart assets.
type SupersetChartFields struct {
	ID             int      `json:"id" metadata:"id" description:"Chart id in Superset"`
	VizType        string   `json:"viz_type" metadata:"viz_type" description:"Superset visualisation type"`
	ChartType      string   `json:"chart_type" metadata:"chart_type" description:"Normalised chart type (Table, Line, Bar, Pie, ...)"`
	DatasourceID   int      `json:"datasource_id" metadata:"datasource_id" description:"Id of the dataset the chart reads"`
	DatasourceType string   `json:"datasource_type" metadata:"datasource_type" description:"Kind of datasource (table)"`
	Dataset        string   `json:"dataset" metadata:"dataset" description:"Dataset the chart reads (schema.table)"`
	URL            string   `json:"url" metadata:"url" description:"Link to the chart"`
	Owners         []string `json:"owners" metadata:"owners" description:"Names of the owners"`
	Dashboards     []string `json:"dashboards" metadata:"dashboards" description:"Titles of the dashboards the chart is on"`
	ChangedOn      string   `json:"changed_on" metadata:"changed_on" description:"Last change time (UTC)"`
}

// SupersetDatasetFields describes the metadata fields the plugin emits
// for Data Model Object assets (Superset datasets).
type SupersetDatasetFields struct {
	ID          int      `json:"id" metadata:"id" description:"Dataset id in Superset"`
	Kind        string   `json:"kind" metadata:"kind" description:"physical (a table) or virtual (a SQL query)"`
	Database    string   `json:"database" metadata:"database" description:"Name of the Superset database connection"`
	DatabaseID  int      `json:"database_id" metadata:"database_id" description:"Id of the database connection"`
	Backend     string   `json:"backend" metadata:"backend" description:"SQLAlchemy dialect of the database (postgresql, snowflake, ...)"`
	Schema      string   `json:"schema" metadata:"schema" description:"Schema the dataset lives in"`
	TableName   string   `json:"table_name" metadata:"table_name" description:"Table name, or the virtual dataset's name"`
	URL         string   `json:"url" metadata:"url" description:"Link to the dataset"`
	Owners      []string `json:"owners" metadata:"owners" description:"Names of the owners"`
	Description string   `json:"description" metadata:"description" description:"Dataset description"`
	ChangedOn   string   `json:"changed_on" metadata:"changed_on" description:"Last change time (UTC)"`
}

// SupersetColumnFields describes the per-column fields embedded in a
// dataset asset's schema.
type SupersetColumnFields struct {
	ColumnName  string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType    string `json:"data_type" metadata:"data_type" description:"Column data type"`
	Description string `json:"description" metadata:"description" description:"Column description"`
	Expression  string `json:"expression" metadata:"expression" description:"SQL expression of a calculated column"`
}

// SupersetDatabaseFields describes the metadata fields the plugin emits
// for DataSource assets (Superset database connections).
type SupersetDatabaseFields struct {
	ID             int    `json:"id" metadata:"id" description:"Database id in Superset"`
	Backend        string `json:"backend" metadata:"backend" description:"SQLAlchemy dialect (postgresql, snowflake, ...)"`
	Driver         string `json:"driver" metadata:"driver" description:"SQLAlchemy driver"`
	Host           string `json:"host" metadata:"host" description:"Database host"`
	Port           string `json:"port" metadata:"port" description:"Database port"`
	Database       string `json:"database" metadata:"database" description:"Database or catalog the connection opens"`
	SQLAlchemyURI  string `json:"sqlalchemy_uri" metadata:"sqlalchemy_uri" description:"Connection URI with the password masked"`
	ExposeInSQLLab bool   `json:"expose_in_sqllab" metadata:"expose_in_sqllab" description:"Whether the database is available in SQL Lab"`
	AllowDML       bool   `json:"allow_dml" metadata:"allow_dml" description:"Whether SQL Lab may run DML against it"`
}
