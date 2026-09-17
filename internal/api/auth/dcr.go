package auth

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/rs/zerolog/log"
)

// Registration is unauthenticated and every accepted request becomes a row, so
// the bounds below are what stops anyone from filling the database with it.
const (
	maxDCRBodyBytes    = 4 << 10
	maxDCRRedirectURIs = 5
	maxRedirectURILen  = 256
)

func isLoopbackRedirectURI(raw string) bool {
	if len(raw) > maxRedirectURILen {
		return false
	}
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

// isAllowedExternalRedirectURI reports whether raw is an https redirect URI on an allowlisted host.
func isAllowedExternalRedirectURI(raw string, allowedHosts []string) bool {
	if len(allowedHosts) == 0 || len(raw) > maxRedirectURILen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	// RFC 6749 forbids fragments in redirect URIs.
	if u.User != nil || u.Fragment != "" || strings.Contains(raw, "#") {
		return false
	}
	for _, entry := range allowedHosts {
		if redirectHostMatches(entry, u) {
			return true
		}
	}
	return false
}

// redirectHostMatches reports whether an allowlist entry matches the URI host; an explicit :443 counts as no port since the scheme is always https here.
func redirectHostMatches(entry string, u *url.URL) bool {
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return false
	}
	port := u.Port()
	if port == "443" {
		port = ""
	}
	if host, entryPort, err := net.SplitHostPort(strings.ToLower(entry)); err == nil {
		if entryPort == "443" {
			entryPort = ""
		}
		return host == hostname && port == entryPort
	}
	return strings.ToLower(entry) == hostname && port == ""
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
	r.Body = http.MaxBytesReader(w, r.Body, maxDCRBodyBytes)

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

	if len(req.RedirectURIs) > maxDCRRedirectURIs {
		respondOAuthError(w, http.StatusBadRequest, "invalid_client_metadata",
			fmt.Sprintf("at most %d redirect_uris are accepted", maxDCRRedirectURIs))
		return
	}

	for _, uri := range req.RedirectURIs {
		if !isLoopbackRedirectURI(uri) && !isAllowedExternalRedirectURI(uri, h.config.Auth.DCR.AllowedRedirectHosts) {
			respondOAuthError(w, http.StatusBadRequest, "invalid_redirect_uri",
				"redirect_uris must be loopback http URLs (http://localhost, http://127.0.0.1, or http://[::1]) or https URLs on a host listed in auth.dcr.allowed_redirect_hosts")
			return
		}
	}

	if len(req.ClientName) > 128 {
		req.ClientName = req.ClientName[:128]
	}

	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		respondOAuthError(w, http.StatusBadRequest, "invalid_client_metadata",
			"token_endpoint_auth_method must be \"none\" for public clients")
		return
	}

	clientID := uuid.New().String()

	client := &marmotOAuth2.ClientRecord{
		ID:            clientID,
		RedirectURIs:  req.RedirectURIs,
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid"},
	}

	if err := h.oauthProvider.Store.RegisterClient(r.Context(), client); err != nil {
		log.Error().Err(err).Msg("Failed to store dynamically registered OAuth client")
		respondOAuthError(w, http.StatusInternalServerError, "server_error",
			"Could not register the client")
		return
	}

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
