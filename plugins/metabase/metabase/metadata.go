package metabase

// MetabaseDashboardFields describes the metadata fields the plugin emits
// for Dashboard assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type MetabaseDashboardFields struct {
	ID           int    `json:"id" metadata:"id" description:"Dashboard id in Metabase"`
	Collection   string `json:"collection" metadata:"collection" description:"Path of the collection holding the dashboard"`
	CollectionID int    `json:"collection_id" metadata:"collection_id" description:"Id of the collection holding the dashboard"`
	CreatorID    int    `json:"creator_id" metadata:"creator_id" description:"Id of the user who created the dashboard"`
	CreatedAt    string `json:"created_at" metadata:"created_at" description:"Creation timestamp"`
	UpdatedAt    string `json:"updated_at" metadata:"updated_at" description:"Last update timestamp"`
	CardCount    int    `json:"card_count" metadata:"card_count" description:"Number of cards placed on the dashboard"`
	Archived     bool   `json:"archived" metadata:"archived" description:"Whether the dashboard is archived"`
	URL          string `json:"url" metadata:"url" description:"Link to the dashboard in Metabase"`
}

// MetabaseCardFields describes the metadata fields the plugin emits for
// Chart and Data Model Object assets, which Metabase both calls cards.
type MetabaseCardFields struct {
	ID           int    `json:"id" metadata:"id" description:"Card id in Metabase"`
	Display      string `json:"display" metadata:"display" description:"Metabase visualisation, for example bar or scalar"`
	ChartType    string `json:"chart_type" metadata:"chart_type" description:"Normalised chart type (Table, Bar, Line, Pie, Area, Scatter, Map, Gauge, Text, Other)"`
	CardType     string `json:"card_type" metadata:"card_type" description:"Card kind: question, metric or model"`
	QueryType    string `json:"query_type" metadata:"query_type" description:"Whether the card runs SQL (native) or the query builder (query)"`
	Database     string `json:"database" metadata:"database" description:"Name of the Metabase database the card queries"`
	DatabaseID   int    `json:"database_id" metadata:"database_id" description:"Id of the Metabase database the card queries"`
	TableID      int    `json:"table_id" metadata:"table_id" description:"Id of the card's source table, for query builder cards"`
	Collection   string `json:"collection" metadata:"collection" description:"Path of the collection holding the card"`
	CollectionID int    `json:"collection_id" metadata:"collection_id" description:"Id of the collection holding the card"`
	CreatorID    int    `json:"creator_id" metadata:"creator_id" description:"Id of the user who created the card"`
	CreatedAt    string `json:"created_at" metadata:"created_at" description:"Creation timestamp"`
	UpdatedAt    string `json:"updated_at" metadata:"updated_at" description:"Last update timestamp"`
	Archived     bool   `json:"archived" metadata:"archived" description:"Whether the card is archived"`
	URL          string `json:"url" metadata:"url" description:"Link to the card in Metabase"`
}
