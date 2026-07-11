//go:build !swagger

package v1

import "net/http"

func registerSwagger(_ *http.ServeMux) {}
