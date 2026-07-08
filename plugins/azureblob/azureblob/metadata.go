package azureblob

// AzureBlobContainerFields defines metadata fields for Azure Blob containers
// +marmot:metadata
type AzureBlobContainerFields struct {
	ContainerName         string `json:"container_name" metadata:"container_name" description:"Name of the container"`
	LastModified          string `json:"last_modified" metadata:"last_modified" description:"Last modification timestamp"`
	Etag                  string `json:"etag" metadata:"etag" description:"Entity tag for the container"`
	LeaseStatus           string `json:"lease_status" metadata:"lease_status" description:"Lease status (locked/unlocked)"`
	LeaseState            string `json:"lease_state" metadata:"lease_state" description:"Lease state (available/leased/expired/breaking/broken)"`
	HasImmutabilityPolicy bool   `json:"has_immutability_policy" metadata:"has_immutability_policy" description:"Whether container has an immutability policy"`
	HasLegalHold          bool   `json:"has_legal_hold" metadata:"has_legal_hold" description:"Whether container has a legal hold"`
	PublicAccess          string `json:"public_access" metadata:"public_access" description:"Public access level (none/blob/container)"`
	BlobCount             int64  `json:"blob_count" metadata:"blob_count" description:"Number of blobs in the container"`
}
