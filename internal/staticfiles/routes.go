//go:build !production

package staticfiles

import "net/http"

func (h *Handler) SetupRoutes(mux *http.ServeMux) error {
	return nil
}
