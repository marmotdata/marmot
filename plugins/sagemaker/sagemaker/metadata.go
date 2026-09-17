package sagemaker

// SageMakerModelFields describes the metadata a Model asset carries. Both a
// deployed model and a model registry group are filed as Models, told apart
// by the kind field.
// +marmot:metadata
type SageMakerModelFields struct {
	Kind                 string `json:"kind" metadata:"kind" description:"Which kind of model this is (model, model_package_group)"`
	Arn                  string `json:"arn" metadata:"arn" description:"The ARN of the model or model package group"`
	Image                string `json:"image" metadata:"image" description:"Container image the model is served from"`
	ModelDataURL         string `json:"model_data_url" metadata:"model_data_url" description:"S3 location of the model artifact"`
	Mode                 string `json:"mode" metadata:"mode" description:"Container mode (SingleModel or MultiModel)"`
	Environment          string `json:"environment" metadata:"environment" description:"Container environment variables, with credential-looking values masked"`
	Containers           string `json:"containers" metadata:"containers" description:"Containers of an inference pipeline, each with its image, model_data_url and mode"`
	ExecutionRoleArn     string `json:"execution_role_arn" metadata:"execution_role_arn" description:"IAM role the model runs under"`
	NetworkIsolation     bool   `json:"network_isolation" metadata:"network_isolation" description:"Whether the model container runs without network access"`
	VpcSubnetCount       int    `json:"vpc_subnet_count" metadata:"vpc_subnet_count" description:"Number of VPC subnets the model is attached to"`
	Description          string `json:"description" metadata:"description" description:"Description of the model package group"`
	Status               string `json:"status" metadata:"status" description:"Status of the model package group"`
	VersionCount         int    `json:"version_count" metadata:"version_count" description:"Number of versions in the model package group"`
	LatestVersion        int32  `json:"latest_version" metadata:"latest_version" description:"Newest version number in the model package group"`
	LatestApprovalStatus string `json:"latest_approval_status" metadata:"latest_approval_status" description:"Approval status of the newest version"`
	LatestImage          string `json:"latest_image" metadata:"latest_image" description:"Container image of the newest approved version"`
	LatestModelDataURL   string `json:"latest_model_data_url" metadata:"latest_model_data_url" description:"S3 artifact of the newest approved version"`
	Domain               string `json:"domain" metadata:"domain" description:"Machine learning domain the model package belongs to"`
	Task                 string `json:"task" metadata:"task" description:"Machine learning task the model package performs"`
	SamplePayloadURL     string `json:"sample_payload_url" metadata:"sample_payload_url" description:"S3 location of a sample inference payload"`
	SupportedContentType string `json:"supported_content_types" metadata:"supported_content_types" description:"Content types the model package accepts"`
	SupportedMIMETypes   string `json:"supported_response_mime_types" metadata:"supported_response_mime_types" description:"Response MIME types the model package returns"`
	ModelQualityURI      string `json:"model_quality_statistics_s3_uri" metadata:"model_quality_statistics_s3_uri" description:"S3 location of the model quality statistics report"`
	CreatedAt            string `json:"created_at" metadata:"created_at" description:"When the model or group was created"`
	Region               string `json:"region" metadata:"region" description:"AWS region the resource lives in"`
	URL                  string `json:"url" metadata:"url" description:"Link to the model in the AWS console. Not set for a model package group"`
}

// SageMakerEndpointFields describes the metadata an Endpoint asset carries.
// +marmot:metadata
type SageMakerEndpointFields struct {
	Arn              string `json:"arn" metadata:"arn" description:"The ARN of the endpoint"`
	Status           string `json:"status" metadata:"status" description:"Endpoint status (InService, Creating, Failed)"`
	EndpointConfig   string `json:"endpoint_config" metadata:"endpoint_config" description:"Name of the endpoint configuration in use"`
	Variants         string `json:"variants" metadata:"variants" description:"Production variants, each with its name, model, instance_type, instance_count, weight and serverless flag"`
	DataCaptureS3URI string `json:"data_capture_s3_uri" metadata:"data_capture_s3_uri" description:"S3 location captured requests and responses are written to"`
	KMSKeyID         string `json:"kms_key_id" metadata:"kms_key_id" description:"KMS key the endpoint storage volume is encrypted with"`
	CreatedAt        string `json:"created_at" metadata:"created_at" description:"When the endpoint was created"`
	LastModifiedAt   string `json:"last_modified_at" metadata:"last_modified_at" description:"When the endpoint was last modified"`
	Region           string `json:"region" metadata:"region" description:"AWS region the endpoint lives in"`
	URL              string `json:"url" metadata:"url" description:"Link to the endpoint in the AWS console"`
}

// SageMakerFeatureGroupFields describes the metadata a feature group Dataset
// asset carries.
// +marmot:metadata
type SageMakerFeatureGroupFields struct {
	Arn               string `json:"arn" metadata:"arn" description:"The ARN of the feature group"`
	RecordIdentifier  string `json:"record_identifier" metadata:"record_identifier" description:"Feature that identifies a record"`
	EventTimeFeature  string `json:"event_time_feature" metadata:"event_time_feature" description:"Feature holding the event timestamp"`
	OnlineStore       bool   `json:"online_store" metadata:"online_store" description:"Whether the online store is enabled"`
	OfflineStoreS3URI string `json:"offline_store_s3_uri" metadata:"offline_store_s3_uri" description:"S3 location of the offline store"`
	GlueTable         string `json:"glue_table" metadata:"glue_table" description:"Glue table the offline store is queryable through, as database.table"`
	Status            string `json:"status" metadata:"status" description:"Feature group status"`
	Description       string `json:"description" metadata:"description" description:"Description of the feature group"`
	CreatedAt         string `json:"created_at" metadata:"created_at" description:"When the feature group was created"`
	Region            string `json:"region" metadata:"region" description:"AWS region the feature group lives in"`
}

// SageMakerFeatureFields describes the per-feature fields embedded in a
// feature group's schema.
type SageMakerFeatureFields struct {
	ColumnName   string `json:"column_name" metadata:"column_name" description:"Feature name"`
	DataType     string `json:"data_type" metadata:"data_type" description:"Feature type (String, Integral, Fractional)"`
	IsNullable   bool   `json:"is_nullable" metadata:"is_nullable" description:"False for the record identifier and event time features, which every record must carry"`
	IsPrimaryKey bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the feature is the record identifier"`
}

// SageMakerTrainingJobFields describes the metadata a training Job asset
// carries.
// +marmot:metadata
type SageMakerTrainingJobFields struct {
	Arn                 string `json:"arn" metadata:"arn" description:"The ARN of the training job"`
	Status              string `json:"status" metadata:"status" description:"Training job status (InProgress, Completed, Failed, Stopped)"`
	FailureReason       string `json:"failure_reason" metadata:"failure_reason" description:"Why the training job failed"`
	AlgorithmImage      string `json:"algorithm_image" metadata:"algorithm_image" description:"Container image the job trained with"`
	AlgorithmName       string `json:"algorithm_name" metadata:"algorithm_name" description:"Marketplace algorithm the job trained with"`
	HyperParameters     string `json:"hyperparameters" metadata:"hyperparameters" description:"Hyperparameters the job ran with"`
	InstanceType        string `json:"instance_type" metadata:"instance_type" description:"Instance type the job ran on"`
	InstanceCount       int32  `json:"instance_count" metadata:"instance_count" description:"Number of instances the job ran on"`
	InputChannels       string `json:"input_channels" metadata:"input_channels" description:"Training channels, each mapping a channel name to its S3 location"`
	OutputS3Path        string `json:"output_s3_path" metadata:"output_s3_path" description:"S3 prefix the job wrote its output to"`
	ModelArtifactsS3URI string `json:"model_artifacts_s3_uri" metadata:"model_artifacts_s3_uri" description:"S3 location of the model artifact the job produced"`
	FinalMetrics        string `json:"final_metrics" metadata:"final_metrics" description:"Last value the job reported for each metric"`
	StartedAt           string `json:"started_at" metadata:"started_at" description:"When training started"`
	EndedAt             string `json:"ended_at" metadata:"ended_at" description:"When training ended"`
	CreatedAt           string `json:"created_at" metadata:"created_at" description:"When the training job was created"`
	Region              string `json:"region" metadata:"region" description:"AWS region the training job ran in"`
	URL                 string `json:"url" metadata:"url" description:"Link to the training job in the AWS console"`
}
