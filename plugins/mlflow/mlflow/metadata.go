package mlflow

// MLflowModelFields describes the metadata fields the MLflow plugin emits
// for registered model assets. It is kept as a documentation-only struct
// so downstream tooling can introspect the shape of the metadata map.
type MLflowModelFields struct {
	Description     string            `json:"description" metadata:"description" description:"Registered model description"`
	Tags            map[string]string `json:"tags" metadata:"tags" description:"Registered model tags"`
	CreatedAt       string            `json:"created_at" metadata:"created_at" description:"When the registered model was created"`
	UpdatedAt       string            `json:"updated_at" metadata:"updated_at" description:"When the registered model was last updated"`
	LatestVersion   string            `json:"latest_version" metadata:"latest_version" description:"Highest version number"`
	VersionCount    int               `json:"version_count" metadata:"version_count" description:"Number of versions"`
	Aliases         map[string]string `json:"aliases" metadata:"aliases" description:"Alias to version number"`
	Stage           string            `json:"stage" metadata:"stage" description:"Stage of the latest version, when one is set"`
	RunID           string            `json:"run_id" metadata:"run_id" description:"Run that produced the latest version"`
	RunName         string            `json:"run_name" metadata:"run_name" description:"Name of that run"`
	ExperimentID    string            `json:"experiment_id" metadata:"experiment_id" description:"Experiment the run belongs to"`
	Experiment      string            `json:"experiment" metadata:"experiment" description:"Name of that experiment"`
	ArtifactURI     string            `json:"artifact_uri" metadata:"artifact_uri" description:"Where the run's artifacts are stored"`
	Status          string            `json:"status" metadata:"status" description:"Status of the latest version"`
	Hyperparameters map[string]string `json:"hyperparameters" metadata:"hyperparameters" description:"Parameters logged to the run"`
	Metrics         map[string]any    `json:"metrics" metadata:"metrics" description:"Latest value of each metric logged to the run"`
	URL             string            `json:"url" metadata:"url" description:"Link to the model in the MLflow UI"`
}

// MLflowExperimentFields describes the metadata fields emitted for
// experiment assets.
type MLflowExperimentFields struct {
	ExperimentID     string            `json:"experiment_id" metadata:"experiment_id" description:"Experiment id"`
	ArtifactLocation string            `json:"artifact_location" metadata:"artifact_location" description:"Where the experiment's runs store artifacts"`
	LifecycleStage   string            `json:"lifecycle_stage" metadata:"lifecycle_stage" description:"Lifecycle stage (active)"`
	Tags             map[string]string `json:"tags" metadata:"tags" description:"Experiment tags"`
	CreatedAt        string            `json:"created_at" metadata:"created_at" description:"When the experiment was created"`
	UpdatedAt        string            `json:"updated_at" metadata:"updated_at" description:"When the experiment was last updated"`
	URL              string            `json:"url" metadata:"url" description:"Link to the experiment in the MLflow UI"`
}

// MLflowDatasetFields describes the metadata fields emitted for dataset
// assets, one per dataset logged to the run behind a model.
type MLflowDatasetFields struct {
	Digest     string `json:"digest" metadata:"digest" description:"Content digest MLflow computed for the dataset"`
	SourceType string `json:"source_type" metadata:"source_type" description:"Kind of source the dataset was read from"`
	Source     any    `json:"source" metadata:"source" description:"Where the dataset was read from"`
	Context    string `json:"context" metadata:"context" description:"What the dataset was used for (training, eval)"`
	Profile    any    `json:"profile" metadata:"profile" description:"Profile MLflow computed for the dataset, such as row counts"`
}

// MLflowColumnFields describes the per-feature fields embedded in a model
// or dataset asset's schema.
type MLflowColumnFields struct {
	ColumnName string `json:"column_name" metadata:"column_name" description:"Feature or column name"`
	DataType   string `json:"data_type" metadata:"data_type" description:"MLflow data type"`
	IsNullable bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether the input is optional"`
}
