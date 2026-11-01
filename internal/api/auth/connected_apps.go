package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/rs/zerolog/log"
)

// ConnectedApp is an OAuth client the current user has allowed and that still holds a token.
type ConnectedApp struct {
	marmotOAuth2.Grant
	ScopeDescriptions []marmotOAuth2.ScopeDescription `json:"scope_descriptions"`
} // @name ConnectedApp

// ConnectedAppsResponse lists the current user's connected apps.
type ConnectedAppsResponse struct {
	Apps []ConnectedApp `json:"apps"`
} // @name ConnectedAppsResponse

// listConnectedApps lists the OAuth clients the current user has allowed.
//
// @Summary List connected apps
// @Description List the OAuth clients, such as MCP clients and the CLI, that the current user has allowed and that still hold a valid token
// @Tags users
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ConnectedAppsResponse
// @Failure 401 {object} common.ErrorResponse
// @ID getUsersMeConnectedApps
// @Router /api/v1/users/me/connected-apps [get]
func (h *Handler) listConnectedApps(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	resp := ConnectedAppsResponse{Apps: []ConnectedApp{}}
	if h.oauthProvider == nil || h.oauthProvider.Grants == nil {
		common.RespondJSON(w, http.StatusOK, resp)
		return
	}

	grants, err := h.oauthProvider.Grants.ListGrants(r.Context(), usr.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to list connected apps")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list connected apps")
		return
	}
	for _, g := range grants {
		resp.Apps = append(resp.Apps, ConnectedApp{Grant: g, ScopeDescriptions: marmotOAuth2.DescribeScopes(g.Scopes)})
	}
	common.RespondJSON(w, http.StatusOK, resp)
}

// revokeConnectedApp revokes an OAuth client's access for the current user.
//
// @Summary Revoke a connected app
// @Description Revoke an OAuth client's access to the current user's account. Tokens already issued to it stop working at once; the client has to ask again to get back in.
// @Tags users
// @Param client_id path string true "OAuth client ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @ID deleteUsersMeConnectedAppsClientID
// @Router /api/v1/users/me/connected-apps/{client_id} [delete]
func (h *Handler) revokeConnectedApp(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if h.oauthProvider == nil || h.oauthProvider.Grants == nil {
		common.RespondError(w, http.StatusNotFound, "Connected app not found")
		return
	}

	clientID := r.PathValue("client_id")
	found, err := h.oauthProvider.Grants.RevokeGrant(r.Context(), usr.ID, clientID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Str("client_id", clientID).Msg("Failed to revoke connected app")
		common.RespondError(w, http.StatusInternalServerError, "Failed to revoke connected app")
		return
	}
	if !found {
		common.RespondError(w, http.StatusNotFound, "Connected app not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
