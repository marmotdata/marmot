package health

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/health",
			Method:  http.MethodGet,
			Handler: h.health,
		},
	}
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}
