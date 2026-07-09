//go:build swagger

package v1

import (
	"net/http"

	_ "github.com/marmotdata/marmot/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

func registerSwagger(mux *http.ServeMux) {
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}
