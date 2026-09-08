package redash

// The structs in this file document the metadata each Redash asset kind
// carries. They are never instantiated: discovery builds plain maps, and
// these describe the shape of those maps for the docs and for tooling that
// introspects a plugin's output.

// RedashDashboardFields describes the metadata on a Dashboard asset.
type RedashDashboardFields struct {
	ID            int      `json:"id" metadata:"id" description:"Redash dashboard id"`
	Slug          string   `json:"slug" metadata:"slug" description:"URL slug"`
	URL           string   `json:"url" metadata:"url" description:"Link to the dashboard in Redash"`
	Tags          []string `json:"tags" metadata:"tags" description:"Tags set on the dashboard in Redash"`
	IsDraft       bool     `json:"is_draft" metadata:"is_draft" description:"Whether the dashboard is unpublished"`
	IsArchived    bool     `json:"is_archived" metadata:"is_archived" description:"Whether the dashboard is archived"`
	Owner         string   `json:"owner" metadata:"owner" description:"Display name of the user who created the dashboard"`
	CreatedAt     string   `json:"created_at" metadata:"created_at" description:"Creation timestamp"`
	UpdatedAt     string   `json:"updated_at" metadata:"updated_at" description:"Last update timestamp"`
	WidgetCount   int      `json:"widget_count" metadata:"widget_count" description:"Number of widgets on the dashboard, text widgets included"`
	QueryCount    int      `json:"query_count" metadata:"query_count" description:"Number of distinct queries the dashboard reads"`
	Notes         string   `json:"notes" metadata:"notes" description:"Text widget content, joined with blank lines. Redash dashboards have no description field"`
	RedashVersion string   `json:"redash_version" metadata:"redash_version" description:"Redash release the dashboard was read from, which decides the URL shape"`
}

// RedashChartFields describes the metadata on a Chart asset.
type RedashChartFields struct {
	WidgetID          int    `json:"widget_id" metadata:"widget_id" description:"Redash widget id"`
	VisualizationID   int    `json:"visualization_id" metadata:"visualization_id" description:"Redash visualization id"`
	VisualizationType string `json:"visualization_type" metadata:"visualization_type" description:"Redash visualization type, for example CHART, TABLE or COUNTER"`
	ChartType         string `json:"chart_type" metadata:"chart_type" description:"Normalised chart shape, read from options.globalSeriesType for a CHART"`
	QueryID           int    `json:"query_id" metadata:"query_id" description:"Id of the query the chart renders"`
	QueryName         string `json:"query_name" metadata:"query_name" description:"Name of the query the chart renders"`
	DataSource        string `json:"data_source" metadata:"data_source" description:"Name of the data source the query runs against"`
	Dashboard         string `json:"dashboard" metadata:"dashboard" description:"Name of the dashboard the chart sits on"`
	URL               string `json:"url" metadata:"url" description:"Link to the visualization in Redash"`
	IsHidden          bool   `json:"is_hidden" metadata:"is_hidden" description:"Whether the widget is hidden on the dashboard"`
}

// RedashQueryFields describes the metadata on a Data Model Object asset,
// which is how a saved Redash query is catalogued.
type RedashQueryFields struct {
	ID           int      `json:"id" metadata:"id" description:"Redash query id"`
	Description  string   `json:"description" metadata:"description" description:"Query description"`
	DataSource   string   `json:"data_source" metadata:"data_source" description:"Name of the data source the query runs against"`
	DataSourceID int      `json:"data_source_id" metadata:"data_source_id" description:"Redash data source id"`
	Tags         []string `json:"tags" metadata:"tags" description:"Tags set on the query in Redash"`
	Owner        string   `json:"owner" metadata:"owner" description:"Display name of the user who created the query"`
	Schedule     string   `json:"schedule" metadata:"schedule" description:"Refresh schedule: interval in seconds, time, day of week and until date"`
	Runtime      float64  `json:"runtime" metadata:"runtime" description:"Seconds the last run took"`
	IsDraft      bool     `json:"is_draft" metadata:"is_draft" description:"Whether the query is unpublished"`
	IsArchived   bool     `json:"is_archived" metadata:"is_archived" description:"Whether the query is archived"`
	Parameters   string   `json:"parameters" metadata:"parameters" description:"Query parameters, each with a name, title and type"`
	CreatedAt    string   `json:"created_at" metadata:"created_at" description:"Creation timestamp"`
	UpdatedAt    string   `json:"updated_at" metadata:"updated_at" description:"Last update timestamp"`
	URL          string   `json:"url" metadata:"url" description:"Link to the query in Redash"`
}

// RedashDataSourceFields describes the metadata on a DataSource asset. The
// connection options are copied through an allowlist, so a password is never
// recorded even when the API returns one.
type RedashDataSourceFields struct {
	ID          int    `json:"id" metadata:"id" description:"Redash data source id"`
	Type        string `json:"type" metadata:"type" description:"Redash data source type, for example pg, mysql or snowflake"`
	Syntax      string `json:"syntax" metadata:"syntax" description:"Query syntax the data source accepts, for example sql or json"`
	ViewOnly    bool   `json:"view_only" metadata:"view_only" description:"Whether the data source is read only for the current user"`
	Paused      bool   `json:"paused" metadata:"paused" description:"Whether query execution is paused"`
	PauseReason string `json:"pause_reason" metadata:"pause_reason" description:"Why the data source is paused"`
	Host        string `json:"host" metadata:"host" description:"Connection host"`
	Port        int    `json:"port" metadata:"port" description:"Connection port"`
	DBName      string `json:"dbname" metadata:"dbname" description:"Database name"`
	Database    string `json:"database" metadata:"database" description:"Database name, for sources that call it that"`
	Catalog     string `json:"catalog" metadata:"catalog" description:"Catalog name, for Trino, Presto and Databricks"`
	Schema      string `json:"schema" metadata:"schema" description:"Default schema"`
	User        string `json:"user" metadata:"user" description:"Connection user"`
	Project     string `json:"project" metadata:"project" description:"Google Cloud project, for BigQuery"`
	Dataset     string `json:"dataset" metadata:"dataset" description:"Dataset name"`
	Account     string `json:"account" metadata:"account" description:"Account identifier, for Snowflake"`
	Warehouse   string `json:"warehouse" metadata:"warehouse" description:"Warehouse name, for Snowflake"`
	Region      string `json:"region" metadata:"region" description:"Cloud region"`
	URL         string `json:"url" metadata:"url" description:"Connection URL, for sources configured with one"`
}
