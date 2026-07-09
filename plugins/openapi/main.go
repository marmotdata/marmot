package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/openapi/openapi"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   openapi.Meta(),
		Source: &openapi.Source{},
	})
}
