package elasticsearch

// ElasticsearchIndexFields represents Elasticsearch index metadata fields
// +marmot:metadata
type ElasticsearchIndexFields struct {
	Cluster      string `json:"cluster" metadata:"cluster" description:"Name of the Elasticsearch cluster"`
	IndexName    string `json:"index_name" metadata:"index_name" description:"Name of the index"`
	Health       string `json:"health" metadata:"health" description:"Health status of the index (green, yellow, red)"`
	Status       string `json:"status" metadata:"status" description:"Open/close status of the index"`
	UUID         string `json:"uuid" metadata:"uuid" description:"UUID of the index"`
	Shards       int    `json:"shards" metadata:"shards" description:"Number of primary shards"`
	Replicas     int    `json:"replicas" metadata:"replicas" description:"Number of replica shards"`
	DocsCount    int64  `json:"docs_count" metadata:"docs_count" description:"Number of documents in the index"`
	StoreSize    string `json:"store_size" metadata:"store_size" description:"Total store size of the index"`
	CreationDate string `json:"creation_date" metadata:"creation_date" description:"Date and time when the index was created"`
}

// ElasticsearchFieldMapping represents a single field mapping in an Elasticsearch index
// +marmot:metadata
type ElasticsearchFieldMapping struct {
	FieldName string `json:"field_name" metadata:"field_name" description:"Full dotted path of the field"`
	FieldType string `json:"field_type" metadata:"field_type" description:"Elasticsearch field type (keyword, text, long, etc.)"`
	Index     string `json:"index" metadata:"index" description:"Whether the field is indexed"`
	Analyzer  string `json:"analyzer" metadata:"analyzer" description:"Analyzer used for the field"`
}

// ElasticsearchDataStreamFields represents Elasticsearch data stream metadata fields
// +marmot:metadata
type ElasticsearchDataStreamFields struct {
	DataStreamName string `json:"data_stream_name" metadata:"data_stream_name" description:"Name of the data stream"`
	TimestampField string `json:"timestamp_field" metadata:"timestamp_field" description:"Name of the timestamp field"`
	BackingIndices int    `json:"backing_indices" metadata:"backing_indices" description:"Number of backing indices"`
	Generation     int    `json:"generation" metadata:"generation" description:"Current generation of the data stream"`
	Status         string `json:"status" metadata:"status" description:"Health status of the data stream"`
	ILMPolicy      string `json:"ilm_policy" metadata:"ilm_policy" description:"ILM policy applied to the data stream"`
	Template       string `json:"template" metadata:"template" description:"Index template used by the data stream"`
}

// ElasticsearchAliasFields represents Elasticsearch alias metadata fields
// +marmot:metadata
type ElasticsearchAliasFields struct {
	AliasName     string `json:"alias_name" metadata:"alias_name" description:"Name of the alias"`
	Indices       string `json:"indices" metadata:"indices" description:"Comma-separated list of indices the alias points to"`
	IsWriteIndex  string `json:"is_write_index" metadata:"is_write_index" description:"Whether the alias has a designated write index"`
	FilterDefined string `json:"filter_defined" metadata:"filter_defined" description:"Whether a filter is defined on the alias"`
}
