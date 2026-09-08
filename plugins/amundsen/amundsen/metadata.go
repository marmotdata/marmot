package amundsen

// AmundsenTableFields describes the metadata fields the plugin emits on
// a table or view asset. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type AmundsenTableFields struct {
	Key                      string   `json:"key" metadata:"amundsen.key" description:"Amundsen node key, for example postgres://prod.public/orders"`
	Database                 string   `json:"database" metadata:"amundsen.database" description:"Amundsen database, which holds the technology name rather than a database"`
	Cluster                  string   `json:"cluster" metadata:"amundsen.cluster" description:"Amundsen cluster, usually an environment label such as prod or gold"`
	Schema                   string   `json:"schema" metadata:"amundsen.schema" description:"Schema holding the table"`
	Table                    string   `json:"table" metadata:"amundsen.table" description:"Table name as Amundsen records it"`
	Badges                   []string `json:"badges" metadata:"amundsen.badges" description:"Badges applied to the table in Amundsen"`
	Tags                     []string `json:"tags" metadata:"amundsen.tags" description:"Amundsen tags of type default"`
	ProgrammaticDescriptions []string `json:"programmatic_descriptions" metadata:"amundsen.programmatic_descriptions" description:"Descriptions written by an automated source rather than by a person"`
	LastUpdatedAt            string   `json:"last_updated_at" metadata:"amundsen.last_updated_at" description:"When the table last changed, as Amundsen recorded it"`
	URL                      string   `json:"url" metadata:"amundsen.url" description:"Table page in the Amundsen web app"`
	SchemaDescription        string   `json:"schema_description" metadata:"schema_description" description:"Description of the schema holding the table"`
	Owners                   []string `json:"owners" metadata:"owners" description:"Owner display names"`
	OwnerEmails              []string `json:"owner_emails" metadata:"owner_emails" description:"Owner email addresses"`
	OwnerTeams               []string `json:"owner_teams" metadata:"owner_teams" description:"Teams the owners belong to"`
}

// AmundsenColumnFields describes the per-column fields embedded in a
// table asset's schema.
type AmundsenColumnFields struct {
	ColumnName  string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType    string `json:"data_type" metadata:"data_type" description:"Column type as the source system reports it"`
	IsNullable  bool   `json:"is_nullable" metadata:"is_nullable" description:"Always true: Amundsen does not record nullability"`
	Description string `json:"description" metadata:"description" description:"Column description"`
}

// AmundsenDashboardFields describes the metadata fields the plugin emits
// on a dashboard asset.
type AmundsenDashboardFields struct {
	Key               string   `json:"key" metadata:"amundsen.key" description:"Amundsen node key, for example superset_dashboard://prod.finance/revenue"`
	Cluster           string   `json:"cluster" metadata:"amundsen.cluster" description:"Amundsen cluster the dashboard group sits under"`
	Group             string   `json:"group" metadata:"amundsen.group" description:"Dashboard group name"`
	GroupURL          string   `json:"group_url" metadata:"amundsen.group_url" description:"Dashboard group address in the BI tool"`
	GroupDescription  string   `json:"group_description" metadata:"amundsen.group_description" description:"Dashboard group description"`
	Product           string   `json:"product" metadata:"amundsen.product" description:"BI tool the dashboard belongs to"`
	URL               string   `json:"url" metadata:"amundsen.url" description:"Dashboard address in the BI tool"`
	AmundsenURL       string   `json:"amundsen_url" metadata:"amundsen.amundsen_url" description:"Dashboard page in the Amundsen web app"`
	QueryNames        []string `json:"query_names" metadata:"amundsen.query_names" description:"Names of the queries feeding the dashboard"`
	Tags              []string `json:"tags" metadata:"amundsen.tags" description:"Amundsen tags of type default"`
	Badges            []string `json:"badges" metadata:"amundsen.badges" description:"Badges applied to the dashboard in Amundsen"`
	LastSuccessfulRun string   `json:"last_successful_run" metadata:"amundsen.last_successful_run" description:"When the dashboard last refreshed successfully"`
	ChartCount        int      `json:"chart_count" metadata:"chart_count" description:"Number of charts on the dashboard"`
}

// AmundsenChartFields describes the metadata fields the plugin emits on
// a chart asset.
type AmundsenChartFields struct {
	DashboardKey string `json:"dashboard_key" metadata:"amundsen.dashboard_key" description:"Amundsen key of the dashboard holding the chart"`
	Group        string `json:"group" metadata:"amundsen.group" description:"Dashboard group name"`
	Dashboard    string `json:"dashboard" metadata:"amundsen.dashboard" description:"Dashboard name"`
	ChartID      string `json:"chart_id" metadata:"amundsen.chart_id" description:"Chart id in the BI tool"`
	URL          string `json:"url" metadata:"amundsen.url" description:"Chart address in the BI tool"`
	Product      string `json:"product" metadata:"amundsen.product" description:"BI tool the chart belongs to"`
	ChartType    string `json:"chart_type" metadata:"chart_type" description:"Chart type, for example bar or line"`
}
