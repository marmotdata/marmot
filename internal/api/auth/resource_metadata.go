package auth

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

func (h *Handler) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	root, derived := common.PublicRootURL(r, h.config)
	if derived {
		// Built from this request's host, so a shared cache must not hand it to a client that reached Marmot another way.
		w.Header().Set("Cache-Control", "no-store")
	}
	meta := protectedResourceMetadata{
		Resource:               root,
		AuthorizationServers:   []string{root},
		ScopesSupported:        []string{"openid"},
		BearerMethodsSupported: []string{"header"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meta)
}
