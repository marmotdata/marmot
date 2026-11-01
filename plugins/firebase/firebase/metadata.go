package firebase

// FirebaseProjectFields describes the project-level metadata fields the
// plugin adds to every asset. They are kept as documentation-only structs so
// downstream tooling can introspect the shape of the metadata map.
type FirebaseProjectFields struct {
	ProjectID   string `json:"firebase_project_id" metadata:"firebase_project_id" description:"Firebase project ID"`
	ProjectNum  int64  `json:"firebase_project_number" metadata:"firebase_project_number" description:"Google-assigned project number"`
	DisplayName string `json:"firebase_project_display_name" metadata:"firebase_project_display_name" description:"Project display name"`
}

// FirestoreDatabaseFields describes the metadata fields the plugin emits for
// a Firestore database asset.
type FirestoreDatabaseFields struct {
	DatabaseID             string `json:"database_id" metadata:"database_id" description:"Firestore database ID, (default) for the default database"`
	DatabaseKind           string `json:"database_kind" metadata:"database_kind" description:"Which Firebase database this is: firestore or realtime"`
	LocationID             string `json:"location_id" metadata:"location_id" description:"Region the database runs in"`
	DatabaseType           string `json:"database_type" metadata:"database_type" description:"FIRESTORE_NATIVE or DATASTORE_MODE"`
	ConcurrencyMode        string `json:"concurrency_mode" metadata:"concurrency_mode" description:"Default transaction concurrency control mode"`
	PointInTimeRecovery    string `json:"point_in_time_recovery" metadata:"point_in_time_recovery" description:"Whether point in time recovery is enabled"`
	DeleteProtection       string `json:"delete_protection" metadata:"delete_protection" description:"Whether the database is protected from deletion"`
	VersionRetentionPeriod string `json:"version_retention_period" metadata:"version_retention_period" description:"How long past versions of the data are readable"`
	EarliestVersionTime    string `json:"earliest_version_time" metadata:"earliest_version_time" description:"Oldest timestamp a read can ask for"`
	CreateTime             string `json:"create_time" metadata:"create_time" description:"When the database was created"`
	UpdateTime             string `json:"update_time" metadata:"update_time" description:"When the database resource was last changed"`
	UID                    string `json:"uid" metadata:"uid" description:"System-generated database UUID"`
	FreeTier               bool   `json:"free_tier" metadata:"free_tier" description:"Whether the database is eligible for the free tier"`
}

// RealtimeDatabaseFields describes the metadata fields the plugin emits for a
// Realtime Database instance asset.
type RealtimeDatabaseFields struct {
	InstanceID   string `json:"instance_id" metadata:"instance_id" description:"Realtime Database instance ID"`
	DatabaseKind string `json:"database_kind" metadata:"database_kind" description:"Which Firebase database this is: firestore or realtime"`
	DatabaseURL  string `json:"database_url" metadata:"database_url" description:"Hostname the instance is served on"`
	InstanceType string `json:"instance_type" metadata:"instance_type" description:"DEFAULT_DATABASE or USER_DATABASE"`
	State        string `json:"state" metadata:"state" description:"Lifecycle state, for example ACTIVE or DISABLED"`
}

// FirestoreCollectionFields describes the metadata fields the plugin emits
// for a Firestore collection asset.
type FirestoreCollectionFields struct {
	DatabaseID        string `json:"database_id" metadata:"database_id" description:"Firestore database the collection lives in"`
	CollectionID      string `json:"collection_id" metadata:"collection_id" description:"Collection ID, the last segment of the path"`
	CollectionPath    string `json:"collection_path" metadata:"collection_path" description:"Collection path with parent document IDs left out"`
	Depth             int    `json:"depth" metadata:"depth" description:"Nesting level, 1 for a root collection"`
	SampledDocuments  int    `json:"sampled_documents" metadata:"sampled_documents" description:"How many documents were read to infer the fields"`
	CollectionGroupID string `json:"collection_group_id" metadata:"collection_group_id" description:"ID a collection group query uses to reach every copy of a subcollection"`
	ParentPath        string `json:"parent_path" metadata:"parent_path" description:"Path of the parent collection, for subcollections"`
}

// FirestoreColumnFields describes the per-field columns inferred from the
// sampled documents and stored in an asset's schema.
type FirestoreColumnFields struct {
	ColumnName string `json:"column_name" metadata:"column_name" description:"Document field name"`
	DataType   string `json:"data_type" metadata:"data_type" description:"Inferred Firestore type, or mixed when the sample disagreed"`
	IsNullable bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the field was absent or null in any sampled document"`
}
