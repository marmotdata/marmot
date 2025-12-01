package plugins

import (
	"context"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/plugin"
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

func (h *Handler) listPlugins(w http.ResponseWriter, r *http.Request) {
	registry := plugin.GetRegistry()
	plugins := registry.List()

	common.RespondJSON(w, http.StatusOK, plugins)
}

// @Summary Get AWS credential detection status
// @Description Detects if AWS credentials are available from environment or config files
// @Tags plugins
// @Produce json
// @Success 200 {object} plugin.AWSCredentialStatus
// @Router /api/v1/plugins/aws/credentials/status [get]
func (h *Handler) awsCredentialStatus(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	status := plugin.DetectAWSCredentials(ctx)
	common.RespondJSON(w, http.StatusOK, status)
}
