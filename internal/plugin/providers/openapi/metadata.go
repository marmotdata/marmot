package openapi

// OpenAPIFields represents OpenAPI-specific metadata fields
// +marmot:metadata
type OpenAPIFields struct {
	ContactEmail	string `json:"contact_email" metadata:"contact_email" description:"Contact email"`
	ContactName	string `json:"contact_name" metadata:"contact_name" description:"Contact name"`
	ContactURL	string `json:"contact_url" metadata:"contact_url" description:"Contact URL"`
	Description	string `json:"description" metadata:"description" description:"Description of the API"`
	ExternalDocs	string `json:"external_docs" metadata:"external_docs" description:"Link to the external documentation"`
	LicenseIdentifier	string `json:"license_identifier" metadata:"license_identifier" description:"SPDX license experession for the API"`
	LicenseName	string `json:"license_name" metadata:"license_name" description:"Name of the license"`
	LicenseURL	string `json:"license_url" metadata:"license_url" description:"URL of the license"`
	NumEndpoints	int	`json:"num_endpoints" metadata:"num_endpoints" description:"Number of endpoints in the OpenAPI specification"`
	OpenAPIVersion 	string `json:"openapi_version" metadata:"openapi_version" description:"Version of the OpenAPI spec"`
	Servers		[]string `json:"servers" metadata:"servers" description:"URL of the servers of the API"`
	ServiceName	string `json:"service_name" metadata:"service_name" description:"Name of the service that owns the resource"`
	ServiceVersion	string `json:"service_version" metadata:"service_version" description:"Version of the service"`
	TermsOfService	string `json:"terms_of_service" metadata:"terms_of_service" description:"Link to the page that describes the terms of service"`
}

// EndpointFields represents endpoints in OpenAPI specifications
// +marmot:metadata
type EndpointFields struct {
	Description	string `json:"description" metadata:"description" description:"A verbose explanation of the operation behaviour."`
	StatusCodes	[]string `json:"status_codes" metadata:"status_codes" description:"All HTTP response status codes that are returned for this endpoint."`
	HTTPMethod	string `json:"http_method" metadata:"http_method" description:"HTTP method"`
	OperationID	string `json:"operation_id" metadata:"operation_id" description:"Unique identifier of the operation"`
	Path		string `json:"path" metadata:"path" description:"Path"`
	Summary		string `json:"summary" metadata:"summary" description:"A short summary of what the operation does"`
}
