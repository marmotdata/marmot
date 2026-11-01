package roles

import (
	_ "embed"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/role"
)

//go:embed schema/role.schema.json
var roleSchema []byte

// ValidateRoleResponse is the outcome of checking a role document without saving it.
type ValidateRoleResponse struct {
	Valid    bool                   `json:"valid"`
	Problems []role.DocumentProblem `json:"problems"`
} // @name ValidateRoleResponse

// @Summary Validate a role document
// @Description Checks a role document, in JSON or YAML, without saving it, and lists every problem found with where it is, so an editor can mark each one. Pass id to check an edit to an existing role, so it keeping its own name is not reported as a clash.
// @Tags roles
// @Accept json
// @Accept application/yaml
// @Produce json
// @Param id query string false "ID of the role being edited"
// @Param role body createRoleRequest true "Role document"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ValidateRoleResponse
// @Failure 400 {object} common.DocumentErrorResponse
// @ID postRolesValidate
// @Router /api/v1/roles/validate [post]
func (h *Handler) validateRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := common.DecodeDocument(r, &req); err != nil {
		common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	problems, err := h.roleService.ValidateDocument(r.Context(), role.CreateInput{
		Name:           req.Name,
		Description:    req.Description,
		PermIDs:        req.PermIDs,
		PermissionKeys: req.PermissionKeys,
	}, r.URL.Query().Get("id"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate role document")
		common.RespondError(w, http.StatusInternalServerError, "Failed to validate role")
		return
	}
	if problems == nil {
		problems = []role.DocumentProblem{}
	}
	common.RespondJSON(w, http.StatusOK, ValidateRoleResponse{Valid: len(problems) == 0, Problems: problems})
}

// @Summary Role document schema
// @Description The JSON Schema a role document follows, for editors and tooling.
// @Tags roles
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @ID getRolesSchema
// @Router /api/v1/roles/schema [get]
func (h *Handler) roleSchema(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/schema+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(roleSchema)
}
