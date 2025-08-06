package openapi

// OpenAPIFields represents OpenAPI-specific metadata fields
// +marmot:metadata
type OpenAPIFields struct {
	Description	string `json:"description" metadata:"description" description:"Description of the API"`
	ServiceName	string `json:"service_name" metadata:"service_name" description:"Name of the service that owns the resource"`
	ServiceVersion	string `json:"service_version" metadata:"service_version" description:"Version of the service"`
	OpenAPIVersion 	string `json:"openapi_version" metadata:"openapi_version" description:"Version of the OpenAPI spec"`
	Servers		[]string `json:"servers" metadata:"servers" description:"URL of the servers of the API"`
	NumPaths	int	`json:"num_paths" metadata:"num_paths" description:"Number of paths in the API"`
}
