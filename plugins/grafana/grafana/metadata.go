package grafana

// GrafanaDashboardFields describes the metadata fields the Grafana
// plugin emits for Dashboard assets. It is kept as a documentation-only
// struct so downstream tooling can introspect the shape of the metadata
// map.
type GrafanaDashboardFields struct {
	UID           string   `json:"uid" metadata:"uid" description:"Dashboard uid"`
	ID            int64    `json:"id" metadata:"id" description:"Dashboard numeric id"`
	URL           string   `json:"url" metadata:"url" description:"Dashboard URL in Grafana"`
	Folder        string   `json:"folder" metadata:"folder" description:"Title of the folder holding the dashboard"`
	FolderUID     string   `json:"folder_uid" metadata:"folder_uid" description:"Uid of the folder holding the dashboard"`
	Tags          []string `json:"tags" metadata:"tags" description:"Tags set on the dashboard in Grafana"`
	Version       int      `json:"version" metadata:"version" description:"Dashboard version number"`
	SchemaVersion int      `json:"schema_version" metadata:"schema_version" description:"Dashboard JSON schema version"`
	Created       string   `json:"created" metadata:"created" description:"When the dashboard was created"`
	Updated       string   `json:"updated" metadata:"updated" description:"When the dashboard was last saved"`
	CreatedBy     string   `json:"created_by" metadata:"created_by" description:"User who created the dashboard"`
	UpdatedBy     string   `json:"updated_by" metadata:"updated_by" description:"User who last saved the dashboard"`
	Provisioned   bool     `json:"provisioned" metadata:"provisioned" description:"Whether the dashboard is managed by provisioning"`
	Refresh       string   `json:"refresh" metadata:"refresh" description:"Auto refresh interval"`
	TimeFrom      string   `json:"time_from" metadata:"time_from" description:"Start of the default time range"`
	TimeTo        string   `json:"time_to" metadata:"time_to" description:"End of the default time range"`
	PanelCount    int      `json:"panel_count" metadata:"panel_count" description:"Number of panels, not counting rows and text panels"`
}

// GrafanaChartFields describes the metadata fields emitted for Chart
// assets, one per dashboard panel.
type GrafanaChartFields struct {
	PanelID        int64  `json:"panel_id" metadata:"panel_id" description:"Panel id within the dashboard"`
	PanelType      string `json:"panel_type" metadata:"panel_type" description:"Grafana panel type, for example timeseries"`
	ChartType      string `json:"chart_type" metadata:"chart_type" description:"Normalised chart type (Line, Table, Bar, ...)"`
	Dashboard      string `json:"dashboard" metadata:"dashboard" description:"Title of the dashboard holding the panel"`
	DashboardUID   string `json:"dashboard_uid" metadata:"dashboard_uid" description:"Uid of the dashboard holding the panel"`
	Datasource     string `json:"datasource" metadata:"datasource" description:"Name of the data source the panel queries"`
	DatasourceType string `json:"datasource_type" metadata:"datasource_type" description:"Type of the data source the panel queries"`
	DatasourceUID  string `json:"datasource_uid" metadata:"datasource_uid" description:"Uid of the data source the panel queries"`
	URL            string `json:"url" metadata:"url" description:"URL of the panel in Grafana"`
	TargetCount    int    `json:"target_count" metadata:"target_count" description:"Number of queries the panel runs"`
}

// GrafanaDatasourceFields describes the metadata fields emitted for
// DataSource assets. Credentials are never read.
type GrafanaDatasourceFields struct {
	UID       string `json:"uid" metadata:"uid" description:"Data source uid"`
	ID        int64  `json:"id" metadata:"id" description:"Data source numeric id"`
	Type      string `json:"type" metadata:"type" description:"Data source plugin type, for example grafana-postgresql-datasource"`
	TypeName  string `json:"type_name" metadata:"type_name" description:"Data source plugin display name"`
	URL       string `json:"url" metadata:"url" description:"Address the data source connects to"`
	Database  string `json:"database" metadata:"database" description:"Database the data source connects to"`
	IsDefault bool   `json:"is_default" metadata:"is_default" description:"Whether this is the default data source"`
	ReadOnly  bool   `json:"read_only" metadata:"read_only" description:"Whether the data source is read-only in Grafana"`
	Access    string `json:"access" metadata:"access" description:"Access mode (proxy or direct)"`
}
