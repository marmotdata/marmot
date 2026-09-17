package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/vertexai/vertexai"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   vertexai.Meta(),
		Source: &vertexai.Source{},
	})
}
