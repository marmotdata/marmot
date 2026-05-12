// Package auth resolves credentials for the Marmot SDK.
package auth

import (
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
)

type Scheme string

const (
	SchemeAPIKey Scheme = "X-API-Key" //nolint:gosec // HTTP header name, not a credential
	SchemeBearer Scheme = "Bearer"
)

// Credential signs requests to the Marmot API.
type Credential interface {
	Token() string
	Scheme() Scheme
	Source() string
	AuthInfo() runtime.ClientAuthInfoWriter
}

func APIKey(token string) Credential {
	return &apiKeyCred{token: token, source: "explicit"}
}

func Bearer(token string) Credential {
	return &bearerCred{token: token, source: "explicit"}
}

type apiKeyCred struct {
	token  string
	source string
}

func (c *apiKeyCred) Token() string  { return c.token }
func (c *apiKeyCred) Scheme() Scheme { return SchemeAPIKey }
func (c *apiKeyCred) Source() string { return c.source }
func (c *apiKeyCred) AuthInfo() runtime.ClientAuthInfoWriter {
	return httptransport.APIKeyAuth("X-API-Key", "header", c.token)
}

type bearerCred struct {
	token  string
	source string
}

func (c *bearerCred) Token() string  { return c.token }
func (c *bearerCred) Scheme() Scheme { return SchemeBearer }
func (c *bearerCred) Source() string { return c.source }
func (c *bearerCred) AuthInfo() runtime.ClientAuthInfoWriter {
	return httptransport.BearerToken(c.token)
}
