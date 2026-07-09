package lambda

// LambdaFields represents Lambda-specific metadata fields
// +marmot:metadata
type LambdaFields struct {
	FunctionArn              string            `json:"function_arn" metadata:"function_arn" description:"The ARN of the Lambda function"`
	Runtime                  string            `json:"runtime" metadata:"runtime" description:"The runtime environment for the function (e.g. go1.x, python3.12, nodejs20.x)"`
	Handler                  string            `json:"handler" metadata:"handler" description:"The function's entry point handler"`
	Role                     string            `json:"role" metadata:"role" description:"The IAM execution role ARN"`
	CodeSize                 int64             `json:"code_size" metadata:"code_size" description:"The size of the function's deployment package in bytes"`
	CodeSha256               string            `json:"code_sha256" metadata:"code_sha256" description:"SHA256 hash of the deployment package"`
	PackageType              string            `json:"package_type" metadata:"package_type" description:"Deployment package type (Zip or Image)"`
	MemorySizeMB             int32             `json:"memory_size_mb" metadata:"memory_size_mb" description:"Memory allocated to the function in MB"`
	TimeoutSeconds           int32             `json:"timeout_seconds" metadata:"timeout_seconds" description:"Function execution timeout in seconds"`
	Description              string            `json:"description" metadata:"description" description:"The function's description"`
	LastModified             string            `json:"last_modified" metadata:"last_modified" description:"Date and time the function was last modified"`
	Version                  string            `json:"version" metadata:"version" description:"The function version"`
	Architectures            string            `json:"architectures" metadata:"architectures" description:"Instruction set architectures (x86_64, arm64)"`
	EnvironmentVariableCount int               `json:"environment_variable_count" metadata:"environment_variable_count" description:"Number of environment variables configured"`
	VpcID                    string            `json:"vpc_id" metadata:"vpc_id" description:"VPC ID if the function is connected to a VPC"`
	SubnetCount              int               `json:"subnet_count" metadata:"subnet_count" description:"Number of VPC subnets"`
	SecurityGroupCount       int               `json:"security_group_count" metadata:"security_group_count" description:"Number of VPC security groups"`
	EphemeralStorageMB       int32             `json:"ephemeral_storage_mb" metadata:"ephemeral_storage_mb" description:"Ephemeral storage allocated in MB"`
	Layers                   string            `json:"layers" metadata:"layers" description:"Lambda layer ARNs attached to the function"`
	LayerCount               int               `json:"layer_count" metadata:"layer_count" description:"Number of Lambda layers attached"`
	TracingMode              string            `json:"tracing_mode" metadata:"tracing_mode" description:"X-Ray tracing mode (Active or PassThrough)"`
	State                    string            `json:"state" metadata:"state" description:"Current state of the function (Active, Pending, Inactive, Failed)"`
	LastUpdateStatus         string            `json:"last_update_status" metadata:"last_update_status" description:"Status of the last update (Successful, Failed, InProgress)"`
	Tags                     map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}
