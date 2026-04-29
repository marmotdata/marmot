package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/ory/fosite"
)

func isLoopbackRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type dcrRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

type dcrResponse struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (h *Handler) handleDCR(w http.ResponseWriter, r *http.Request) {
	var req dcrRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.RedirectURIs) == 0 {
		respondOAuthError(w, http.StatusBadRequest, "invalid_client_metadata",
			"redirect_uris is required")
		return
	}

	for _, uri := range req.RedirectURIs {
		if !isLoopbackRedirectURI(uri) {
			respondOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri",
				"redirect_uris must be loopback http URLs (http://localhost, http://127.0.0.1, or http://[::1])")
			return
		}
	}

	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		respondOAuthError(w, http.StatusBadRequest, "invalid_client_metadata",
			"token_endpoint_auth_method must be \"none\" for public clients")
		return
	}

	clientID := uuid.New().String()

	client := &fosite.DefaultClient{
		ID:            clientID,
		Public:        true,
		RedirectURIs:  req.RedirectURIs,
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}

	h.oauthProvider.Store.RegisterClient(client)

	resp := dcrResponse{
		ClientID:                clientID,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              []string{"authorization_code"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
