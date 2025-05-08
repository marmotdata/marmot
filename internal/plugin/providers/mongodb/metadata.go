package mongodb

// MongoDBFields represents MongoDB-specific metadata fields
// +marmot:metadata
type MongoDBFields struct {
	Host             string `json:"host" metadata:"host" description:"MongoDB server hostname"`
	Port             int    `json:"port" metadata:"port" description:"MongoDB server port"`
	Database         string `json:"database" metadata:"database" description:"Database name"`
	Collection       string `json:"collection" metadata:"collection" description:"Collection name"`
	ObjectType       string `json:"object_type" metadata:"object_type" description:"Object type (collection, view)"`
	Size             int64  `json:"size" metadata:"size" description:"Collection size in bytes"`
	DocumentCount    int64  `json:"document_count" metadata:"document_count" description:"Approximate document count"`
	Created          string `json:"created" metadata:"created" description:"Creation timestamp if available"`
	Capped           bool   `json:"capped" metadata:"capped" description:"Whether the collection is capped"`
	MaxSize          int64  `json:"max_size" metadata:"max_size" description:"Maximum size for capped collections"`
	MaxDocuments     int64  `json:"max_documents" metadata:"max_documents" description:"Maximum document count for capped collections"`
	StorageEngine    string `json:"storage_engine" metadata:"storage_engine" description:"Storage engine used"`
	IndexCount       int    `json:"index_count" metadata:"index_count" description:"Number of indexes on collection"`
	ShardingEnabled  bool   `json:"sharding_enabled" metadata:"sharding_enabled" description:"Whether sharding is enabled"`
	ShardKey         string `json:"shard_key" metadata:"shard_key" description:"Shard key if collection is sharded"`
	Replicated       bool   `json:"replicated" metadata:"replicated" description:"Whether collection is replicated"`
	ValidationLevel  string `json:"validation_level" metadata:"validation_level" description:"Validation level if schema validation is enabled"`
	ValidationAction string `json:"validation_action" metadata:"validation_action" description:"Validation action if schema validation is enabled"`
}

// MongoDBIndexFields represents MongoDB index metadata
// +marmot:metadata
type MongoDBIndexFields struct {
	Name          string `json:"name" metadata:"name" description:"Index name"`
	Fields        string `json:"fields" metadata:"fields" description:"Fields included in the index"`
	Unique        bool   `json:"unique" metadata:"unique" description:"Whether the index enforces uniqueness"`
	Sparse        bool   `json:"sparse" metadata:"sparse" description:"Whether the index is sparse"`
	Background    bool   `json:"background" metadata:"background" description:"Whether the index was built in the background"`
	TTL           int    `json:"ttl" metadata:"ttl" description:"Time-to-live in seconds if TTL index"`
	Partial       bool   `json:"partial" metadata:"partial" description:"Whether the index is partial"`
	PartialFilter string `json:"partial_filter" metadata:"partial_filter" description:"Filter expression for partial indexes"`
	Type          string `json:"type" metadata:"type" description:"Index type (e.g., single field, compound, text, geo)"`
}

// MongoDBSchemaFields represents MongoDB schema metadata from schema sampling
// +marmot:metadata
type MongoDBSchemaFields struct {
	FieldName    string   `json:"field_name" metadata:"field_name" description:"Field name"`
	DataTypes    []string `json:"data_types" metadata:"data_types" description:"Observed data types"`
	IsRequired   bool     `json:"is_required" metadata:"is_required" description:"Whether field appears in all documents"`
	Frequency    float64  `json:"frequency" metadata:"frequency" description:"Frequency of field occurrence in documents"`
	SampleValues string   `json:"sample_values" metadata:"sample_values" description:"Sample values from documents"`
	Description  string   `json:"description" metadata:"description" description:"Field description from validation schema if available"`
}

