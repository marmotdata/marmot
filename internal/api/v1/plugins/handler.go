package plugins

import (
	"context"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/pkg/config"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Handler struct {
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{userService: userService, authService: authService, config: config}
}

// Both routes answer with deployment state: which plugins this server has
// registered, and whether it holds AWS credentials. Neither is public.
func (h *Handler) Routes() []common.Route {
	authenticated := common.WithAuth(h.userService, h.authService, h.config)
	return []common.Route{
		{
			Path:   "/api/v1/plugins",
			Method: http.MethodGet,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authenticated,
				common.RequirePermission(h.userService, "ingestion", "view"),
			},
			Handler: h.listPlugins,
		},
		{
			// manage rather than view: this reports the server's own credential
			// environment, and only the pipeline editor asks for it.
			Path:   "/api/v1/plugins/aws/credentials/status",
			Method: http.MethodGet,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authenticated,
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
			Handler: h.awsCredentialStatus,
		},
	}
}

// ListPluginsResponse wraps the registered plugin list with a Loading
// flag so the UI can render a "plugins still loading" banner while
// server startup finishes registering them, instead of a misleading
// "no plugins available" state.
type ListPluginsResponse struct {
	Plugins []PluginInfo `json:"plugins"`
	Loading bool         `json:"loading"`
} // @name ListPluginsResponse

// PluginInfo is a plugin's metadata with a summary of what it brings, read from that metadata so the gallery and the new pipeline form can say it without each plugin repeating it.
type PluginInfo struct {
	pluginsdk.Meta
	// Catalogs names the kinds of asset the plugin documents, in the order it lists them.
	Catalogs []string `json:"catalogs"`
	// ProducesLineage reports whether the plugin says it discovers lineage between assets.
	ProducesLineage bool `json:"produces_lineage"`
} // @name PluginInfo

// summarise builds a plugin's summary from its metadata. Asset schemas name what it catalogs, and its feature list says whether it finds lineage.
func summarise(meta pluginsdk.Meta) PluginInfo {
	info := PluginInfo{Meta: meta, Catalogs: []string{}}
	seen := map[string]bool{}
	for _, schema := range meta.AssetSchemas {
		name := strings.TrimSpace(schema.DisplayName)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		info.Catalogs = append(info.Catalogs, name)
	}
	for _, feature := range meta.Features {
		if strings.EqualFold(strings.TrimSpace(feature), "lineage") {
			info.ProducesLineage = true
		}
	}
	return info
}

// @Summary List registered plugins
// @Tags plugins
// @Produce json
// @Success 200 {object} ListPluginsResponse
// @ID getPlugins
// @Router /api/v1/plugins [get]
func (h *Handler) listPlugins(w http.ResponseWriter, r *http.Request) {
	metas := plugin.GetRegistry().List()
	plugins := make([]PluginInfo, 0, len(metas))
	for _, meta := range metas {
		plugins = append(plugins, summarise(meta))
	}
	common.RespondJSON(w, http.StatusOK, ListPluginsResponse{
		Plugins: plugins,
		Loading: !plugin.GetLoadState().Ready(),
	})
}

// AWSCredentialStatus is the response for
// GET /api/v1/plugins/aws/credentials/status. The UI calls that
// endpoint while a user configures an AWS-based plugin (S3, Glue, and
// so on) to show whether the Marmot server already has AWS credentials
// in its environment: it tells the user up front whether "use default
// credentials" will work or whether they need to enter keys. The check
// itself is pluginsdk.DetectAWSCredentials, which walks the same
// credential chain the AWS plugins use when they connect. This local
// struct exists so the API response schema is owned here; the SDK type
// can change shape without silently changing our API.
type AWSCredentialStatus struct {
	Available bool     `json:"available"`
	Sources   []string `json:"sources"`
	Error     string   `json:"error,omitempty"`
} // @name AWSCredentialStatus

// @Summary Get AWS credential detection status
// @Description Detects if AWS credentials are available from environment or config files
// @Tags plugins
// @Produce json
// @Success 200 {object} AWSCredentialStatus
// @ID getPluginsAwsCredentialsStatus
// @Router /api/v1/plugins/aws/credentials/status [get]
func (h *Handler) awsCredentialStatus(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	status := pluginsdk.DetectAWSCredentials(ctx)
	common.RespondJSON(w, http.StatusOK, AWSCredentialStatus{
		Available: status.Available,
		Sources:   status.Sources,
		Error:     status.Error,
	})
}
