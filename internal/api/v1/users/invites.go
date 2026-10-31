package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/limits"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

// TeamAdder adds members to teams, so an invitation can place the person in their teams before they arrive.
type TeamAdder interface {
	AddMember(ctx context.Context, teamID, userID, role string) error
}

// WithTeams lets invitations add people to teams.
func (h *Handler) WithTeams(t TeamAdder) *Handler {
	h.teams = t
	return h
}

// InviteRequest is who to invite, what they can do and which teams they join.
type InviteRequest struct {
	Email     string   `json:"email"`
	Name      string   `json:"name,omitempty"`
	RoleNames []string `json:"role_names"`
	TeamIDs   []string `json:"team_ids,omitempty"`
} // @name InviteRequest

// InviteResponse carries the one-time link. Marmot sends no email, so whoever invited the person passes the link on; it is shown only in this response.
type InviteResponse struct {
	User      *user.User `json:"user"`
	Token     string     `json:"token"`
	InviteURL string     `json:"invite_url"`
	ExpiresAt time.Time  `json:"expires_at"`
	// FailedTeamIDs are the teams the person could not be added to, such as teams whose membership comes from single sign-on.
	FailedTeamIDs []string `json:"failed_team_ids,omitempty"`
} // @name InviteResponse

// AcceptInviteRequest sets the invited person's password and, optionally, corrects their name.
type AcceptInviteRequest struct {
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
} // @name AcceptInviteRequest

func (h *Handler) inviteRoutes() []common.Route {
	manage := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "users", "manage"),
	}
	public := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithRateLimit(h.config, 10, 60),
	}
	return []common.Route{
		{Path: "/api/v1/users/invites", Method: http.MethodPost, Handler: h.inviteUser, Middleware: manage},
		{Path: "/api/v1/users/invites/{id}/renew", Method: http.MethodPost, Handler: h.renewInvite, Middleware: manage},
		{Path: "/api/v1/users/invites/{id}", Method: http.MethodDelete, Handler: h.revokeInvite, Middleware: manage},
		{Path: "/api/v1/invites/{token}", Method: http.MethodGet, Handler: h.getInvite, Middleware: public},
		{Path: "/api/v1/invites/{token}/accept", Method: http.MethodPost, Handler: h.acceptInvite, Middleware: public},
	}
}

// inviteURL is absolute when the server knows its own address, and otherwise a path the UI resolves against the address it was loaded from.
func (h *Handler) inviteURL(token string) string {
	path := "/invite/" + token
	if h.config != nil && h.config.Server.RootURL != "" {
		return strings.TrimRight(h.config.Server.RootURL, "/") + path
	}
	return path
}

func respondInviteError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, user.ErrInviteNotFound):
		common.RespondError(w, http.StatusNotFound, "This invitation link is not valid. It may have been used already or withdrawn.")
	case errors.Is(err, user.ErrInviteExpired):
		common.RespondError(w, http.StatusGone, "This invitation link has expired. Ask whoever invited you for a new one.")
	case errors.Is(err, user.ErrNotInvited):
		common.RespondError(w, http.StatusConflict, "This member has no pending invitation")
	case errors.Is(err, user.ErrPasswordSignInDisabled):
		common.RespondError(w, http.StatusConflict, "Password sign-in is turned off. Accept the invitation by signing in with single sign-on.")
	default:
		return respondUserError(w, err)
	}
	return true
}

// @Summary Invite a member
// @Description Creates a pending member with the given roles and teams and returns a one-time link for them to set a password. Marmot sends no email: pass the link on yourself. The link works for 7 days.
// @Tags users
// @Accept json
// @Produce json
// @Param invite body InviteRequest true "Who to invite"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} InviteResponse
// @Failure 400 {object} common.ValidationErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID postUsersInvites
// @Router /api/v1/users/invites [post]
func (h *Handler) inviteUser(w http.ResponseWriter, r *http.Request) {
	var req InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	invitedBy := ""
	if u, ok := common.GetAuthenticatedUser(r.Context()); ok {
		invitedBy = u.ID
	}

	inv, err := h.userService.Invite(r.Context(), user.InviteInput{Email: req.Email, Name: req.Name, RoleNames: req.RoleNames}, invitedBy)
	if err != nil {
		if limitErr, ok := limits.AsLimitExceeded(err); ok {
			common.RespondLimitExceeded(w, limitErr)
			return
		}
		if respondInviteError(w, err) {
			return
		}
		log.Error().Err(err).Msg("Failed to invite user")
		common.RespondError(w, http.StatusInternalServerError, "Failed to invite user")
		return
	}

	resp := InviteResponse{User: inv.User, Token: inv.Token, InviteURL: h.inviteURL(inv.Token), ExpiresAt: inv.ExpiresAt}
	for _, teamID := range req.TeamIDs {
		if h.teams == nil {
			resp.FailedTeamIDs = append(resp.FailedTeamIDs, teamID)
			continue
		}
		if err := h.teams.AddMember(r.Context(), teamID, inv.User.ID, "member"); err != nil {
			log.Warn().Err(err).Str("team_id", teamID).Str("user_id", inv.User.ID).Msg("Failed to add invited user to team")
			resp.FailedTeamIDs = append(resp.FailedTeamIDs, teamID)
		}
	}

	common.RespondJSON(w, http.StatusCreated, resp)
}

// @Summary Renew an invitation
// @Description Replaces a pending member's invitation link with a new one that works for another 7 days. The old link stops working.
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} InviteResponse
// @Failure 409 {object} common.ErrorResponse
// @ID postUsersIDInvite
// @Router /api/v1/users/invites/{id}/renew [post]
func (h *Handler) renewInvite(w http.ResponseWriter, r *http.Request) {
	invitedBy := ""
	if u, ok := common.GetAuthenticatedUser(r.Context()); ok {
		invitedBy = u.ID
	}
	inv, err := h.userService.RenewInvite(r.Context(), r.PathValue("id"), invitedBy)
	if err != nil {
		if respondInviteError(w, err) {
			return
		}
		log.Error().Err(err).Msg("Failed to renew invitation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to renew invitation")
		return
	}
	common.RespondJSON(w, http.StatusOK, InviteResponse{User: inv.User, Token: inv.Token, InviteURL: h.inviteURL(inv.Token), ExpiresAt: inv.ExpiresAt})
}

// @Summary Withdraw an invitation
// @Description Withdraws a pending invitation and removes the pending member it created.
// @Tags users
// @Param id path string true "User ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 409 {object} common.ErrorResponse
// @ID deleteUsersIDInvite
// @Router /api/v1/users/invites/{id} [delete]
func (h *Handler) revokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := h.userService.RevokeInvite(r.Context(), r.PathValue("id")); err != nil {
		if respondInviteError(w, err) {
			return
		}
		log.Error().Err(err).Msg("Failed to withdraw invitation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to withdraw invitation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Look up an invitation
// @Description Returns who an invitation link is for, so the accept page can greet them. Needs no sign-in: the link's token is the credential.
// @Tags users
// @Produce json
// @Param token path string true "Invitation token"
// @Success 200 {object} user.InviteInfo
// @Failure 404 {object} common.ErrorResponse
// @Failure 410 {object} common.ErrorResponse
// @ID getInvitesToken
// @Router /api/v1/invites/{token} [get]
func (h *Handler) getInvite(w http.ResponseWriter, r *http.Request) {
	info, err := h.userService.GetInvite(r.Context(), r.PathValue("token"))
	if err != nil {
		if respondInviteError(w, err) {
			return
		}
		log.Error().Err(err).Msg("Failed to look up invitation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to look up invitation")
		return
	}
	common.RespondJSON(w, http.StatusOK, info)
}

// @Summary Accept an invitation
// @Description Sets the invited member's password, activates them and signs them in. The link cannot be used again.
// @Tags users
// @Accept json
// @Produce json
// @Param token path string true "Invitation token"
// @Param body body AcceptInviteRequest true "The new password"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} common.ValidationErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 410 {object} common.ErrorResponse
// @ID postInvitesTokenAccept
// @Router /api/v1/invites/{token}/accept [post]
func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var req AcceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	u, err := h.userService.AcceptInvite(r.Context(), r.PathValue("token"), req.Password, req.Name)
	if err != nil {
		if respondInviteError(w, err) {
			return
		}
		log.Error().Err(err).Msg("Failed to accept invitation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to accept invitation")
		return
	}

	token, err := h.authService.GenerateToken(r.Context(), u, map[string]interface{}{"theme": ""})
	if err != nil {
		log.Error().Err(err).Str("user_id", u.ID).Msg("Failed to generate token")
		common.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	common.RespondJSON(w, http.StatusOK, TokenResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: 24 * 60 * 60})
}
