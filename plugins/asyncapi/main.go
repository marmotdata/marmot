package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/asyncapi/asyncapi"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   asyncapi.Meta(),
		Source: &asyncapi.Source{},
	})
}
