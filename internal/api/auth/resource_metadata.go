package auth

import (
	"encoding/json"
	"net/http"
	"sort"

	coreauth "github.com/marmotdata/marmot/internal/core/auth"
)

type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

func (h *Handler) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	servers := []string{h.config.Server.RootURL}
	for _, issuer := range collectIssuers(h.oauthManager) {
		if issuer != h.config.Server.RootURL {
			servers = append(servers, issuer)
		}
	}

	meta := protectedResourceMetadata{
		Resource:               h.config.Server.RootURL,
		AuthorizationServers:   servers,
		ScopesSupported:        []string{"openid", "email", "profile"},
		BearerMethodsSupported: []string{"header"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meta)
}

func collectIssuers(mgr *coreauth.OAuthManager) []string {
	seen := make(map[string]struct{})
	var issuers []string

	for _, provider := range mgr.GetProviders() {
		ip, ok := provider.(coreauth.IssuerProvider)
		if !ok {
			continue
		}
		u := ip.IssuerURL()
		if u == "" {
			continue
		}
		if _, exists := seen[u]; exists {
			continue
		}
		seen[u] = struct{}{}
		issuers = append(issuers, u)
	}

	sort.Strings(issuers)
	return issuers
}
