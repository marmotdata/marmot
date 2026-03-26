package teams

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	teamService *team.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

// ListTeamsResponse represents the response from the teams list endpoint.
type ListTeamsResponse struct {
	Teams  []team.Team `json:"teams"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// CreateTeamRequest represents the request body for creating a team.
type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateTeamRequest represents the request body for updating a team.
type UpdateTeamRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
}

// AddMemberRequest represents the request body for adding a team member.
type AddMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// UpdateMemberRoleRequest represents the request body for updating a member role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role"`
}

// ListMembersResponse represents the response from the list members endpoint.
type ListMembersResponse struct {
	Members []team.TeamMemberWithUser `json:"members"`
}

// MessageResponse represents a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// ListSSOMappingsResponse represents the response from the SSO mappings list endpoint.
type ListSSOMappingsResponse struct {
	Mappings []team.SSOTeamMapping `json:"mappings"`
}

// CreateSSOMappingRequest represents the request body for creating an SSO mapping.
type CreateSSOMappingRequest struct {
	Provider     string `json:"provider"`
	SSOGroupName string `json:"sso_group_name"`
	TeamID       string `json:"team_id"`
	MemberRole   string `json:"member_role"`
}

// UpdateSSOMappingRequest represents the request body for updating an SSO mapping.
type UpdateSSOMappingRequest struct {
	TeamID     string `json:"team_id"`
	MemberRole string `json:"member_role"`
}

// SearchOwnersResponse represents the response from the search owners endpoint.
type SearchOwnersResponse struct {
	Owners []team.Owner `json:"owners"`
}

func NewHandler(teamService *team.Service, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		teamService: teamService,
		userService: userService,
		authService: authService,
		config:      cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/teams",
			Method:  http.MethodGet,
			Handler: h.listTeams,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "view"),
			},
		},
		{
			Path:    "/api/v1/teams",
			Method:  http.MethodPost,
			Handler: h.createTeam,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}",
			Method:  http.MethodGet,
			Handler: h.getTeam,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "view"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}",
			Method:  http.MethodPut,
			Handler: h.updateTeam,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteTeam,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/members",
			Method:  http.MethodGet,
			Handler: h.listMembers,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "view"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/members",
			Method:  http.MethodPost,
			Handler: h.addMember,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/members/{userId}",
			Method:  http.MethodDelete,
			Handler: h.removeMember,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/members/{userId}/role",
			Method:  http.MethodPut,
			Handler: h.updateMemberRole,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/members/{userId}/convert-to-manual",
			Method:  http.MethodPost,
			Handler: h.convertMemberToManual,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "teams", "manage"),
			},
		},
		{
			Path:    "/api/v1/sso/team-mappings",
			Method:  http.MethodGet,
			Handler: h.listSSOMappings,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "sso", "manage"),
			},
		},
		{
			Path:    "/api/v1/sso/team-mappings",
			Method:  http.MethodPost,
			Handler: h.createSSOMapping,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "sso", "manage"),
			},
		},
		{
			Path:    "/api/v1/sso/team-mappings/{id}",
			Method:  http.MethodGet,
			Handler: h.getSSOMapping,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "sso", "manage"),
			},
		},
		{
			Path:    "/api/v1/sso/team-mappings/{id}",
			Method:  http.MethodPut,
			Handler: h.updateSSOMapping,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "sso", "manage"),
			},
		},
		{
			Path:    "/api/v1/sso/team-mappings/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteSSOMapping,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "sso", "manage"),
			},
		},
		{
			Path:    "/api/v1/owners/search",
			Method:  http.MethodGet,
			Handler: h.searchOwners,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
	}
}

// @Summary List teams
// @Description Get a paginated list of teams
// @Tags teams
// @Accept json
// @Produce json
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Success 200 {object} ListTeamsResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams [get]
func (h *Handler) listTeams(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	teams, total, err := h.teamService.ListTeams(r.Context(), limit, offset)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list teams")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"teams":  teams,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// @Summary Create a team
// @Description Create a new team
// @Tags teams
// @Accept json
// @Produce json
// @Param team body CreateTeamRequest true "Team creation request"
// @Success 201 {object} team.Team
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams [post]
func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	var req CreateTeamRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	user, _ := common.GetAuthenticatedUser(r.Context())
	createdTeam, err := h.teamService.CreateTeam(r.Context(), req.Name, req.Description, user.ID)
	if err != nil {
		if err == team.ErrTeamNameExists {
			common.RespondError(w, http.StatusConflict, "Team name already exists")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to create team")
		return
	}

	common.RespondJSON(w, http.StatusCreated, createdTeam)
}

// @Summary Get a team
// @Description Get a team by its ID
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Success 200 {object} team.Team
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id} [get]
func (h *Handler) getTeam(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	t, err := h.teamService.GetTeam(r.Context(), id)
	if err != nil {
		if err == team.ErrTeamNotFound {
			common.RespondError(w, http.StatusNotFound, "Team not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to get team")
		return
	}

	common.RespondJSON(w, http.StatusOK, t)
}

// @Summary Update a team
// @Description Update a team's fields by its ID
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param team body UpdateTeamRequest true "Team update request"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id} [put]
func (h *Handler) updateTeam(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateTeamRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.teamService.UpdateTeamFields(r.Context(), id, req.Name, req.Description, req.Metadata, req.Tags)
	if err != nil {
		if err == team.ErrTeamNotFound {
			common.RespondError(w, http.StatusNotFound, "Team not found")
			return
		}
		if err == team.ErrCannotEditSSOTeam {
			common.RespondError(w, http.StatusForbidden, "Cannot edit SSO-managed team")
			return
		}
		if err == team.ErrTeamNameExists {
			common.RespondError(w, http.StatusConflict, "Team name already exists")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to update team")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Team updated"})
}

// @Summary Delete a team
// @Description Delete a team by its ID
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Success 200 {object} MessageResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id} [delete]
func (h *Handler) deleteTeam(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := h.teamService.DeleteTeam(r.Context(), id)
	if err != nil {
		if err == team.ErrTeamNotFound {
			common.RespondError(w, http.StatusNotFound, "Team not found")
			return
		}
		if err == team.ErrCannotEditSSOTeam {
			common.RespondError(w, http.StatusForbidden, "Cannot delete SSO-managed team")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete team")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Team deleted"})
}

// @Summary List team members
// @Description Get the members of a team
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Success 200 {object} ListMembersResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id}/members [get]
func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	members, err := h.teamService.ListMembers(r.Context(), id)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list members")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"members": members,
	})
}

// @Summary Add a team member
// @Description Add a user as a member of a team
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param member body AddMemberRequest true "Member addition request"
// @Success 201 {object} MessageResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id}/members [post]
func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req AddMemberRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserID == "" {
		common.RespondError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	if req.Role == "" {
		req.Role = team.RoleMember
	}

	if req.Role != team.RoleOwner && req.Role != team.RoleMember {
		common.RespondError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	err := h.teamService.AddMember(r.Context(), id, req.UserID, req.Role)
	if err != nil {
		if err == team.ErrCannotEditSSOTeam {
			common.RespondError(w, http.StatusForbidden, "Cannot edit SSO-managed team")
			return
		}
		if err == team.ErrMemberAlreadyExists {
			common.RespondError(w, http.StatusConflict, "User is already a member")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to add member")
		return
	}

	common.RespondJSON(w, http.StatusCreated, map[string]string{"message": "Member added"})
}

// @Summary Remove a team member
// @Description Remove a user from a team
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param userId path string true "User ID"
// @Success 200 {object} MessageResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id}/members/{userId} [delete]
func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := r.PathValue("userId")

	err := h.teamService.RemoveMember(r.Context(), id, userID)
	if err != nil {
		if err == team.ErrMemberNotFound {
			common.RespondError(w, http.StatusNotFound, "Member not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to remove member")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Member removed"})
}

// @Summary Update member role
// @Description Update the role of a team member
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param userId path string true "User ID"
// @Param role body UpdateMemberRoleRequest true "Role update request"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id}/members/{userId}/role [put]
func (h *Handler) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := r.PathValue("userId")

	var req UpdateMemberRoleRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Role != team.RoleOwner && req.Role != team.RoleMember {
		common.RespondError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	err := h.teamService.UpdateMemberRole(r.Context(), id, userID, req.Role)
	if err != nil {
		if err == team.ErrMemberNotFound {
			common.RespondError(w, http.StatusNotFound, "Member not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to update member role")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Member role updated"})
}

// @Summary Convert member to manual
// @Description Convert an SSO-managed team member to a manually managed member
// @Tags teams
// @Accept json
// @Produce json
// @Param id path string true "Team ID"
// @Param userId path string true "User ID"
// @Success 200 {object} MessageResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /teams/{id}/members/{userId}/convert-to-manual [post]
func (h *Handler) convertMemberToManual(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := r.PathValue("userId")

	err := h.teamService.ConvertMemberToManual(r.Context(), id, userID)
	if err != nil {
		if err == team.ErrMemberNotFound {
			common.RespondError(w, http.StatusNotFound, "Member not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to convert member")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Member converted to manual"})
}

// @Summary List SSO team mappings
// @Description Get a list of SSO group to team mappings
// @Tags sso
// @Accept json
// @Produce json
// @Param provider query string false "Filter by SSO provider"
// @Success 200 {object} ListSSOMappingsResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /sso/team-mappings [get]
func (h *Handler) listSSOMappings(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")

	mappings, err := h.teamService.ListSSOMappings(r.Context(), provider)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list SSO mappings")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"mappings": mappings,
	})
}

// @Summary Create an SSO team mapping
// @Description Create a new SSO group to team mapping
// @Tags sso
// @Accept json
// @Produce json
// @Param mapping body CreateSSOMappingRequest true "SSO mapping creation request"
// @Success 201 {object} team.SSOTeamMapping
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /sso/team-mappings [post]
func (h *Handler) createSSOMapping(w http.ResponseWriter, r *http.Request) {
	var req CreateSSOMappingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Provider == "" || req.SSOGroupName == "" || req.TeamID == "" {
		common.RespondError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	if req.MemberRole == "" {
		req.MemberRole = team.RoleMember
	}

	mapping, err := h.teamService.CreateSSOMapping(r.Context(), req.Provider, req.SSOGroupName, req.TeamID, req.MemberRole)
	if err != nil {
		if err == team.ErrMappingAlreadyExists {
			common.RespondError(w, http.StatusConflict, "SSO mapping already exists")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to create SSO mapping")
		return
	}

	common.RespondJSON(w, http.StatusCreated, mapping)
}

// @Summary Get an SSO team mapping
// @Description Get an SSO team mapping by its ID
// @Tags sso
// @Accept json
// @Produce json
// @Param id path string true "SSO mapping ID"
// @Success 200 {object} team.SSOTeamMapping
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /sso/team-mappings/{id} [get]
func (h *Handler) getSSOMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	mapping, err := h.teamService.GetSSOMapping(r.Context(), id)
	if err != nil {
		if err == team.ErrMappingNotFound {
			common.RespondError(w, http.StatusNotFound, "SSO mapping not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to get SSO mapping")
		return
	}

	common.RespondJSON(w, http.StatusOK, mapping)
}

// @Summary Update an SSO team mapping
// @Description Update an SSO team mapping by its ID
// @Tags sso
// @Accept json
// @Produce json
// @Param id path string true "SSO mapping ID"
// @Param mapping body UpdateSSOMappingRequest true "SSO mapping update request"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /sso/team-mappings/{id} [put]
func (h *Handler) updateSSOMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req UpdateSSOMappingRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TeamID == "" || req.MemberRole == "" {
		common.RespondError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	err := h.teamService.UpdateSSOMapping(r.Context(), id, req.TeamID, req.MemberRole)
	if err != nil {
		if err == team.ErrMappingNotFound {
			common.RespondError(w, http.StatusNotFound, "SSO mapping not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to update SSO mapping")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "SSO mapping updated"})
}

// @Summary Delete an SSO team mapping
// @Description Delete an SSO team mapping by its ID
// @Tags sso
// @Accept json
// @Produce json
// @Param id path string true "SSO mapping ID"
// @Success 200 {object} MessageResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /sso/team-mappings/{id} [delete]
func (h *Handler) deleteSSOMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := h.teamService.DeleteSSOMapping(r.Context(), id)
	if err != nil {
		if err == team.ErrMappingNotFound {
			common.RespondError(w, http.StatusNotFound, "SSO mapping not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete SSO mapping")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "SSO mapping deleted"})
}

// @Summary Search owners
// @Description Search for asset owners (users and teams)
// @Tags owners
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param limit query int false "Maximum number of results" default(20)
// @Success 200 {object} SearchOwnersResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /owners/search [get]
func (h *Handler) searchOwners(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		common.RespondError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	owners, err := h.teamService.SearchOwners(r.Context(), query, limit)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to search owners")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"owners": owners,
	})
}
