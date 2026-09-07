package mariadb

// MariaDBDatabaseFields describes the metadata fields the plugin emits for
// the database asset. It is a documentation-only struct so downstream
// tooling can introspect the shape of the metadata map.
type MariaDBDatabaseFields struct {
	Host          string `json:"host" metadata:"host" description:"MariaDB server hostname"`
	Port          int    `json:"port" metadata:"port" description:"MariaDB server port"`
	Database      string `json:"database" metadata:"database" description:"Database name"`
	ServerVersion string `json:"server_version" metadata:"server_version" description:"MariaDB server version"`
	CharacterSet  string `json:"character_set" metadata:"character_set" description:"Default character set of the database"`
	Collation     string `json:"collation" metadata:"collation" description:"Default collation of the database"`
}

// MariaDBFields describes the metadata fields the plugin emits for table,
// view and sequence assets.
type MariaDBFields struct {
	Host            string `json:"host" metadata:"host" description:"MariaDB server hostname"`
	Port            int    `json:"port" metadata:"port" description:"MariaDB server port"`
	Database        string `json:"database" metadata:"database" description:"Database name"`
	Schema          string `json:"schema" metadata:"schema" description:"Schema name (same as the database in MariaDB)"`
	TableName       string `json:"table_name" metadata:"table_name" description:"Object name"`
	ObjectType      string `json:"object_type" metadata:"object_type" description:"Object type (table, view, sequence)"`
	Engine          string `json:"engine" metadata:"engine" description:"Storage engine"`
	Collation       string `json:"collation" metadata:"collation" description:"Table collation"`
	RowCount        int64  `json:"row_count" metadata:"row_count" description:"Approximate row count"`
	DataLength      int64  `json:"data_length" metadata:"data_length" description:"Data size in bytes"`
	IndexLength     int64  `json:"index_length" metadata:"index_length" description:"Index size in bytes"`
	AutoIncrement   int64  `json:"auto_increment" metadata:"auto_increment" description:"Next auto increment value"`
	Created         string `json:"created" metadata:"created" description:"Creation timestamp"`
	Updated         string `json:"updated" metadata:"updated" description:"Last update timestamp"`
	Comment         string `json:"comment" metadata:"comment" description:"Table comment"`
	SystemVersioned bool   `json:"system_versioned" metadata:"system_versioned" description:"Whether the table keeps row history (WITH SYSTEM VERSIONING)"`
	Temporary       bool   `json:"temporary" metadata:"temporary" description:"Whether the table is temporary (only when the server exposes it)"`
}

// MariaDBViewFields describes the extra metadata fields emitted for views.
type MariaDBViewFields struct {
	CheckOption  string `json:"check_option" metadata:"check_option" description:"WITH CHECK OPTION setting (NONE, LOCAL, CASCADED)"`
	IsUpdatable  bool   `json:"is_updatable" metadata:"is_updatable" description:"Whether the view accepts writes"`
	Definer      string `json:"definer" metadata:"definer" description:"Account that defined the view"`
	SecurityType string `json:"security_type" metadata:"security_type" description:"SQL SECURITY setting (DEFINER, INVOKER)"`
}

// MariaDBSequenceFields describes the extra metadata fields emitted for
// sequences.
type MariaDBSequenceFields struct {
	StartValue   int64 `json:"start_value" metadata:"start_value" description:"Value the sequence started at"`
	MinimumValue int64 `json:"minimum_value" metadata:"minimum_value" description:"Smallest value the sequence produces"`
	MaximumValue int64 `json:"maximum_value" metadata:"maximum_value" description:"Largest value the sequence produces"`
	Increment    int64 `json:"increment" metadata:"increment" description:"Step between values"`
	CacheSize    int64 `json:"cache_size" metadata:"cache_size" description:"Number of values cached per fetch"`
	CycleOption  bool  `json:"cycle_option" metadata:"cycle_option" description:"Whether the sequence restarts after reaching its limit"`
}

// MariaDBColumnFields describes the per-column fields embedded in an
// asset's schema.
type MariaDBColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Full column type, for example varchar(255) or int(11)"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value or expression"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
	IsAutoIncrement   bool   `json:"is_auto_increment" metadata:"is_auto_increment" description:"Whether the column auto-increments"`
	IsGenerated       bool   `json:"is_generated" metadata:"is_generated" description:"Whether the column is computed from an expression"`
	IsInvisible       bool   `json:"is_invisible" metadata:"is_invisible" description:"Whether the column is hidden from SELECT *"`
	CharacterSet      string `json:"character_set" metadata:"character_set" description:"Character set"`
	Collation         string `json:"collation" metadata:"collation" description:"Collation"`
}
