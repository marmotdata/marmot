package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/glue/glue"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   glue.Meta(),
		Source: &glue.Source{},
	})
}
