package auth

// Credential records what a request authenticated with, for features that must stop working as soon as that credential is revoked rather than when a derived token expires. Lookup is the SHA-256 lookup of an API key, the same value the key tables index, and empty for other kinds.
type Credential struct {
	Type   string
	Lookup string
}

// Credential types. They name kinds of credential, not credentials.
const (
	CredentialUserAPIKey           = "user_api_key" //nolint:gosec // G101: a type name, not a secret
	CredentialServiceAccountAPIKey = "service_account_api_key"
	CredentialSession              = "session"
	CredentialKubernetes           = "kubernetes"
	CredentialOIDC                 = "oidc"
)

// IsAPIKey reports whether the credential is a user or service account API key.
func (c Credential) IsAPIKey() bool {
	return c.Type == CredentialUserAPIKey || c.Type == CredentialServiceAccountAPIKey
}
