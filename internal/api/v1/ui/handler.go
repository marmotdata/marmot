package ui

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
)

type Handler struct {
	config *config.Config
}

func NewHandler(config *config.Config) *Handler {
	return &Handler{
		config: config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/ui/config",
			Method:  http.MethodGet,
			Handler: h.getUIConfig,
		},
	}
}

type UIConfigResponse struct {
	Banner BannerResponse `json:"banner"`
}

type BannerResponse struct {
	Enabled     bool   `json:"enabled"`
	Dismissible bool   `json:"dismissible"`
	Variant     string `json:"variant"`
	Message     string `json:"message"`
	ID          string `json:"id"`
}

// @Summary Get UI configuration
// @Description Get UI configuration including banner settings
// @Tags ui
// @Produce json
// @Success 200 {object} UIConfigResponse
// @Router /ui/config [get]
func (h *Handler) getUIConfig(w http.ResponseWriter, r *http.Request) {
	response := UIConfigResponse{
		Banner: BannerResponse{
			Enabled:     h.config.UI.Banner.Enabled,
			Dismissible: h.config.UI.Banner.Dismissible,
			Variant:     h.config.UI.Banner.Variant,
			Message:     h.config.UI.Banner.Message,
			ID:          h.config.UI.Banner.ID,
		},
	}

	common.RespondJSON(w, http.StatusOK, response)
}
