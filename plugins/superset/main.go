package main

import (
	pluginsdk "github.com/marmotdata/plugin-sdk"

	"github.com/marmotdata/marmot/plugins/superset/superset"
)

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta:   superset.Meta(),
		Source: &superset.Source{},
	})
}
