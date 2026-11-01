package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/timescale/timescale"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   timescale.Meta(),
		Source: &timescale.Source{},
	})
}
