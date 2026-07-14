package plugins

import (
	"context"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/plugin"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/plugins",
			Method:  http.MethodGet,
			Handler: h.listPlugins,
		},
		{
			Path:    "/api/v1/plugins/aws/credentials/status",
			Method:  http.MethodGet,
			Handler: h.awsCredentialStatus,
		},
	}
}

// ListPluginsResponse wraps the registered plugin list with a Loading
// flag so the UI can render a "plugins still loading" banner while
// server startup finishes registering them, instead of a misleading
// "no plugins available" state.
type ListPluginsResponse struct {
	Plugins []pluginsdk.Meta `json:"plugins"`
	Loading bool             `json:"loading"`
} // @name ListPluginsResponse

// @Summary List registered plugins
// @Tags plugins
// @Produce json
// @Success 200 {object} ListPluginsResponse
// @Router /api/v1/plugins [get]
func (h *Handler) listPlugins(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, ListPluginsResponse{
		Plugins: plugin.GetRegistry().List(),
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
