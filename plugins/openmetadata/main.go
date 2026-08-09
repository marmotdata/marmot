package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/openmetadata/openmetadata"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   openmetadata.Meta(),
		Source: &openmetadata.Source{},
	})
}
