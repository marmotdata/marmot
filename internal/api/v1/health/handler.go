package health

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/plugin"
)

type Handler struct {
	loadState *plugin.LoadState
}

func NewHandler() *Handler {
	return &Handler{loadState: plugin.GetLoadState()}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{Path: "/health", Method: http.MethodGet, Handler: h.live},
		{Path: "/livez", Method: http.MethodGet, Handler: h.live},
		{Path: "/readyz", Method: http.MethodGet, Handler: h.ready},
	}
}

// live proves the process is up. It never fails so k8s does not restart
// a pod while plugin loading is still in progress.
func (h *Handler) live(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// ready reports whether the server is ready to serve plugin-dependent
// traffic. Currently gates on plugin loading; extend the waiting_on
// slice for future dependencies (database, search, etc.).
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.loadState.Ready() {
		common.RespondJSON(w, http.StatusOK, map[string]any{"ready": true})
		return
	}
	w.Header().Set("Retry-After", "5")
	common.RespondJSON(w, http.StatusServiceUnavailable, map[string]any{
		"ready":      false,
		"waiting_on": []string{"plugins"},
	})
}
