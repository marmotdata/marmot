package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/settings"
	"github.com/marmotdata/marmot/internal/core/sysinfo"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/rs/zerolog/log"
)

var productVersion = "dev"

// SetVersion records the product version the binary was released as, which only the command package knows.
func SetVersion(v string) {
	if v != "" {
		productVersion = v
	}
}

// Extras are what the system and sign-in settings need beyond the reindex endpoints.
type Extras struct {
	DB       *pgxpool.Pool
	Settings settings.Store
	OAuth    *auth.OAuthManager
	Teams    *team.Service
	Roles    role.Service
}

// With adds the system and sign-in settings endpoints.
func (h *Handler) With(e Extras) *Handler {
	h.extras = &e
	return h
}

func (h *Handler) extraRoutes() []common.Route {
	if h.extras == nil {
		return nil
	}
	admin := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "users", "manage"),
	}
	sso := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "sso", "manage"),
	}
	return []common.Route{
		{Path: "/api/v1/admin/system", Method: http.MethodGet, Handler: h.getSystem, Middleware: admin},
		{Path: "/api/v1/admin/auth/providers", Method: http.MethodGet, Handler: h.listProviders, Middleware: sso},
		{Path: "/api/v1/admin/auth/providers/{type}", Method: http.MethodPut, Handler: h.updateProvider, Middleware: sso},
		{Path: "/api/v1/admin/auth/settings", Method: http.MethodGet, Handler: h.getSignInSettings, Middleware: sso},
		{Path: "/api/v1/admin/auth/settings", Method: http.MethodPut, Handler: h.updateSignInSettings, Middleware: sso},
	}
}

// SystemResponse describes the running instance. It is admin-only: versions tell an attacker which advisories apply.
type SystemResponse struct {
	Build       sysinfo.Build     `json:"build"`
	Database    sysinfo.Database  `json:"database"`
	SearchIndex SystemSearchIndex `json:"search_index"`
} // @name SystemResponse

// SystemSearchIndex is the search index's size and, when Elasticsearch mirrors it, when it was last rebuilt in full.
type SystemSearchIndex struct {
	sysinfo.SearchIndex
	// Backend is postgres, or elasticsearch when Elasticsearch serves searches from a copy of the index.
	Backend         string     `json:"backend"`
	ReindexRunning  bool       `json:"reindex_running"`
	LastFullReindex *time.Time `json:"last_full_reindex,omitempty"`
} // @name SystemSearchIndex

// @Summary Get system information
// @Description Product version and build, the Postgres version and the size of the search index. Admin only.
// @Tags admin
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} SystemResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @ID getAdminSystem
// @Router /api/v1/admin/system [get]
func (h *Handler) getSystem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := SystemResponse{Build: sysinfo.ReadBuild(productVersion)}

	db, err := sysinfo.ReadDatabase(ctx, h.extras.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read database information")
		common.RespondError(w, http.StatusInternalServerError, "Failed to read system information")
		return
	}
	resp.Database = db

	idx, err := sysinfo.ReadSearchIndex(ctx, h.extras.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read search index information")
		common.RespondError(w, http.StatusInternalServerError, "Failed to read system information")
		return
	}
	resp.SearchIndex = SystemSearchIndex{SearchIndex: idx, Backend: "postgres"}
	if h.reindexer != nil {
		resp.SearchIndex.Backend = "elasticsearch"
		resp.SearchIndex.ReindexRunning = h.reindexer.Running()
		if h.extras.Settings != nil {
			last, err := settings.LastFullReindex(ctx, h.extras.Settings)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to read last full reindex time")
			}
			resp.SearchIndex.LastFullReindex = last
		}
	}

	common.RespondJSON(w, http.StatusOK, resp)
}

// SSOProviderStatus is a configured single sign-on provider as the Authentication settings show it.
type SSOProviderStatus struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	IssuerURL string `json:"issuer_url,omitempty"`
	// Enabled is false when an administrator has switched the provider off here; its configuration is kept.
	Enabled bool `json:"enabled"`
	// MemberCount is how many members have signed in with the provider at least once.
	MemberCount int `json:"member_count"`
} // @name SSOProviderStatus

// SSOProviderStatusList lists the configured providers.
type SSOProviderStatusList struct {
	Providers []SSOProviderStatus `json:"providers"`
} // @name SSOProviderStatusList

// UpdateSSOProviderRequest switches a provider on or off.
type UpdateSSOProviderRequest struct {
	Enabled bool `json:"enabled"`
} // @name UpdateSSOProviderRequest

// protocols says how each provider type signs people in; those not listed use OpenID Connect.
var protocols = map[string]string{
	"github": "OAuth 2.0",
	"slack":  "OAuth 2.0",
}

func (h *Handler) providerStatuses(r *http.Request) ([]SSOProviderStatus, error) {
	counts, err := h.userService.CountIdentitiesByProvider(r.Context())
	if err != nil {
		return nil, err
	}
	out := []SSOProviderStatus{}
	if h.extras.OAuth == nil {
		return out, nil
	}
	for _, p := range h.extras.OAuth.AllProviders() {
		item := SSOProviderStatus{
			Type:        p.Type(),
			Name:        p.Name(),
			Protocol:    "OIDC",
			Enabled:     h.extras.OAuth.Enabled(p.Type()),
			MemberCount: counts[p.Type()],
		}
		if proto, ok := protocols[p.Type()]; ok {
			item.Protocol = proto
		}
		if ip, ok := p.(auth.IssuerProvider); ok {
			item.IssuerURL = ip.IssuerURL()
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// @Summary List single sign-on providers with their status
// @Description Every provider configured in the server config, whether it is switched on and how many members have signed in with it.
// @Tags admin
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} SSOProviderStatusList
// @Failure 403 {object} common.ErrorResponse
// @ID getAdminAuthProviders
// @Router /api/v1/admin/auth/providers [get]
func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	list, err := h.providerStatuses(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list single sign-on providers")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list single sign-on providers")
		return
	}
	common.RespondJSON(w, http.StatusOK, SSOProviderStatusList{Providers: list})
}

// @Summary Switch a single sign-on provider on or off
// @Description Providers are configured in the server config; this switches one off without removing its configuration, or back on. Members who signed in with it keep their accounts.
// @Tags admin
// @Accept json
// @Produce json
// @Param type path string true "Provider type, such as okta"
// @Param body body UpdateSSOProviderRequest true "Whether the provider is on"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} SSOProviderStatus
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID putAdminAuthProvidersType
// @Router /api/v1/admin/auth/providers/{type} [put]
func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request) {
	providerType := r.PathValue("type")
	if h.extras.OAuth == nil || h.extras.OAuth.AllProviders()[providerType] == nil {
		common.RespondError(w, http.StatusNotFound, "No such provider is configured")
		return
	}
	var req UpdateSSOProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx := r.Context()
	if !req.Enabled {
		// Switching off the last provider while password sign-in is off would leave only administrators able to sign in.
		cfg, err := settings.GetSignIn(ctx, h.extras.Settings)
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "Failed to read sign-in settings")
			return
		}
		if !cfg.AllowPasswordSignIn && h.enabledProvidersOtherThan(providerType) == 0 {
			common.RespondError(w, http.StatusConflict, "Turn password sign-in on before switching off the last single sign-on provider")
			return
		}
	}

	by := ""
	if u, ok := common.GetAuthenticatedUser(ctx); ok {
		by = u.ID
	}
	if err := settings.SetProviderEnabled(ctx, h.extras.Settings, providerType, req.Enabled, by); err != nil {
		log.Error().Err(err).Msg("Failed to switch single sign-on provider")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update provider")
		return
	}
	h.extras.OAuth.Refresh()

	list, err := h.providerStatuses(r)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to read provider")
		return
	}
	for _, p := range list {
		if p.Type == providerType {
			common.RespondJSON(w, http.StatusOK, p)
			return
		}
	}
	common.RespondError(w, http.StatusNotFound, "No such provider is configured")
}

func (h *Handler) enabledProvidersOtherThan(providerType string) int {
	if h.extras.OAuth == nil {
		return 0
	}
	n := 0
	for t := range h.extras.OAuth.AllProviders() {
		if t != providerType && h.extras.OAuth.Enabled(t) {
			n++
		}
	}
	return n
}

// @Summary Get sign-in settings
// @Description Whether single sign-on creates new members, the role and team they start with, and whether members may sign in with a password.
// @Tags admin
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} settings.SignIn
// @Failure 403 {object} common.ErrorResponse
// @ID getAdminAuthSettings
// @Router /api/v1/admin/auth/settings [get]
func (h *Handler) getSignInSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := settings.GetSignIn(r.Context(), h.extras.Settings)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read sign-in settings")
		common.RespondError(w, http.StatusInternalServerError, "Failed to read sign-in settings")
		return
	}
	common.RespondJSON(w, http.StatusOK, cfg)
}

// @Summary Update sign-in settings
// @Description Replaces the sign-in settings. Administrators can always sign in with a password, so turning password sign-in off cannot lock everyone out; it is refused while no single sign-on provider is on.
// @Tags admin
// @Accept json
// @Produce json
// @Param body body settings.SignIn true "Sign-in settings"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} settings.SignIn
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID putAdminAuthSettings
// @Router /api/v1/admin/auth/settings [put]
func (h *Handler) updateSignInSettings(w http.ResponseWriter, r *http.Request) {
	var cfg settings.SignIn
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	ctx := r.Context()

	if cfg.DefaultRole == "" {
		common.RespondError(w, http.StatusBadRequest, "A default role is required")
		return
	}
	if h.extras.Roles != nil {
		roles, err := h.extras.Roles.List(ctx)
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "Failed to read roles")
			return
		}
		found := false
		for _, ro := range roles {
			if ro.Name == cfg.DefaultRole {
				found = true
				break
			}
		}
		if !found {
			common.RespondError(w, http.StatusBadRequest, "The default role does not exist")
			return
		}
	}
	if cfg.DefaultTeamID != "" && h.extras.Teams != nil {
		if _, err := h.extras.Teams.GetTeam(ctx, cfg.DefaultTeamID); err != nil {
			if errors.Is(err, team.ErrTeamNotFound) {
				common.RespondError(w, http.StatusBadRequest, "The default team does not exist")
				return
			}
			common.RespondError(w, http.StatusInternalServerError, "Failed to read the default team")
			return
		}
	}
	if !cfg.AllowPasswordSignIn && h.enabledProvidersOtherThan("") == 0 {
		common.RespondError(w, http.StatusConflict, "Switch a single sign-on provider on before turning password sign-in off")
		return
	}

	by := ""
	if u, ok := common.GetAuthenticatedUser(ctx); ok {
		by = u.ID
	}
	if err := h.extras.Settings.Set(ctx, settings.KeySignIn, cfg, by); err != nil {
		log.Error().Err(err).Msg("Failed to save sign-in settings")
		common.RespondError(w, http.StatusInternalServerError, "Failed to save sign-in settings")
		return
	}
	common.RespondJSON(w, http.StatusOK, cfg)
}

// reindexRecorder passes reindex events on and notes when a full reindex finishes, so the System page can say how fresh the Elasticsearch copy is after a restart.
type reindexRecorder struct {
	search.ReindexBroadcaster
	store settings.Store
}

// RecordReindexCompletion wraps b so every finished full reindex is recorded, whether an administrator started it or the server did at startup.
func RecordReindexCompletion(b search.ReindexBroadcaster, store settings.Store) search.ReindexBroadcaster {
	if b == nil {
		b = &search.NoopReindexBroadcaster{}
	}
	return &reindexRecorder{ReindexBroadcaster: b, store: store}
}

func (r *reindexRecorder) BroadcastCompleted(indexed, errs, total int) {
	r.ReindexBroadcaster.BroadcastCompleted(indexed, errs, total)
	if r.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.store.Set(ctx, settings.KeyLastFullReindex, time.Now().UTC(), ""); err != nil {
		log.Warn().Err(err).Msg("Failed to record search reindex completion")
	}
}
