package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/gluepipeline/gluepipeline"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   gluepipeline.Meta(),
		Source: &gluepipeline.Source{},
	})
}
