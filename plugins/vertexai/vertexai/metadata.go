package vertexai

// Every Vertex AI asset also carries resource_id, location, project_id,
// create_time, update_time and one label_<key> entry per resource label.

// VertexAICommonFields describes the metadata every Vertex AI asset
// carries, whatever its kind.
// +marmot:metadata
type VertexAICommonFields struct {
	ResourceID string `json:"resource_id" metadata:"resource_id" description:"Vertex AI id of the resource, unique within its project and location"`
	Location   string `json:"location" metadata:"location" description:"Region the resource lives in"`
	ProjectID  string `json:"project_id" metadata:"project_id" description:"Google Cloud project the resource belongs to"`
	CreateTime string `json:"create_time" metadata:"create_time" description:"When the resource was created"`
	UpdateTime string `json:"update_time" metadata:"update_time" description:"When the resource was last updated"`
}

// VertexAIModelFields describes the metadata a Model asset carries.
// +marmot:metadata
type VertexAIModelFields struct {
	DisplayName                       string `json:"display_name" metadata:"display_name" description:"Display name of the model, which Vertex AI does not require to be unique"`
	VersionID                         string `json:"version_id" metadata:"version_id" description:"Version of the model this entry describes"`
	VersionAliases                    string `json:"version_aliases" metadata:"version_aliases" description:"Aliases the version can be referenced by"`
	VersionDescription                string `json:"version_description" metadata:"version_description" description:"Description of this version"`
	VersionCreateTime                 string `json:"version_create_time" metadata:"version_create_time" description:"When this version was created"`
	ArtifactURI                       string `json:"artifact_uri" metadata:"artifact_uri" description:"Cloud Storage directory holding the model artifact"`
	MetadataSchemaURI                 string `json:"metadata_schema_uri" metadata:"metadata_schema_uri" description:"Schema describing the model's additional metadata"`
	ContainerImage                    string `json:"container_image" metadata:"container_image" description:"Container image the model is served from"`
	PredictSchemataInstance           string `json:"predict_schemata_instance" metadata:"predict_schemata_instance" description:"Schema of a single prediction instance"`
	PredictSchemataParameters         string `json:"predict_schemata_parameters" metadata:"predict_schemata_parameters" description:"Schema of the prediction parameters"`
	PredictSchemataPrediction         string `json:"predict_schemata_prediction" metadata:"predict_schemata_prediction" description:"Schema of a single prediction"`
	SupportedDeploymentResourcesTypes string `json:"supported_deployment_resources_types" metadata:"supported_deployment_resources_types" description:"Resource types the model can be deployed with"`
	SupportedInputStorageFormats      string `json:"supported_input_storage_formats" metadata:"supported_input_storage_formats" description:"Input formats the model accepts for batch prediction"`
	SupportedOutputStorageFormats     string `json:"supported_output_storage_formats" metadata:"supported_output_storage_formats" description:"Output formats the model writes for batch prediction"`
	TrainingPipeline                  string `json:"training_pipeline" metadata:"training_pipeline" description:"Id of the training pipeline that uploaded the model"`
	PipelineJob                       string `json:"pipeline_job" metadata:"pipeline_job" description:"Id of the pipeline job that produced the model"`
	BaseModelSource                   string `json:"base_model_source" metadata:"base_model_source" description:"Model Garden name or Genie URI of the model this one is derived from"`
	DeployedModelCount                int    `json:"deployed_model_count" metadata:"deployed_model_count" description:"Number of endpoints the model is deployed to"`
}

// VertexAIEndpointFields describes the metadata an Endpoint asset carries.
// +marmot:metadata
type VertexAIEndpointFields struct {
	DisplayName                  string `json:"display_name" metadata:"display_name" description:"Display name of the endpoint, which Vertex AI does not require to be unique"`
	Network                      string `json:"network" metadata:"network" description:"VPC network the endpoint is peered with"`
	DedicatedEndpointEnabled     bool   `json:"dedicated_endpoint_enabled" metadata:"dedicated_endpoint_enabled" description:"Set when the endpoint has a dedicated DNS name, absent otherwise"`
	DedicatedEndpointDNS         string `json:"dedicated_endpoint_dns" metadata:"dedicated_endpoint_dns" description:"DNS name of the dedicated endpoint"`
	TrafficSplit                 string `json:"traffic_split" metadata:"traffic_split" description:"How traffic is shared between deployed models, as id=percent pairs"`
	DeployedModelCount           int    `json:"deployed_model_count" metadata:"deployed_model_count" description:"Number of models deployed to the endpoint"`
	DeployedModels               string `json:"deployed_models" metadata:"deployed_models" description:"Display names of the models deployed to the endpoint"`
	ModelDeploymentMonitoringJob string `json:"model_deployment_monitoring_job" metadata:"model_deployment_monitoring_job" description:"Id of the monitoring job watching the endpoint"`
}

// VertexAIDatasetFields describes the metadata a managed dataset carries.
// Feature groups are Datasets too, and carry the feature group fields
// below instead.
// +marmot:metadata
type VertexAIDatasetFields struct {
	DisplayName       string `json:"display_name" metadata:"display_name" description:"Display name of the dataset, which Vertex AI does not require to be unique"`
	MetadataSchemaURI string `json:"metadata_schema_uri" metadata:"metadata_schema_uri" description:"Schema the dataset's metadata follows"`
	DataItemCount     int64  `json:"data_item_count" metadata:"data_item_count" description:"Number of data items in the dataset"`
	DatasetKind       string `json:"dataset_kind" metadata:"dataset_kind" description:"Kind of dataset, read from the metadata schema URI, for example image_1.0.0"`
	SavedQueryCount   int    `json:"saved_query_count" metadata:"saved_query_count" description:"Number of saved queries defined on the dataset"`
	ModelReference    string `json:"model_reference" metadata:"model_reference" description:"Model the dataset was created for"`
	SourceURIs        string `json:"source_uris" metadata:"source_uris" description:"BigQuery and Cloud Storage URIs the dataset reads from"`
}

// VertexAIFeatureGroupFields describes the metadata a feature group
// Dataset asset carries.
// +marmot:metadata
type VertexAIFeatureGroupFields struct {
	DisplayName         string `json:"display_name" metadata:"display_name" description:"Id of the feature group, which is what Vertex AI shows as its name"`
	FeatureGroupID      string `json:"feature_group_id" metadata:"feature_group_id" description:"Id of the feature group"`
	EntityIDColumns     string `json:"entity_id_columns" metadata:"entity_id_columns" description:"Columns of the source table that identify an entity"`
	BigQuerySourceURI   string `json:"big_query_source_uri" metadata:"big_query_source_uri" description:"BigQuery table the features are read from"`
	Dense               bool   `json:"dense" metadata:"dense" description:"Set when every feature is written on every row, absent otherwise"`
	StaticDataSource    bool   `json:"static_data_source" metadata:"static_data_source" description:"Set when the source table does not change, absent otherwise"`
	ServiceAccountEmail string `json:"service_account_email" metadata:"service_account_email" description:"Service account the feature group reads its source with"`
	FeatureCount        int    `json:"feature_count" metadata:"feature_count" description:"Number of features in the group"`
}

// VertexAIFeatureFields describes the per-feature fields embedded in a
// feature group's schema.
type VertexAIFeatureFields struct {
	ColumnName  string `json:"column_name" metadata:"column_name" description:"Feature id"`
	DataType    string `json:"data_type" metadata:"data_type" description:"Feature value type, lowercased, for example string or int64"`
	IsNullable  bool   `json:"is_nullable" metadata:"is_nullable" description:"Always true: Vertex AI does not record which features are mandatory"`
	Description string `json:"description" metadata:"description" description:"Description of the feature"`
}

// VertexAIPipelineJobFields describes the metadata a pipeline Job asset
// carries.
// +marmot:metadata
type VertexAIPipelineJobFields struct {
	DisplayName    string `json:"display_name" metadata:"display_name" description:"Display name of the pipeline job, which Vertex AI does not require to be unique"`
	State          string `json:"state" metadata:"state" description:"Pipeline state, for example PIPELINE_STATE_SUCCEEDED"`
	StartTime      string `json:"start_time" metadata:"start_time" description:"When the pipeline started running"`
	EndTime        string `json:"end_time" metadata:"end_time" description:"When the pipeline finished"`
	ScheduleName   string `json:"schedule_name" metadata:"schedule_name" description:"Schedule that created the run"`
	TemplateURI    string `json:"template_uri" metadata:"template_uri" description:"Location of the pipeline template the run was compiled from"`
	ServiceAccount string `json:"service_account" metadata:"service_account" description:"Service account the pipeline ran as"`
	ErrorMessage   string `json:"error_message" metadata:"error_message" description:"Why the pipeline failed"`
}
