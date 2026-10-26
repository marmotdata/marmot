// Package pluginserve exposes enrolment for plugins that serve queries from outside Marmot.
package pluginserve

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/pluginserve"
	"github.com/marmotdata/marmot/pkg/config"
)

// maxEnrolBody bounds an enrolment request: a CSR and up to a hundred masked source configs.
const maxEnrolBody = 1 << 20

type Handler struct {
	enroller    *pluginserve.Enroller
	keyring     *pluginserve.Keyring
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(enroller *pluginserve.Enroller, keyring *pluginserve.Keyring, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{enroller: enroller, keyring: keyring, userService: userService, authService: authService, config: config}
}

func (h *Handler) Routes() []common.Route {
	withAuth := common.WithAuth(h.userService, h.authService, h.config)
	// Rotating the serving keys changes what every serving plugin trusts, so it is limited to administrators.
	admin := common.RequirePermission(h.userService, "users", "manage")
	return []common.Route{{
		Path:       "/api/v1/query/serve/keys",
		Method:     http.MethodGet,
		Handler:    h.listKeys,
		Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, admin},
	}, {
		Path:       "/api/v1/query/serve/keys/rotate",
		Method:     http.MethodPost,
		Handler:    h.rotateKeys,
		Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, admin},
	}, {
		Path:    "/api/v1/query/serve/enrol",
		Method:  http.MethodPost,
		Handler: h.enrol,
		Middleware: []func(http.HandlerFunc) http.HandlerFunc{
			common.WithAuth(h.userService, h.authService, h.config),
			common.RequirePermission(h.userService, "ingestion", "serve"),
			common.WithRateLimit(h.config, 60, 60),
		},
	}}
}

// @Summary Enrol a serving plugin
// @Description Issues a short-lived client certificate for a plugin running outside Marmot to connect and serve queries for the named sources. The caller must hold ingestion:serve, and each source must be free or already served by the caller.
// @Tags query
// @Accept json
// @Produce json
// @Param request body pluginserve.EnrolInput true "Certificate request and sources"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} pluginserve.EnrolResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Router /query/serve/enrol [post]
func (h *Handler) enrol(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var in pluginserve.EnrolInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEnrolBody)).Decode(&in); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	credential, _ := common.CredentialFromContext(r.Context())
	res, err := h.enroller.Enrol(r.Context(), principal, credential, in)
	switch {
	case errors.Is(err, pluginserve.ErrInvalidEnrolment):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, pluginserve.ErrForbidden):
		common.RespondError(w, http.StatusForbidden, err.Error())
	case err != nil:
		log.Error().Err(err).Msg("Failed to enrol serving plugin")
		common.RespondError(w, http.StatusInternalServerError, "Failed to enrol")
	default:
		common.RespondJSON(w, http.StatusOK, res)
	}
}

// RotateKeysRequest starts a key rotation.
type RotateKeysRequest struct {
	// Immediate switches to the new keys at once and stops trusting the old ones, for a suspected compromise; serving plugins must then be restarted.
	Immediate bool `json:"immediate"`
	// CAKeyURI and SigningKeyURI name new KMS keys. Leave both empty to have Marmot generate keys it stores encrypted, which is refused when the current keys are in a KMS.
	CAKeyURI      string `json:"ca_key_uri,omitempty"`
	SigningKeyURI string `json:"signing_key_uri,omitempty"`
} // @name ServeRotateKeysRequest

// @Summary List serving key generations
// @Tags query
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} pluginserve.GenerationStatus
// @Router /query/serve/keys [get]
func (h *Handler) listKeys(w http.ResponseWriter, _ *http.Request) {
	common.RespondJSON(w, http.StatusOK, h.keyring.Generations())
}

// @Summary Rotate serving keys
// @Description Starts a new generation of the CA and query signing keys that secure serving plugins. A graceful rotation publishes the new keys, uses them after the overlap and then retires the old ones, without interrupting serving plugins. An immediate rotation is for a suspected compromise: it switches at once and serving plugins must be restarted.
// @Tags query
// @Accept json
// @Produce json
// @Param request body RotateKeysRequest true "Rotation"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} pluginserve.GenerationStatus
// @Failure 400 {object} common.ErrorResponse
// @Router /query/serve/keys/rotate [post]
func (h *Handler) rotateKeys(w http.ResponseWriter, r *http.Request) {
	var req RotateKeysRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	err := h.keyring.Rotate(r.Context(), pluginserve.KeyConfig{CAKeyURI: req.CAKeyURI, SigningKeyURI: req.SigningKeyURI}, req.Immediate)
	if err != nil {
		log.Error().Err(err).Msg("Failed to rotate serving keys")
		common.RespondError(w, http.StatusBadRequest, "Failed to rotate serving keys: "+err.Error())
		return
	}
	principal, _ := common.PrincipalFromContext(r.Context())
	log.Warn().Bool("immediate", req.Immediate).Str("by", auditSubject(principal)).Msg("Serving keys rotated")
	common.RespondJSON(w, http.StatusOK, h.keyring.Generations())
}

func auditSubject(p auth.Principal) string {
	if p == nil {
		return ""
	}
	return p.AuditSubject()
}
