package auth

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

type asMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTokenTypesSupported        []string `json:"subject_token_types_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

func (h *Handler) handleASMetadata(w http.ResponseWriter, r *http.Request) {
	root, derived := common.PublicRootURL(r, h.config)
	if derived {
		// Built from this request's host, so a shared cache must not hand it to a client that reached Marmot another way.
		w.Header().Set("Cache-Control", "no-store")
	}
	meta := asMetadata{
		Issuer:                 root,
		AuthorizationEndpoint:  root + "/oauth/authorize",
		TokenEndpoint:          root + "/oauth/token",
		RegistrationEndpoint:   root + "/oauth/register",
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported: []string{
			"authorization_code",
			grantTypeTokenExchange,
		},
		SubjectTokenTypesSupported: []string{
			tokenTypeIDToken,
			tokenTypeAccessToken,
		},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported:                   []string{"openid"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meta)
}
