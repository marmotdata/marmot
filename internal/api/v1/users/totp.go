package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/mfa"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type TOTPRequest struct {
	Password    string `json:"password"`
	Code        string `json:"code"`
	MFAToken    string `json:"mfa_token"`
	NewPassword string `json:"new_password"`
}

type RecoveryResponse struct {
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
	AccessToken   string   `json:"access_token"`
}

func (h *Handler) totpRoutes() []common.Route {
	// MFA throttling is required even when optional instance-wide rate limits are off.
	cfg := *h.config
	cfg.RateLimit.Enabled = true
	limit := common.WithRateLimit(&cfg, 300, 60)
	protect := common.WithAuth(h.userService, h.authService, h.config)
	return []common.Route{
		{Path: "/api/v1/users/login/totp", Method: http.MethodPost, Handler: h.verifyTOTPLogin, Middleware: []func(http.HandlerFunc) http.HandlerFunc{limit}},
		{Path: "/api/v1/users/login/password", Method: http.MethodPost, Handler: h.changeLoginPassword, Middleware: []func(http.HandlerFunc) http.HandlerFunc{limit}},
		{Path: "/api/v1/users/login/totp/setup", Method: http.MethodPost, Handler: h.setupRequiredTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{limit}},
		{Path: "/api/v1/users/login/totp/confirm", Method: http.MethodPost, Handler: h.confirmRequiredTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{limit}},
		{Path: "/api/v1/users/totp", Method: http.MethodGet, Handler: h.totpStatus, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect}},
		{Path: "/api/v1/users/totp/enrolled", Method: http.MethodGet, Handler: h.listTOTPEnrollment, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequirePermission(h.userService, "users", "view")}},
		{Path: "/api/v1/users/totp/setup", Method: http.MethodPost, Handler: h.setupTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequireJWTSession("local"), limit}},
		{Path: "/api/v1/users/totp/confirm", Method: http.MethodPost, Handler: h.confirmTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequireJWTSession("local"), limit}},
		{Path: "/api/v1/users/totp", Method: http.MethodDelete, Handler: h.disableTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequireJWTSession("local"), limit}},
		{Path: "/api/v1/users/totp/recovery", Method: http.MethodPost, Handler: h.regenerateRecovery, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequireJWTSession("local"), limit}},
		{Path: "/api/v1/users/totp/reset/{id}", Method: http.MethodDelete, Handler: h.resetTOTP, Middleware: []func(http.HandlerFunc) http.HandlerFunc{protect, common.RequireJWTSession(""), common.RequirePermission(h.userService, "users", "manage"), limit}},
	}
}

// @Summary List users with an enrolled second factor
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string][]string
// @ID listTOTPEnrollment
// @Router /api/v1/users/totp/enrolled [get]
func (h *Handler) listTOTPEnrollment(w http.ResponseWriter, r *http.Request) {
	if h.mfa == nil {
		h.respondMFAError(w, mfa.ErrUnavailable)
		return
	}
	ids, err := h.mfa.EnabledUserIDs(r.Context())
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	common.RespondJSON(w, 200, map[string][]string{"user_ids": ids})
}

func (h *Handler) respondMFAError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mfa.ErrUnavailable):
		common.RespondError(w, 503, "TOTP encryption is not configured")
	case errors.Is(err, mfa.ErrNotLocal):
		common.RespondError(w, 403, "TOTP requires a local account")
	case errors.Is(err, mfa.ErrRateLimited):
		common.RespondError(w, 429, "Too many verification attempts")
	case errors.Is(err, mfa.ErrAlreadyEnabled), errors.Is(err, mfa.ErrNotEnabled):
		common.RespondError(w, 409, "TOTP state changed")
	case errors.Is(err, mfa.ErrInvalidCode), errors.Is(err, mfa.ErrInvalidChallenge):
		common.RespondError(w, 401, "Invalid code or expired challenge")
	default:
		log.Error().Err(err).Msg("TOTP operation failed")
		common.RespondError(w, 500, "TOTP operation failed")
	}
}

func (h *Handler) totpInput(w http.ResponseWriter, r *http.Request) (TOTPRequest, bool) {
	var input TOTPRequest
	w.Header().Set("Cache-Control", "no-store")
	if h.mfa == nil {
		h.respondMFAError(w, mfa.ErrUnavailable)
		return input, false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		common.RespondError(w, 400, "Invalid request body")
		return input, false
	}
	if len(input.Password) > 72 || len(input.Code) > 128 || len(input.MFAToken) > 256 || len(input.NewPassword) > 72 {
		common.RespondError(w, 400, "Invalid request body")
		return input, false
	}
	return input, true
}

func (h *Handler) totpUser(w http.ResponseWriter, r *http.Request) (*user.User, bool) {
	u, ok := common.GetAuthenticatedUser(r.Context())
	if !ok || u == nil || common.IsSyntheticUser(u) {
		common.RespondError(w, 403, "Local account required")
		return nil, false
	}
	if h.mfa == nil {
		h.respondMFAError(w, mfa.ErrUnavailable)
		return nil, false
	}
	return u, true
}

func (h *Handler) reauthenticate(w http.ResponseWriter, r *http.Request, u *user.User, password string) bool {
	authenticated, err := h.userService.Authenticate(r.Context(), u.Username, password)
	if err != nil || authenticated.ID != u.ID {
		common.RespondError(w, 401, "Invalid credentials")
		return false
	}
	return true
}

// @Summary Get local two-factor status
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Success 200 {object} mfa.Status
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID totpStatus
// @Router /api/v1/users/totp [get]
func (h *Handler) totpStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := h.totpUser(w, r)
	if !ok {
		return
	}
	status, err := h.mfa.Status(r.Context(), u.ID)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	common.RespondJSON(w, 200, status)
}

// @Summary Prepare an encrypted TOTP enrollment
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Security BearerAuth
// @Success 200 {object} mfa.SetupResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID setupTOTP
// @Router /api/v1/users/totp/setup [post]
func (h *Handler) setupTOTP(w http.ResponseWriter, r *http.Request) {
	u, ok := h.totpUser(w, r)
	if !ok {
		return
	}
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if !h.reauthenticate(w, r, u, in.Password) {
		return
	}
	setup, err := h.mfa.Setup(r.Context(), u.ID)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	common.RespondJSON(w, 200, setup)
}

// @Summary Confirm TOTP and return recovery codes once
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Security BearerAuth
// @Success 200 {object} RecoveryResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID confirmTOTP
// @Router /api/v1/users/totp/confirm [post]
func (h *Handler) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	u, ok := h.totpUser(w, r)
	if !ok {
		return
	}
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	codes, err := h.mfa.Confirm(r.Context(), u.ID, in.Code)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	h.respondTOTPRecovery(w, r, u.ID, codes)
}

// @Summary Disable TOTP with password and second factor
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Security BearerAuth
// @Success 200 {object} RecoveryResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID disableTOTP
// @Router /api/v1/users/totp [delete]
func (h *Handler) disableTOTP(w http.ResponseWriter, r *http.Request) {
	if h.config.Auth.TOTP.Required {
		common.RespondError(w, 403, "Two-factor authentication is required")
		return
	}
	u, ok := h.totpUser(w, r)
	if !ok {
		return
	}
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if !h.reauthenticate(w, r, u, in.Password) {
		return
	}
	if err := h.mfa.Disable(r.Context(), u.ID, in.Code); err != nil {
		h.respondMFAError(w, err)
		return
	}
	h.respondTOTPRecovery(w, r, u.ID, nil)
}

// Enrollment endpoints accept only a password-verified, purpose-bound challenge.
// They never accept a normal session token in place of the challenge.
// @Summary Prepare required TOTP enrollment after password login
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Enrollment challenge"
// @Success 200 {object} mfa.SetupResult
// @Failure 401 {object} common.ErrorResponse
// @ID setupRequiredTOTP
// @Router /api/v1/users/login/totp/setup [post]
func (h *Handler) setupRequiredTOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if !h.config.Auth.TOTP.Required {
		common.RespondError(w, 404, "Not found")
		return
	}
	id, err := h.mfa.EnrollmentUser(r.Context(), in.MFAToken)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	setup, err := h.mfa.Setup(r.Context(), id)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	common.RespondJSON(w, 200, setup)
}

// @Summary Confirm required TOTP enrollment and return recovery codes once
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Enrollment challenge and code"
// @Success 200 {object} RecoveryResponse
// @Failure 401 {object} common.ErrorResponse
// @ID confirmRequiredTOTP
// @Router /api/v1/users/login/totp/confirm [post]
func (h *Handler) confirmRequiredTOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if !h.config.Auth.TOTP.Required {
		common.RespondError(w, 404, "Not found")
		return
	}
	id, codes, err := h.mfa.ConfirmEnrollment(r.Context(), in.MFAToken, in.Code)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	h.respondTOTPRecovery(w, r, id, codes)
}

// @Summary Replace one-use recovery codes
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Security BearerAuth
// @Success 200 {object} RecoveryResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID regenerateRecovery
// @Router /api/v1/users/totp/recovery [post]
func (h *Handler) regenerateRecovery(w http.ResponseWriter, r *http.Request) {
	u, ok := h.totpUser(w, r)
	if !ok {
		return
	}
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if !h.reauthenticate(w, r, u, in.Password) {
		return
	}
	codes, err := h.mfa.RegenerateRecovery(r.Context(), u.ID, in.Code)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	h.respondTOTPRecovery(w, r, u.ID, codes)
}

// @Summary Administratively reset a local second factor
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID resetTOTP
// @Router /api/v1/users/totp/reset/{id} [delete]
func (h *Handler) resetTOTP(w http.ResponseWriter, r *http.Request) {
	if h.mfa == nil {
		h.respondMFAError(w, mfa.ErrUnavailable)
		return
	}
	actor, ok := common.GetAuthenticatedUser(r.Context())
	if !ok || actor.ID == r.PathValue("id") {
		common.RespondError(w, 403, "Cannot reset own two-factor authentication")
		return
	}
	if err := h.mfa.Reset(r.Context(), r.PathValue("id")); err != nil {
		h.respondMFAError(w, err)
		return
	}
	w.WriteHeader(204)
}

// @Summary Complete local login using a second factor
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID verifyTOTPLogin
// @Router /api/v1/users/login/totp [post]
func (h *Handler) verifyTOTPLogin(w http.ResponseWriter, r *http.Request) {
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	id, err := h.mfa.VerifyLogin(r.Context(), in.MFAToken, in.Code)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	u, err := h.userService.Get(r.Context(), id)
	if err != nil || !u.Active || u.MustChangePassword {
		common.RespondError(w, 401, "Invalid credentials")
		return
	}
	h.issueSession(w, r, u)
}

// @Summary Change a mandatory password before the second factor
// @Tags users
// @Accept json
// @Produce json
// @Param request body TOTPRequest true "Verification request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID changeLoginPassword
// @Router /api/v1/users/login/password [post]
func (h *Handler) changeLoginPassword(w http.ResponseWriter, r *http.Request) {
	in, ok := h.totpInput(w, r)
	if !ok {
		return
	}
	if utf8.RuneCountInString(in.NewPassword) < 8 {
		common.RespondError(w, 400, "Password must contain at least 8 characters")
		return
	}
	id, err := h.mfa.ConsumePasswordChallenge(r.Context(), in.MFAToken)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	u, err := h.userService.UpdatePassword(r.Context(), id, in.NewPassword)
	if err != nil {
		if !respondUserError(w, err) {
			h.respondMFAError(w, err)
		}
		return
	}
	h.completeLocalLogin(w, r, u)
}

func (h *Handler) respondTOTPRecovery(w http.ResponseWriter, r *http.Request, id string, codes []string) {
	u, err := h.userService.Get(r.Context(), id)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	claims := map[string]interface{}{}
	for key, value := range u.Preferences {
		claims[key] = value
	}
	claims["auth_method"] = "local"
	token, err := h.authService.GenerateToken(r.Context(), u, claims)
	if err != nil {
		h.respondMFAError(w, err)
		return
	}
	common.RespondJSON(w, 200, RecoveryResponse{RecoveryCodes: codes, AccessToken: token})
}
