package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/pinot/pinot"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   pinot.Meta(),
		Source: &pinot.Source{},
	})
}
